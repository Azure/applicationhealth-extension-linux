package vmwatch

const (
	SubstatusKeyNameVMWatch = "VMWatch"

	VMWatchConfigFileName     = "vmwatch.conf"
	VMWatchVerboseLogFileName = "vmwatch.log"
	VMWatchDefaultTests       = "disk_io:outbound_connectivity:clockskew:az_storage_blob"
	VMWatchMaxProcessAttempts = 3
)
