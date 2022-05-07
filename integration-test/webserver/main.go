package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"strings"
	"time"
)

const (
	TimeoutFlag                            = "t" // force request to timeout
	TimeoutInSeconds                       = 60
	ApplicationHealthStateMissingFlag      = "m" // send missing response body
	InvalidApplicationHealthStateValueFlag = "x" // send invalid value for health state

	ApplicationHealthStateResponseKey = "ApplicationHealthState" // Response body key name
)

// Flags passed to webserver in command line args to send correct health state values
var stateMap = map[string]string{
	"i":   "Initializing",
	"h":   "Healthy",
	"d":   "Draining",
	"unk": "Unknown",
	"di":  "Disabled",
	"b":   "Busy",
	"u":   "Unhealthy",
}

func main() {
	states := flag.String("states", "", "contains comma separated [i, h, d, unk, di, b, u] repesenting [initializing, healthy, draining, unknown, disabled, busy, unhealthy]")
	flag.Parse()
	originalHealthStates := strings.Split(*states, ",")
	healthStates := strings.Split(*states, ",")
	var shouldExitOnEmptyHealthStates = len(healthStates) > 0
	httpMutex := http.NewServeMux()
	httpServer := http.Server{Addr: ":8080", Handler: httpMutex}
	httpsServer := http.Server{Addr: ":443", Handler: httpMutex}

	// sends json resonse body with application health state expected by extension
	// looks at the first state in the healthStates array and dequeues that element after its iterated
	httpMutex.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if len(healthStates) > 0 {
			stateFlag := healthStates[0]
			log.Printf("Serving health state %s: %v", stateFlag, stateMap[stateFlag])
			healthStates = healthStates[1:]

			response := make(map[string]string)
			switch stateFlag {
			case TimeoutFlag:
				log.Printf("Sleeping for %d seconds", TimeoutInSeconds)
				time.Sleep(TimeoutInSeconds * time.Second)

			case ApplicationHealthStateMissingFlag:
				log.Printf("Sending response with missing app health state")
				response["Hello"] = "World"

			case InvalidApplicationHealthStateValueFlag:
				log.Printf("Sending response with invalid app health state")
				response[ApplicationHealthStateResponseKey] = "Hello!"

			default:
				log.Printf("Sending response with app health state: %s", stateMap[stateFlag])
				response[ApplicationHealthStateResponseKey] = stateMap[stateFlag]
			}
			w.Header().Set("Content-Type", "application/json")
			resp, err := json.Marshal(response)
			if err != nil {
				log.Printf("Error happened in JSON marshal. Err: %s", err)
			}
			w.Write(resp)
		}

		// if healthStates is non-empty, this means that the test is only meant to run till we iterate over all healthstates, so the servers are shutdown
		if shouldExitOnEmptyHealthStates && len(healthStates) == 0 {
			go func() {
				log.Printf("Finished serving health states: %v", originalHealthStates)
				log.Printf("Shutting down http and https server")
				httpServer.Shutdown(context.Background())
				httpsServer.Shutdown(context.Background())
			}()
		}
	})

	go httpServer.ListenAndServe()
	httpsServer.ListenAndServeTLS("webservercert.pem", "webserverkey.pem")
}
