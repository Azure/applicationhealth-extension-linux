package apphealth

import "github.com/pkg/errors"

var (
	ErrTcpMustNotIncludeRequestPath    = errors.New("'requestPath' cannot be specified when using 'tcp' protocol")
	ErrTcpConfigurationMustIncludePort = errors.New("'port' must be specified when using 'tcp' protocol")
	ErrProbeSettleTimeExceedsThreshold = errors.New("Probe settle time (intervalInSeconds * numberOfProbes) cannot exceed 240 seconds")
)

// AppHealthPluginSettings holds the configuration of the AppHealth plugin, It will belong as an embedded field in the publicSettings struct.
// Which will be deserialized from public configuration section of the extension handler.
// This should be in sync with publicSettingsSchema from platform/schema/schema.go.
type AppHealthPluginSettings struct {
	Protocol          string `json:"protocol"`
	Port              int    `json:"port,int"`
	RequestPath       string `json:"requestPath"`
	IntervalInSeconds int    `json:"intervalInSeconds,int"`
	NumberOfProbes    int    `json:"numberOfProbes,int"`
	GracePeriod       int    `json:"gracePeriod,int"`
}

func (s *AppHealthPluginSettings) GetProtocol() string {
	return s.Protocol
}

func (s *AppHealthPluginSettings) GetRequestPath() string {
	return s.RequestPath
}

func (s *AppHealthPluginSettings) GetPort() int {
	return s.Port
}

func (s *AppHealthPluginSettings) GetIntervalInSeconds() int {
	var intervalInSeconds = s.IntervalInSeconds
	if intervalInSeconds == 0 {
		return defaultIntervalInSeconds
	} else {
		return intervalInSeconds
	}
}

func (s *AppHealthPluginSettings) GetNumberOfProbes() int {
	var numberOfProbes = s.NumberOfProbes
	if numberOfProbes == 0 {
		return defaultNumberOfProbes
	} else {
		return numberOfProbes
	}
}

func (s *AppHealthPluginSettings) GetGracePeriod() int {
	var gracePeriod = s.GracePeriod
	if gracePeriod == 0 {
		return s.GetIntervalInSeconds() * s.GetNumberOfProbes()
	} else {
		return gracePeriod
	}
}

// validate makes logical validation on the handlerSettings which already passed
// the schema validation.
func (s AppHealthPluginSettings) Validate() error {
	if s.GetProtocol() == "tcp" && s.GetPort() == 0 {
		return ErrTcpConfigurationMustIncludePort
	}

	if s.GetProtocol() == "tcp" && s.GetRequestPath() != "" {
		return ErrTcpMustNotIncludeRequestPath
	}

	probeSettlingTime := s.GetIntervalInSeconds() * s.GetNumberOfProbes()
	if probeSettlingTime > maximumProbeSettleTime {
		return ErrProbeSettleTimeExceedsThreshold
	}

	return nil
}
