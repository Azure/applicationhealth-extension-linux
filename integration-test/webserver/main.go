package main

import (
    "flag"
    "log"
    "net/http"
    "strings"
    "context"
    "encoding/json"
)

var stateMap = map[string]string {
    "i": "initializing",
    "h": "healthy",
    "d": "draining",
    "unk": "unknown",
    "di": "disabled",
    "b": "busy",
    "u": "unhealthy",
} 

const ApplicationHealthStateResponseKey = "ApplicationHealthState" 

var (
    response = map[string]string {
        ApplicationHealthStateResponseKey: "",
    }
)

func main() {
    states := flag.String("states", "", "contains comma separated [i, h, d, unk, di, b, u] repesenting [initializing, healthy, draining, unknown, disabled, busy, unhealthy]")
    flag.Parse()
    healthStates := strings.Split(*states, ",")
    var shouldExitOnEmptyHealthStates = len(healthStates) > 0
    httpMutex := http.NewServeMux()
    httpServer := http.Server{Addr: ":8080", Handler: httpMutex }
    httpsServer := http.Server{Addr: ":443", Handler: httpMutex }

    // sends json resonse body with "appHealthState" = state
    // looks at the first state in the healthStates array and dequeues that element after its iterated
    httpMutex.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        if len(healthStates) > 0 {
            response[ApplicationHealthStateResponseKey] = stateMap[healthStates[0]]
            healthStates = healthStates[1:]
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
                log.Printf("Shutting down http and https server")                 
                httpServer.Shutdown(context.Background())
                httpsServer.Shutdown(context.Background())        
            }()
        }
    })

    go httpServer.ListenAndServe()
    httpsServer.ListenAndServeTLS("webservercert.pem", "webserverkey.pem")
}