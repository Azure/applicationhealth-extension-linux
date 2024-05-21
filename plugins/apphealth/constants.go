package apphealth

const (
	SubstatusKeyNameAppHealthStatus            = "AppHealthStatus"
	SubstatusKeyNameApplicationHealthState     = "ApplicationHealthState"
	SubstatusKeyNameCustomMetrics              = "CustomMetrics"
	ProbeResponseKeyNameApplicationHealthState = "ApplicationHealthState"
	ProbeResponseKeyNameCustomMetrics          = "CustomMetrics"

	defaultIntervalInSeconds = 5
	defaultNumberOfProbes    = 1
	maximumProbeSettleTime   = 240
)
