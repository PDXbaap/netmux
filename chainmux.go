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
	"time"
	"os"
	"bufio"
	"strings"
	"fmt"
)

func redirectTo(conf string, asked string) string {

	file, err := os.OpenFile(conf, os.O_RDONLY, os.ModeExclusive)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		text := scanner.Text()
		if strings.HasPrefix(text, asked) {
			asked = text[len(asked):]
			asked = strings.TrimSpace(asked)
			return asked
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	return ""
}

func handleTunneling(conf string, w http.ResponseWriter, r *http.Request) {

	dst := redirectTo(conf, r.RequestURI)

	//log.Println("requested ", r.RequestURI, ", redirected to:", dst)

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

func main() {

	flag.Usage = func() {
		fmt.Println("")
		fmt.Println("PDX chainmux, an HTTP-CONNECT based, whitelisted redirecting TCP multiplexer")
		fmt.Println("")
		fmt.Println("-conf		The configuration file. Or via PDX_CHAINMUX_CONF_FILE env")
		fmt.Println("		One line for each whitelist item, first match is selected.")
		fmt.Println("		Config format: requested-URI \\t redirected-URI \\n")
		fmt.Println("-port		The [host]:port chainmux listens on")
		fmt.Println("")
                fmt.Println("Please visit https://github.com/PDXbaap/chainmux to get the latest version.")
		fmt.Println("")
	}

	var conf string
	flag.StringVar(&conf, "conf", "", "conf file for CONNECT redirect")

	var port string
	flag.StringVar(&port, "addr", ":5978","proxy listening address, in host:port format")

	flag.Parse()

	if conf == "" {
		conf = os.Getenv("PDX_CHAINMUX_CONF_FILE")
	}

	server := &http.Server{
		Addr: port,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodConnect {
				handleTunneling(conf, w, r)
			}
		}),}

	log.Println("started PDX chainmux")

	log.Fatal(server.ListenAndServe())

	log.Println("shutdown PDX chainmux")
}
