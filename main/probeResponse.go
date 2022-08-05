package main

import (
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
)

var (
	allowedHealthStatuses = map[HealthStatus]bool{
		Healthy:   true,
		Unhealthy: true,
	}
)

type ProbeResponse struct {
	applicationHealthState HealthStatus `json:"applicationHealthState"`
	customMetrics string `json:"customMetrics,omitempty"`
}

func (p ProbeResponse) validate() error {
	if !allowedHealthStatuses[p.applicationHealthState] {
		return errors.New(fmt.Sprintf("Response body key '%s' has invalid value '%s'", ProbeResponseKeyNameApplicationHealthState, string(p.applicationHealthState)))
	}
	if p.customMetrics != "" {
		var js map[string]interface{}
		if json.Unmarshal([]byte(p.customMetrics), &js) != nil {
			return errors.New(fmt.Sprintf("Response body key '%s' value is not a valid json object: '%s'", ProbeResponseKeyNameCustomMetrics, p.customMetrics))
		}
	}
	return nil
}
