/**
 * PDX chainmux, an HTTP-CONNECT based, whitelisted TCP multiplexer service
 *
 * Credit to https://medium.com/@mlowicki for the original https-proxy work
 */
 
package main

import (
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"time"
	"os"
	"bufio"
	"strings"
	"fmt"
	"sync"
)

type rewriteRules struct {

	lock  sync.Mutex
	data map[string]string
}

var port string
var fconf string

var rules = rewriteRules{data:make(map[string]string)}

func loadRules() {

	file, err := os.OpenFile(fconf, os.O_RDONLY, os.ModeExclusive)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	data := make(map[string]string)

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {

		text := scanner.Text()

		if strings.HasPrefix(text, "#") {
			continue
		}

		el := strings.Fields(scanner.Text())

		if len(el) == 2 {

			el[0] = strings.TrimSpace(el[0])
			el[1] = strings.TrimSpace(el[1])

			if el[0] != "" && el[1] != "" {
				data[el[0]] = el[1]
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}


	rules.lock.Lock()
		rules.data = data
	rules.lock.Unlock()
}

func rewriteTo(asked string) string {

	asked = strings.TrimSpace(asked)

	rules.lock.Lock()
	defer rules.lock.Unlock()

	for k,v := range rules.data {
		if strings.EqualFold(k, asked) {
			return v
		}
	}

	return ""
}

func handleTunneling(w http.ResponseWriter, r *http.Request) {

	dst := rewriteTo("conn://" + r.RequestURI)

	log.Println("CONN: requested ", r.RequestURI, ", redirected to:", dst)

	if dst == "" {
		http.Error(w, r.RequestURI + " is not allowed", http.StatusServiceUnavailable)
		return
	}

	dest_conn, err := net.DialTimeout("tcp", dst, 10*time.Second)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}
	client_conn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
	}
	go transfer(dest_conn, client_conn)
	go transfer(client_conn, dest_conn)
}

func transfer(dst io.WriteCloser, src io.ReadCloser) {
	defer dst.Close()
	defer src.Close()
	io.Copy(dst, src)
}

func handleHTTP(w http.ResponseWriter, r *http.Request) {

	url, err := url.Parse(r.RequestURI)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if strings.EqualFold(r.RequestURI, "http://localhost:" + port + "/chainmux/reconf") &&
		strings.HasPrefix(r.RemoteAddr, "127.0.0.1:") {
                log.Println("reloading rewrite conf file")
		loadRules()
		return
	}

	asked := "http://" + url.Host
	if url.Port() != "" {
		asked += ":" + url.Port()
	} else {
		asked += ":80"
	}

	dst := rewriteTo(asked) //host:port
	if dst == "" {
		http.Error(w, r.RequestURI + " is not allowed", http.StatusServiceUnavailable)
		return
	}

	asked = r.RequestURI

	r.RequestURI = "http://" + dst + url.RequestURI()

	log.Println("HTTP: requested ", asked, ", redirected to:", r.RequestURI)

	resp, err := http.DefaultTransport.RoundTrip(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	defer resp.Body.Close()
	copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func main() {

	flag.Usage = func() {
		fmt.Println("")
		fmt.Println("PDX chainmux, an HTTP-CONNECT based, whitelisted rewritable HTTP & TCP proxy")
		fmt.Println("")
		fmt.Println("-conf		The configuration file. Or via PDX_CHAINMUX_CONF_FILE environment variable")
		fmt.Println("		One line for each whitelist item, first match is selected.")
		fmt.Println("		Config format: proto://asked_host:asked_port \\t target_host:target_port\\n")
		fmt.Println("		Here, proto is conn for HTTP-CONNECT based tunneling, http for  http proxy")
		fmt.Println("		For example,")
		fmt.Println("			conn://chain-x:30303 localhost:30308")
		fmt.Println("			http://pdx.ltd:80 localhost:80")
		fmt.Println("")
		fmt.Println("-addr		The [host]:port chainmux listens on")
		fmt.Println("")
                fmt.Println("Please visit https://github.com/PDXbaap/chainmux to get the latest version.")
		fmt.Println("")
	}

	flag.StringVar(&fconf, "conf", "", "conf file for CONNECT redirect")

	var addr string
	flag.StringVar(&addr, "addr", ":5978","proxy listening address, in host:addr format")

	data := strings.Split(addr, ":")
	if len(data) == 2 {
		port = data[1]
	}

	flag.Parse()

	if fconf == "" {
		fconf = os.Getenv("PDX_CHAINMUX_CONF_FILE")
	}

	loadRules()

	server := &http.Server{
		//ReadTimeout:  10 * time.Second,
		//WriteTimeout: 10 * time.Second,
		//IdleTimeout:  10 * time.Second,
		Addr: addr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodConnect {
				handleTunneling(w, r)
			} else {
				handleHTTP(w, r)
			}
		}),}

	log.Println("started PDX chainmux")

	log.Fatal(server.ListenAndServe())

	log.Println("shutdown PDX chainmux")
}
