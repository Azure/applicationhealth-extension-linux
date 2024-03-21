package apphealth

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

func (p ProbeResponse) validateApplicationHealthState() error {
	if !allowedHealthStatuses[p.ApplicationHealthState] {
		return errors.New(fmt.Sprintf("Response body key '%s' has invalid value '%s'", ProbeResponseKeyNameApplicationHealthState, string(p.ApplicationHealthState)))
	}
	return nil
}

func (p ProbeResponse) ValidateCustomMetrics() error {
	if p.CustomMetrics != "" {
		var js map[string]interface{}
		if json.Unmarshal([]byte(p.CustomMetrics), &js) != nil {
			return errors.New(fmt.Sprintf("Response body key '%s' value is not a valid json object: '%s'", ProbeResponseKeyNameCustomMetrics, p.CustomMetrics))
		}
		if len(js) == 0 {
			return errors.New(fmt.Sprintf("Response body key '%s' value must not be an empty json object: '%s'", ProbeResponseKeyNameCustomMetrics, p.CustomMetrics))
		}
	}
	return nil
}
