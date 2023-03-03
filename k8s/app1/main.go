package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

var hostname string

func hello(w http.ResponseWriter, req *http.Request) {

	log.Println("request: ", req)
	fmt.Fprint(w, "hello: ", hostname)
}

func headers(w http.ResponseWriter, req *http.Request) {

	for name, headers := range req.Header {
		for _, h := range headers {
			fmt.Fprintf(w, "%v: %v\n", name, h)
		}
	}
}

func main() {

	h, err := os.Hostname()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	hostname = h

	log.Println("app started: ", hostname)

	http.HandleFunc("/", hello)
	http.HandleFunc("/headers", headers)

	http.ListenAndServe(":8080", nil)
}
