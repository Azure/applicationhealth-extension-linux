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

	RecordAppHealthHeartBeatIntervalInMinutes = 5

	// Number of minutes to allow between log file updates before considering the existing process as unresponsive/stuck.
	// If the log file hasn't been updated within this interval, a new instance should take over.
	// This should be greater than RecordAppHealthHeartBeatIntervalInMinutes (5 min) to allow
	// for timing variations such as GC pauses, high CPU load, and cgroup throttling.
	AppHealthLogFileStaleThresholdInMinutes = 6

	// HandlerLogDir is the directory where the shim writes handler.log.
	HandlerLogDir = "/var/log/azure/applicationhealth-extension"

	// HandlerLogFile is the log file name written by the shim.
	HandlerLogFile = "handler.log"

	// TODO: The github package responsible for HandlerEnvironment settings is no longer being maintained
	// and it also doesn't have the latest properties like EventsFolder. Importing a separate package
	// is possible, but may result in lots of code churn. We will temporarily keep this as a constant since the
	// events folder is unlikely to change in the future.

	VMWatchBinaryNameAmd64    = "vmwatch_linux_amd64"
	VMWatchBinaryNameArm64    = "vmwatch_linux_arm64"
	VMWatchConfigFileName     = "vmwatch.conf"
	VMWatchVerboseLogFileName = "vmwatch.log"
	VMWatchDefaultTests       = "disk_io:outbound_connectivity:clockskew:az_storage_blob"
	VMWatchMaxProcessAttempts = 3
	VMWatchMaxRetryCycles     = 4
	VMWatchBaseWaitHours      = 3

	ExtensionManifestFileName = "manifest.xml"
)
