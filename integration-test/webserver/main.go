package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

const (
	// Response body key names
	ResponseBodyKeyApplicationHealthState = "ApplicationHealthState"
	ResponseBodyKeyCustomMetrics          = "CustomMetrics"

	// Application Health State flags
	ApplicationHealthStateInvalidFlag = "i"
	ResponseTimeoutFlag               = "t"
	ResponseTimeoutInSeconds          = 35

	// Custom Metrics flags
	CustomMetricsValidFlag       = "valid"
	CustomMetricsInvalidFlag     = "invalid"
	CustomMetricsNilFlag         = "nil"
	CustomMetricsEmptyFlag       = "empty"
	CustomMetricsEmptyObjectFlag = "emptyobj"

	CustomMetricsValidValue       = `{"rollingUpgradePolicy": { "phase": 2, "doNotUpgrade": true, "dummy": "yes" } }`
	CustomMetricsInvalidValue     = `[ "hello", "world" ]`
	CustomMetricsEmptyValue       = ""
	CustomMetricsEmptyObjectValue = "{}"
)

// Flags passed to webserver in command line args to send correct health state values
var healthStateFlagMapping = map[string]string{
	"h": "Healthy",
	"u": "Unhealthy",
}

func HandleFlag(flagStr string) (int, map[string]interface{}) {
	statusCode := 200
	responseBody := make(map[string]interface{})
	if flagStr == "" {
		return statusCode, responseBody
	}

	flags := strings.Split(flagStr, "-")
	statusCodeAndHealthStateFlags := []rune(flags[0])

	// E.g '3' -> StatusCode: 300
	statusCode = HandleStatusCodeFlag(string(statusCodeAndHealthStateFlags[0]))

	// E.g '2h' -> StatusCode: 200, ResponseBody: { "ApplicationHealthState" : "Healthy" }
	if healthStateOrTimeoutFlag := ApplicationHealthStateFlagPresent(flags); healthStateOrTimeoutFlag != "" {
		switch healthStateOrTimeoutFlag {
		case ResponseTimeoutFlag:
			log.Printf("Sleeping for %d seconds", ResponseTimeoutInSeconds)
			time.Sleep(ResponseTimeoutInSeconds * time.Second)
		default:
			key, value := HandleApplicationHealthStateFlag(healthStateOrTimeoutFlag)
			responseBody[key] = value
		}
	}

	// E.g '2h-valid' -> StatusCode: 200, ResponseBody: { "ApplicationHealthState" : "Healthy", "CustomMetrics": "<a raw json string>" }
	if customMetricFlag := CustomMetricsFlagPresent(flags); customMetricFlag != "" {
		key, value := HandleCustomMetricFlag(customMetricFlag)
		responseBody[key] = value
	}

	if len(responseBody) == 0 {
		log.Printf("Sending no response body with status code %v", statusCode)
	} else {
		log.Printf("Sending response body with status code %v: %v", statusCode, responseBody)
	}

	return statusCode, responseBody
}

func ApplicationHealthStateFlagPresent(flags []string) string {
	statusCodeAndHealthStateFlags := []rune(flags[0])
	if len(statusCodeAndHealthStateFlags) > 1 {
		return string(statusCodeAndHealthStateFlags[1])
	}
	return ""
}

func CustomMetricsFlagPresent(flags []string) string {
	if len(flags) == 2 {
		return flags[1]
	}
	return ""
}

func HandleStatusCodeFlag(flag string) int {
	var statusCode int
	fmt.Sscan(flag, &statusCode)
	statusCode *= 100
	return statusCode
}

func HandleApplicationHealthStateFlag(flag string) (string, string) {
	switch flag {
	case ApplicationHealthStateInvalidFlag:
		return ResponseBodyKeyApplicationHealthState, "Hello!"

	default:
		return ResponseBodyKeyApplicationHealthState, getHealthState(flag)
	}
}

