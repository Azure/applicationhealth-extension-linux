package main

import (
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
}

func (p ProbeResponse) validate() error {
	if !allowedHealthStatuses[p.ApplicationHealthState] {
		return errors.New(fmt.Sprintf("Invalid value '%s' for '%s'", string(p.ApplicationHealthState), ApplicationHealthStateResponseKey))
	}
	return nil
}
