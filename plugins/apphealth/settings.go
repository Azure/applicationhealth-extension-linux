package apphealth

import (
	"github.com/Azure/applicationhealth-extension-linux/plugins/settings"
)

type AppHealthSettings struct {
	protocol          string
	port              int
	requestPath       string
	intervalInSeconds int
	numberOfProbes    int
	gracePeriod       int
}

func NewAppHealthSettings(hs *settings.HandlerSettings) *AppHealthSettings {
	return &AppHealthSettings{
		protocol:          hs.Protocol,
		port:              hs.Port,
		requestPath:       hs.RequestPath,
		intervalInSeconds: hs.IntervalInSeconds,
		numberOfProbes:    hs.NumberOfProbes,
		gracePeriod:       hs.GracePeriod,
	}
}

func (s *AppHealthSettings) GetProtocol() string {
	return s.protocol
}

func (s *AppHealthSettings) GetRequestPath() string {
	return s.requestPath
}

func (s *AppHealthSettings) GetPort() int {
	return s.port
}

func (s *AppHealthSettings) GetIntervalInSeconds() int {
	if s.intervalInSeconds == 0 {
		return defaultIntervalInSeconds
	} else {
		return s.intervalInSeconds
	}
}

func (s *AppHealthSettings) GetNumberOfProbes() int {
	if s.numberOfProbes == 0 {
		return defaultNumberOfProbes
	} else {
		return s.numberOfProbes
	}
}

func (s *AppHealthSettings) GetGracePeriod() int {
	if s.gracePeriod == 0 {
		return s.GetIntervalInSeconds() * s.GetNumberOfProbes()
	} else {
		return s.gracePeriod
	}
}
