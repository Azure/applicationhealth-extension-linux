package main

import (
	"fmt"
	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
)

var (
	healthStatusesAllowedInProbeResponse = map[HealthStatus]bool{
		Healthy:   true,
		Unhealthy: true,
		Busy:      true,
	}
	healthStatusesAllowedToBypassGracePeriod = map[HealthStatus]bool{
		Healthy:   true,
		Busy:      true,
	}
)

type ProbeResponse struct {
	ApplicationHealthState HealthStatus `json:"applicationHealthState"`
}

func (p ProbeResponse) validate(ctx *log.Context) error {
	if !healthStatusesAllowedInProbeResponse[p.ApplicationHealthState] {
		return errors.New(fmt.Sprintf("Invalid value '%s' for '%s'", string(p.ApplicationHealthState), ApplicationHealthStateResponseKey))
	}
	return nil
}
