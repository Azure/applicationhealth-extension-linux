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
	ApplicationHealthState HealthStatus `json:"applicationHealthState"`
	CustomMetrics          string       `json:"customMetrics,omitempty"`
}

func (p ProbeResponse) validate() error {
	if !allowedHealthStatuses[p.ApplicationHealthState] {
		return errors.New(fmt.Sprintf("Response body key '%s' has invalid value '%s'", ProbeResponseKeyNameApplicationHealthState, string(p.ApplicationHealthState)))
	}
	if p.CustomMetrics != "" {
		var js map[string]interface{}
		if json.Unmarshal([]byte(p.CustomMetrics), &js) != nil {
			return errors.New(fmt.Sprintf("Response body key '%s' value is not a valid json object: '%s'", ProbeResponseKeyNameCustomMetrics, p.CustomMetrics))
		}
	}
	return nil
}
