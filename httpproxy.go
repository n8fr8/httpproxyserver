package main

import (
	"errors"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
)

var (
	tr = &http.Transport{
		DisableCompression: true,
	}
	errHasRedirect = errors.New("has redirect")
	c              = &http.Client{Transport: tr,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return errHasRedirect
		},
	}
)

func connectProxy(w http.ResponseWriter, r *http.Request) {
	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		log.Print("proxy hijacking ", r.RemoteAddr, ": ", err.Error())
		return
	}
	defer conn.Close()

	_, err = io.WriteString(conn, "HTTP/1.1 200 Connected\r\n\r\n")
	if err != nil {
		return
	}

	dstConn, err := net.Dial("tcp", r.RequestURI)
	if err != nil {
		return
	}
	defer dstConn.Close()

	go io.Copy(conn, dstConn)
	io.Copy(dstConn, conn)
}

func httpProxy(w http.ResponseWriter, r *http.Request) {
	r1, err := http.NewRequest(r.Method, r.URL.String(), r.Body)
	if err != nil && err != errHasRedirect {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for k, v := range r.Header {
		if k == "Proxy-Connection" {
			k = "Connection"
		}
		r1.Header.Set(k, v[0])
	}

	resp, err := c.Do(r1)
	if err != nil && err.(*url.Error).Err != errHasRedirect {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var body []byte
	if err == nil {
		defer resp.Body.Close()

		body, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	for k, v := range resp.Header {
		w.Header().Set(k, v[0])
	}
	w.WriteHeader(resp.StatusCode)
	w.Write(body)
}

func proxyHandler(w http.ResponseWriter, r *http.Request) {
	log.Println(r.Method, r.RequestURI)
	defer func() {
		log.Printf("done %s", r.RequestURI)
	}()

	if r.Method == "CONNECT" {
		connectProxy(w, r)
	} else {
		httpProxy(w, r)
	}
}

func main() {
	log.Fatal(http.ListenAndServe(":8888", http.HandlerFunc(proxyHandler)))
}
