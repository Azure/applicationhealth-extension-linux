package vmwatch

const (
	SubstatusKeyNameVMWatch = "VMWatch"

	VMWatchBinaryNameAmd64    = "vmwatch_linux_amd64"
	VMWatchBinaryNameArm64    = "vmwatch_linux_arm64"
	VMWatchConfigFileName     = "vmwatch.conf"
	VMWatchVerboseLogFileName = "vmwatch.log"
	VMWatchDefaultTests       = "disk_io:outbound_connectivity:clockskew:az_storage_blob"
	VMWatchMaxProcessAttempts = 3

	AppHealthBinaryNameArm64 = "applicationhealth-extension-arm64"
)
