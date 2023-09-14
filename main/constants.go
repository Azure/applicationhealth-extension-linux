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

	// TODO: The github package responsible for HandlerEnvironment settings is no longer being maintained
	// and it also doesn't have the latest properties like EventsFolder. Importing a separate package
	// is possible, but may result in lots of code churn. We will temporarily keep this as a constant since the
	// events folder is unlikely to change in the future.
	HandlerEnvironmentEventsFolderPath = "/var/log/azure/Microsoft.ManagedServices.ApplicationHealthLinux/events"

	VMWatchBinaryNameAmd64    = "vmwatch_linux_amd64"
	VMWatchBinaryNameArm64    = "vmwatch_linux_arm64"
	VMWatchConfigFileName     = "vmwatch.conf"
	VMWatchVerboseLogFileName = "vmwatch.log"
	VMWatchDefaultTests       = "disk_io:outbound_connectivity:clockskew:az_storage_blob"
	VMWatchMaxProcessAttempts = 3
)
