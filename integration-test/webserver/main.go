package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

const (
	TimeoutFlag                            = "t" // force request to timeout
	TimeoutInSeconds                       = 35
	ApplicationHealthStateMissingFlag      = "m" // send missing response body
	InvalidApplicationHealthStateValueFlag = "i" // send invalid value for health state

	ApplicationHealthStateResponseKey = "ApplicationHealthState" // Response body key name
)

// Flags passed to webserver in command line args to send correct health state values
var stateMap = map[string]string{
	"h": "Healthy",
	"u": "Unhealthy",
}

func main() {
	states := flag.String("states", "", "contains comma separated [2,3,4,5] representing status code x00 to send back combined with optional [h,u,t,m,i] for health state")
	flag.Parse()
	originalHealthStates := strings.Split(*states, ",")
	healthStates := strings.Split(*states, ",")
	var shouldExitOnEmptyHealthStates = len(healthStates) > 0

	httpMutex := http.NewServeMux()
	httpServer := http.Server{Addr: ":8080"}
	httpsServer := http.Server{Addr: ":443"}

	// sends json resonse body with application health state expected by extension
	// looks at the first state in the healthStates array and dequeues that element after its iterated
	httpMutex.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		var statusCode = 200
		response := make(map[string]string)
		log.Printf("Health States: %v, len: %v", healthStates, len(healthStates))
		if len(healthStates) > 0 && healthStates[0] != "" {
			strArr := []rune(healthStates[0])
			fmt.Sscan(string(strArr[0]), &statusCode)
			statusCode *= 100
			if len(strArr) > 1 {
				stateFlag := string(strArr[1])

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
			} else {
				log.Printf("Sending no response body with status code %v", statusCode)
			}
		}
		healthStates = healthStates[1:]

		w.WriteHeader(statusCode)
		w.Header().Set("Content-Type", "application/json")
		resp, err := json.Marshal(response)
		if err != nil {
			log.Printf("Error happened in JSON marshal. Err: %s", err)
		}
		w.Write(resp)

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

	httpServer.Handler = httpMutex
	httpsServer.Handler = httpMutex

	log.Printf("Starting http server...")
	go httpServer.ListenAndServe()
	log.Printf("Starting https server...")
	log.Fatal(httpsServer.ListenAndServeTLS("webservercert.pem", "webserverkey.pem"))
	log.Printf("Servers stopped...")
	log.Printf("Finished serving health states: %v", originalHealthStates)
}
