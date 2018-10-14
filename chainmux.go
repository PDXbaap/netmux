// Copyright 2017 The PDX Blockchain Hybercloud Authors
// This file is part of the PDX chainmux implementation.
//
// The PDX Blcockchain Hypercloud is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The PDX Blockchain Hypercloud is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the software. If not, see <http://www.gnu.org/licenses/>.


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
	"path/filepath"
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

		if el == nil || len(el) <= 0 || len(el) > 2 {
			continue
		}

		if len(el) == 1 { //allow

			data[el[0]] = el[0]

		} else if len(el) == 2 { //rewrite

			data[el[0]] = el[1]
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

		matched, error := filepath.Match(k, asked)

		if error == nil && matched {
			if strings.EqualFold(k,v) {//allowed
				return asked
			} else { //rewrite
				return v
			}
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
		fmt.Println("PDX chainmux, a lightweight HTTP & TCP service multiplexer, ver. 1.0")
		fmt.Println("")
		fmt.Println("-conf	The configuration file. Or via PDX_CHAINMUX_CONF_FILE environment variable")
		fmt.Println("")
		fmt.Println("	Configuration file syntax:")
		fmt.Println("		1) One line for each access granted (as-is or rewrite), honoring the first match")
		fmt.Println("		2) Line format: proto://request_host:request_port \\t [service_host:service_port]\\n")
		fmt.Println("		3) proto is conn for HTTP-CONNECT based tunneling, http for http proxy.")
		fmt.Println("		4) proto://request_host:request_port can be a glob match pattern")
		fmt.Println("		5) If no rewrite is desired, no service_host:service_port should be specified")
		fmt.Println("		6) A comment line (one starts with #) or an empty line is ignored by the parser.")
		fmt.Println("")
		fmt.Println("	For example,")
		fmt.Println("		conn://chain-x:30303 localhost:30308")
		fmt.Println("		http://view.pdx.ltd:80 http://localhost:8080")
		fmt.Println("		http://chain.pdx.link*")
		fmt.Println("")
		fmt.Println("-addr	The [host]:port chainmux listens on")
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
