package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"time"
	"strings"
	"sync"
)

type Payload struct {
	Items []Item	`json:"payload"`
}

type Item struct {
	HttpStatusCode	int					`json:"httpStatusCode"`
	Timeout 		*bool				`json:"timeout,omitempty"`
	ResponseBody	*map[string]string	`json:"responseBody,omitempty"`
}

type HealthStatus string

const (
	Initializing HealthStatus = "Initializing"
	Healthy      HealthStatus = "Healthy"
	Unhealthy    HealthStatus = "Unhealthy"
	Unknown      HealthStatus = "Unknown"
	Empty        HealthStatus = ""
)

var (
	allowedHealthStatuses = map[HealthStatus]bool{
		Healthy:   true,
		Unhealthy: true,
	}
)

const (
	TimeoutInSeconds                  = 35
	ApplicationHealthStateResponseKey = "ApplicationHealthState" // Response body key name
)

func healthHandler(w http.ResponseWriter, r *http.Request, wg *sync.WaitGroup) {
	defer wg.Done()
	payloadItem := Item {
		HttpStatusCode: 200,
		Timeout: func() *bool { b := false; return &b }(),
		ResponseBody: nil,
	}

	if len(payload.Items) > 0 {
		numServed += 1
		item := payload.Items[0]
		log.Printf("Serving payload %d/%d: %#v", numServed, numPayloadItems, item)
		payloadItem.HttpStatusCode = item.HttpStatusCode

		if (item.Timeout != nil && *item.Timeout) {
			log.Printf("Sleeping for %d seconds", TimeoutInSeconds)
			time.Sleep(TimeoutInSeconds * time.Second)
		}

		if (item.ResponseBody != nil) {
			if applicationHealthState, ok := (*item.ResponseBody)[ApplicationHealthStateResponseKey]; ok {
				if allowedHealthStatuses[HealthStatus(applicationHealthState)] {
					log.Printf("Sending response with app health state: %s", applicationHealthState)
				} else {
					log.Printf("Sending response with invalid app health state")
				}
			} else {
				log.Printf("Sending response with missing app health state")
			}
			w.Header().Set("Content-Type", "application/json")
			resp, err := json.Marshal(item.ResponseBody)
			if err != nil {
				log.Printf("Error happened in JSON marshal. Err: %s", err)
			}
			w.Write(resp)
		} else {
			log.Printf("Sending no response body with status code %v", payloadItem.HttpStatusCode)
		} 
	}
	payload.Items = payload.Items[1:]
		
	w.WriteHeader(payloadItem.HttpStatusCode)

	// if healthStates is non-empty, this means that the test is only meant to run till we iterate over all healthstates, so the servers are shutdown
	log.Printf("Check if we should exit: %v, %v", shouldExitOnEmptyHealthStates, len(payload.Items) == 0)
	if shouldExitOnEmptyHealthStates && len(payload.Items) == 0 {
		go shutdownServers(httpServer, httpsServer)
	}
}

func startServers(wg *sync.WaitGroup) (*http.Server, *http.Server) {
	httpMutex := http.NewServeMux()
	httpMutex.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		healthHandler(w,r,wg)
	})
	httpServer := http.Server{
		Handler: httpMutex,
		Addr: ":8080"
	}
	httpsServer := http.Server{
		Handler: httpMutex,
		Addr: ":443"
	}
	
	log.Printf("Starting server...")
	go func() {
		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("HttpServer.ListenAndServe(): %v", err)
		}
		if err := httpsServer.ListenAndServeTLS("webservercert.pem", "webserverkey.pem"); err != http.ErrServerClosed {
			log.Fatalf("HttpsServer.ListenAndServe(): %v", err)
		}
	}
	return httpServer, httpsServer
}

func shutdownServers(httpServer *http.Server, httpsServer *http.Server) error, error {
	log.Printf("Shutting down http and https server")
	httpServer.Shutdown(context.Background())
	httpsServer.Shutdown(context.Background())
}

func main() {
	payloadStr := flag.String("payload", "", "json string containing probe responses")
	flag.Parse()
	var originalPayload Payload
	if payloadStr != nil && *payloadStr != "" {
		*payloadStr = strings.TrimSpace(*payloadStr)
		log.Printf("Processing payload: %v", *payloadStr)
		if err := json.Unmarshal([]byte(*payloadStr), &originalPayload); err != nil {
			log.Printf("err: %v", err)
			return
		}
	}
	payload := originalPayload
	shouldExitOnEmptyHealthStates := len(payload.Items) > 0
	numServed := 0
	numPayloadItems := len(originalPayload.Items)
	
	wg:= &sync.WaitGroup{}
	wg.Add(numPayloadItems)
	
	httpServer, httpsServer := go startServers(wg)
	wg.Wait()

	log.Printf("Finished serving payload: %v", originalPayload)
	go shutdownServers(httpServer, httpsServer)
}
