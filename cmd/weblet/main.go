package main

import (
	"flag"
	"fmt"
	"net/http"
)

func main() {
	addr := *flag.String("addr", ":18080", "http listen host:port.")
	flag.Parse()
	fmt.Println("http serve on", addr)
	http.HandleFunc("/test/", func(rw http.ResponseWriter, req *http.Request) {
		fmt.Fprintf(rw, "Wow, test works!")
	})

	if err := http.ListenAndServe(addr, nil); err != nil {
		panic(err)
	}
}