package main

import (
    "net/http"
    "log"
)

func ServeRequest(w http.ResponseWriter, req *http.Request) {
    w.Header().Set("Content-Type", "text/plain")
    w.Write([]byte("Hello world!\n"))
}

func main() {
	go serveHttps()
    err := http.ListenAndServe(":8080", nil)
    if err != nil {
        log.Fatal("ListenAndServe: ", err)
    }
}

func serveHttps() {
	http.HandleFunc("/health", ServeRequest)
    err := http.ListenAndServeTLS(":443", "webservercert.pem", "webserverkey.pem", nil)
    if err != nil {
        log.Fatal("ListenAndServeTLS: ", err)
    }
}