package main

const (
	SubstatusKeyNameAppHealthStatus        = "AppHealthStatus"
	SubstatusKeyNameApplicationHealthState = "ApplicationHealthState"
	SubstatusKeyNameCustomMetrics          = "CustomMetrics"
	SubstatusKeyNameVMWatch                = "VMWatch"

	ProbeResponseKeyNameApplicationHealthState = "ApplicationHealthState"
	ProbeResponseKeyNameCustomMetrics          = "CustomMetrics"

	AppHealthBinaryNameAmd64 = "applicationhealth-extension"
	AppHealthBinaryNameArm64 = "applicationhealth-extension-arm64"

	VMWatchBinaryNameAmd64 = "vmwatch_linux_amd64"
	VMWatchBinaryNameArm64 = "vmwatch_linux_arm64"
	VMWatchConfigFileName  = "vmwatch.conf"
	VMWatchVerboseLogFileName = "vmwatch.log"
)
