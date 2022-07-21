package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"time"
	"strings"
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
	
	httpMutex := http.NewServeMux()
	httpServer := http.Server{Addr: ":8080", Handler: httpMutex}
	httpsServer := http.Server{Addr: ":443", Handler: httpMutex}

	// sends json resonse body with application health state expected by extension
	// looks at the first state in the healthStates array and dequeues that element after its iterated
	httpMutex.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		payloadItem := Item {
			HttpStatusCode: 200,
			Timeout: func() *bool { b := false; return &b }(),
			ResponseBody: nil,
		}

		log.Printf("Current payload: %v, len: %v", payload, len(payload.Items))
		if len(payload.Items) > 0 {
			item := payload.Items[0]
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
		if shouldExitOnEmptyHealthStates && len(payload.Items) == 0 {
			go func() {
				log.Printf("Finished serving payload: %v", originalPayload)
				log.Printf("Shutting down http and https server")
				httpServer.Shutdown(context.Background())
				httpsServer.Shutdown(context.Background())
			}()
		}
	})

	httpServer.ListenAndServe()
	httpsServer.ListenAndServeTLS("webservercert.pem", "webserverkey.pem")
}
