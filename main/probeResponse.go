package main

import (
	"fmt"
	"encoding/json"
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
	CustomMetrics *string `json:"customMetrics,omitempty"`
}

func (p ProbeResponse) validate() error {
	if !allowedHealthStatuses[p.ApplicationHealthState] {
		return errors.New(fmt.Sprintf("Response body key '%s' has invalid value '%#v'", ApplicationHealthStateResponseKey, string(p.ApplicationHealthState)))
	}
    if p.CustomMetrics != nil {
		if !json.Valid([]byte(*p.CustomMetrics)) {
			return errors.New(fmt.Sprintf("Response body key '%s' has invalid json format '%#v'", CustomMetricsResponseKey, *p.CustomMetrics))
		}
	}
	return nil
}
