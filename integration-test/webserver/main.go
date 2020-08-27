package main

import (
	"flag"
	"log"
	"net/http"
	"strings"
	"context"
)

func main() {
	states := flag.String("states", "", "contains comma separated u or h repesenting unhealthy and healthy")
	flag.Parse()
	healthStates := strings.Split(*states, ",")
	var shouldExitOnEmptyHealthStates = len(healthStates) > 0
	httpMutex := http.NewServeMux()
    	httpServer := http.Server{Addr: ":8080", Handler: httpMutex }
	httpsServer := http.Server{Addr: ":443", Handler: httpMutex }
    	httpMutex.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        	var statusCode = 200
		if len(healthStates) > 0 {
			if healthStates[0] == "u" {
				statusCode = 500
			}
			healthStates = healthStates[1:]
		}		
		w.WriteHeader(statusCode)
		if shouldExitOnEmptyHealthStates && len(healthStates) == 0 {
			go func() {
				log.Printf("Shutting down")				 
				httpServer.Shutdown(context.Background())
				httpsServer.Shutdown(context.Background())		
			}()
			
		}
    	})
	go httpServer.ListenAndServe()
	httpsServer.ListenAndServeTLS("webservercert.pem", "webserverkey.pem")
}