func HandleCustomMetricFlag(flag string) (string, interface{}) {
	switch flag {
	case CustomMetricsValidFlag:
		return ResponseBodyKeyCustomMetrics, CustomMetricsValidValue

	case CustomMetricsInvalidFlag:
		return ResponseBodyKeyCustomMetrics, CustomMetricsInvalidValue

	case CustomMetricsNilFlag:
		return ResponseBodyKeyCustomMetrics, nil

	case CustomMetricsEmptyFlag:
		return ResponseBodyKeyCustomMetrics, CustomMetricsEmptyValue

	case CustomMetricsEmptyObjectFlag:
		return ResponseBodyKeyCustomMetrics, CustomMetricsEmptyObjectValue
	}

	return "Hello", "world"
}

func healthHandler(w http.ResponseWriter, r *http.Request, arguments *[]string) {
	log.Printf("Arguments: %v, len: %v", arguments, len(*arguments))
	statusCode, responseBody := HandleFlag((*arguments)[0])
	*arguments = (*arguments)[1:]

	w.WriteHeader(statusCode)
	w.Header().Set("Content-Type", "application/json")
	respBody, err := json.Marshal(responseBody)
	if err != nil {
		log.Printf("Error happened in JSON marshal. Err: %s", err)
	}
	w.Write(respBody)
}

func getSecurityProtocolVersion(securityProtocol string) uint16 {

	switch securityProtocol {
	case "ssl3.0":
		return tls.VersionSSL30
	case "tls1.0":
		return tls.VersionTLS10
	case "tls1.1":
		return tls.VersionTLS11
	case "tls1.2":
		return tls.VersionTLS12
	case "tls1.3":
		return tls.VersionTLS13
	default:
		return tls.VersionTLS13
	}
}

func getHealthState(flag string) string {

	if healthState, ok := healthStateFlagMapping[flag]; ok {
		return healthState
	} else {
		return ""
	}
}

func main() {
	args := flag.String("args", "", `Example usage: '2h-valid' to send StatusCode: 200, ResponseBody: { "ApplicationHealthState": "Healthy", "CustomMetrics": "<valid json>"}`)
	securityProtocol := flag.String("securityProtocol", "tls1.3", "Specifies the security protocol to use for the HTTPS server. Valid options are: tls1.0, tls1.1, tls1.2, tls1.3, ssl3.0. Default is tls1.3.")
	flag.Parse()
	originalArgs := strings.Split(*args, ",")
	arguments := strings.Split(*args, ",")
	var shouldExitOnEmptyArgs = len(arguments) > 0

	httpMutex := http.NewServeMux()
	httpServer := http.Server{
		Addr:    ":8080",
		Handler: httpMutex}

	httpsServer := http.Server{
		Addr:    ":4430", //changing default port from 443 to 4430 to avoid conflict with other services
		Handler: httpMutex,
		TLSConfig: &tls.Config{
			MinVersion: getSecurityProtocolVersion(*securityProtocol),
			MaxVersion: getSecurityProtocolVersion(*securityProtocol)},
	}

	// sends json resonse body with application health state expected by extension
	// looks at the first state in the healthStates array and dequeues that element after its iterated
	httpMutex.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		healthHandler(w, r, &arguments)

		// if arguments is non-empty, this means that the test is only meant to run till we iterate over all arguments, so the servers are shutdown
		if shouldExitOnEmptyArgs && len(arguments) == 0 {
			go func() {
				log.Printf("Finished serving arguments: %v", originalArgs)
				log.Printf("Shutting down http and https server")
				httpServer.Shutdown(context.Background())
				httpsServer.Shutdown(context.Background())
			}()
		}
	})

	log.Printf("Arguments: %v, len: %v", arguments, len(arguments))
	log.Printf("Starting http server...")
	go httpServer.ListenAndServe()
	log.Printf("Starting https server...")
	log.Fatal(httpsServer.ListenAndServeTLS("webservercert.pem", "webserverkey.pem"))
	log.Printf("Servers stopped...")
}
