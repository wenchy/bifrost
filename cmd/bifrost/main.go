package main

import (
	"bytes"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"

	"github.com/Wenchy/bifrost/cmd/bifrost/conf"
	"github.com/Wenchy/bifrost/cmd/bifrost/ws"
	"github.com/Wenchy/bifrost/internal/atom"
)

func main() {
	confpath := flag.String("conf", "conf.yaml", "config file path.")
	flag.Parse()
	conf.InitConf(*confpath) // server config

	atom.InitZap(conf.Conf.Log.Level, conf.Conf.Log.Dir) // log
	defer atom.Log.Sync()

	go ws.Hub.Run()

	if conf.Conf.Server.PeerAddr != "" {
		ws.BuildNewTunnel(conf.Conf.Server.PeerAddr)
	}

	// start server
	http.HandleFunc("/", handleRequestAndRedirect)
	http.HandleFunc("/ws", func(rw http.ResponseWriter, req *http.Request) {
		ws.ServeWS(rw, req)
	})

	if err := http.ListenAndServe(conf.Conf.Server.SelfAddr, nil); err != nil {
		panic(err)
	}
}

func findTarget(req *http.Request) string {

	// step-1: judge by HTTP header field "X-Bifrost-Target"
	target := req.Header.Get("X-Bifrost-Target")
	if target != "" {
		return target
	}
	// step-2: judge by conf
	for _, proxy := range conf.Conf.Proxies {
		matched, err := regexp.MatchString(proxy.Path, req.URL.Path)
		if err != nil {
			atom.Log.Errorf("path: %v, MatchString failed: %v", proxy.Path, err)
			return ""
		}
		if matched {
			return proxy.Target
		}
	}
	return ""
}

// Given a request send it to the appropriate url
func handleRequestAndRedirect(rw http.ResponseWriter, req *http.Request) {
	// requestPayload := getRequestBodyCopy(req)
	target := findTarget(req)
	if target == "" {
		atom.Log.Errorf("target not found of path: %s", req.URL.Path)
		return
	}
	logRequestPayload(req, target)

	ws.Forward(target, rw, req)
	// serveReverseProxy(proxyTargetUrl, rw, req)
}

// Serve a reverse proxy for a given url
func serveReverseProxy(target string, rw http.ResponseWriter, req *http.Request) {
	// parse the url
	url, _ := url.Parse(target)

	// create the reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(url)

	// Update the headers to allow for SSL redirection
	req.URL.Host = url.Host
	req.URL.Scheme = url.Scheme
	req.Header.Set("X-Forwarded-Host", req.Header.Get("Host"))
	req.Host = url.Host

	// Note that ServeHttp is non blocking and uses a go routine under the hood
	proxy.ServeHTTP(rw, req)
}

// get the copy for a given requests body
func getRequestBodyCopy(request *http.Request) io.ReadCloser {
	// Read body to buffer
	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		log.Printf("Error reading body: %v", err)
		panic(err)
	}

	// Because go lang is a pain in the ass if you read the body then any susequent calls
	// are unable to read the body again....
	request.Body = ioutil.NopCloser(bytes.NewBuffer(body))

	return ioutil.NopCloser(bytes.NewBuffer(body))
}

// Log the typeform payload and redirect url
func logRequestPayload(req *http.Request, target string) {
	log.Printf("target: %s, from: %s\n", target, req.URL.String())
}
