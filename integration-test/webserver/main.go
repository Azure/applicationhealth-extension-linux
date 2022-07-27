package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"net/http/httputil"
	"time"
	"strings"
	"sync"
	"fmt"
)

type Payload struct {
	Responses []Response	`json:"payload"`
}

type Response struct {
	HttpStatusCode	int					`json:"httpStatusCode"`
	Timeout 		*bool				`json:"timeout,omitempty"`
	ResponseBody	*map[string]string	`json:"responseBody,omitempty"`
}

func (r *Response) toString() string {
	var timeout = "nil"
	if r.Timeout != nil {
		timeout = fmt.Sprintf("%v", *r.Timeout)
	}

	var responseBody = "nil"
	if r.ResponseBody != nil {
		responseBody = fmt.Sprintf("%#v", *r.ResponseBody)
	}
	return fmt.Sprintf("HttpStatusCode: %d, Timeout: %s, ResponseBody: %s", r.HttpStatusCode, timeout, responseBody)
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
	numOfResponsesServed = 0
	numOfResponses = 1 // keep as 1, so that webserver can keep running if no payload is provided
)

const (
	TimeoutInSeconds                  	= 35
	ApplicationHealthStateResponseKey 	= "ApplicationHealthState"
	CustomMetricsResponseKey 			= "CustomMetrics"
)

func indexHandler(w http.ResponseWriter, r *http.Request) {
	reqDump, err := httputil.DumpRequest(r, true)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("REQUEST:\n%s", string(reqDump))
	w.Write([]byte("Hello World"))
}

func healthHandler(w http.ResponseWriter, r *http.Request, wg *sync.WaitGroup, payload *Payload) {
	defer wg.Done()

	response := Response {
		HttpStatusCode: 200,
		ResponseBody: nil,
	}

	if payload != nil && len((*payload).Responses) > 0 {
		numOfResponsesServed += 1
		payloadResp := payload.Responses[0]
		log.Printf("Serving payload %d/%d: %s", numOfResponsesServed, numOfResponses, payloadResp.toString())
		response.HttpStatusCode = payloadResp.HttpStatusCode

		if (payloadResp.Timeout != nil && *payloadResp.Timeout) {
			log.Printf("Sleeping for %d seconds", TimeoutInSeconds)
			time.Sleep(TimeoutInSeconds * time.Second)
		}

		if (payloadResp.ResponseBody != nil) {
			response.ResponseBody = payloadResp.ResponseBody
			if applicationHealthState, ok := (*payloadResp.ResponseBody)[ApplicationHealthStateResponseKey]; ok {
				if allowedHealthStatuses[HealthStatus(applicationHealthState)] {
					log.Printf("Sending response with app health state: %s", applicationHealthState)
				} else {
					log.Printf("Sending response with invalid app health state")
				}
			} else {
				log.Printf("Sending response with missing app health state")
			}

			if customMetrics, ok := (*payloadResp.ResponseBody)[CustomMetricsResponseKey]; ok {
				var js map[string]interface{}
				if json.Unmarshal([]byte(customMetrics), &js) == nil {
					log.Printf("Sending custom metrics with valid json object: %s", customMetrics)
				} else {
					log.Printf("Sending custom metrics with invalid json object: %s", customMetrics)
				}
			} else {
				log.Printf("Sending response with missing custom metrics")
			}
		} else {
			log.Printf("Sending no response body with status code %v", response.HttpStatusCode)
		} 
	}
	payload.Responses = payload.Responses[1:]
		
	w.WriteHeader(response.HttpStatusCode)
	if (response.ResponseBody != nil) {
		w.Header().Set("Content-Type", "application/json")
		respBody, err := json.Marshal(*response.ResponseBody)
		if err != nil {
			log.Printf("Error happened in JSON marshal. Err: %s", err)
		}
		w.Write(respBody)
	}
}

func startServers(httpServer *http.Server, httpsServer *http.Server, wg *sync.WaitGroup, payload *Payload) {
	log.Printf("Start servers...")
	if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
		log.Printf("HttpServer.ListenAndServe(): %v", err)
	}
	if err := httpsServer.ListenAndServeTLS("webservercert.pem", "webserverkey.pem"); err != http.ErrServerClosed {
		log.Printf("HttpsServer.ListenAndServe(): %v", err)
	}
}

func shutdownServers(httpServer *http.Server, httpsServer *http.Server) {
	log.Printf("Shutting down http and https server")
	httpServer.Shutdown(context.Background())
	httpsServer.Shutdown(context.Background())
}

func main() {
	payloadStr := flag.String("payload", "", "json string containing probe responses")
	flag.Parse()
	var payload Payload
	if payloadStr != nil && *payloadStr != "" {
		*payloadStr = strings.TrimSpace(*payloadStr)
		log.Printf("Processing payload: %v", *payloadStr)
		if err := json.Unmarshal([]byte(*payloadStr), &payload); err != nil {
			log.Printf("err: %v", err)
			return
		}
	}

	originalPayload := payload

	shouldExitOnEmptyHealthStates := len(payload.Responses) > 0
	if shouldExitOnEmptyHealthStates {
		numOfResponses = len(payload.Responses)
	}
	
	wg:= &sync.WaitGroup{}
	wg.Add(numOfResponses)

	httpMutex := http.NewServeMux()
	httpMutex.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		healthHandler(w, r, wg, &payload)
	})
	httpMutex.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		indexHandler(w, r)
	})
	httpServer := http.Server{
		Handler: httpMutex,
		Addr: ":8080",
	}
	httpsServer := http.Server{
		Handler: httpMutex,
		Addr: ":443",
	}
	
	go startServers(&httpServer, &httpsServer, wg, &payload)
	wg.Wait()

	// if healthStates is non-empty, this means that the test is only meant to run till we iterate over all healthstates, so the servers are shutdown
	// otherwise, we keep the server running
	if shouldExitOnEmptyHealthStates {
		log.Printf("Finished serving payload: %v", originalPayload)
		go shutdownServers(&httpServer, &httpsServer)
	}
	log.Printf("Webserver exiting...")
}
