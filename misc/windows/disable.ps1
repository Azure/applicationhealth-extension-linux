$processName = "AppHealthExtension"
# all values in seconds
$totalTimeToWait = 60
$timeWaited = 0
$recheckAfter = 5
Do {
    $processes = Get-Process -Name $processName
    # If the process does not exist, break the loop
    if (!$?) {
        Break
    }
    Start-Sleep -Seconds $recheckAfter
    $timeWaited += $recheckAfter
} While ($timeWaited -lt $totalTimeToWait)

if($processes.Count -gt 0) {
    Stop-Process -Name $processName
}

$architecture = [System.Environment]::GetEnvironmentVariable("PROCESSOR_ARCHITECTURE", [System.EnvironmentVariableTarget]::Process)
$vmWatchProcessName = "vmwatch_windows_amd64"
if ($architecture -ieq "AMD64") {
    Write-Host "The architecture is 64-bit (x64) or AMD64."
} elseif ($architecture -ieq "ARM64") {
    Write-Host "The architecture is ARM64."
    $vmWatchProcessName = "vmwatch_windows_arm64"
} else {
    Write-Host "The architecture is not recognized or is not 64-bit."
    Exit
}

# all values in seconds
$totalTimeToWait = 60
$timeWaited = 0
$recheckAfter = 5
Do {
    $vmWatchProcesses = Get-Process -Name $vmWatchProcessName
    if (!$?) {
        Break
    }
    Start-Sleep -Seconds $recheckAfter
    $timeWaited += $recheckAfter
} While ($timeWaited -lt $totalTimeToWait)

if($vmWatchProcesses.Count -gt 0) {
    Stop-Process -InputObject $vmWatchProcesses
}
