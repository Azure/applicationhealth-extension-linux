package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/Azure/applicationhealth-extension-linux/internal/telemetry"
)

// Package-level function variables to allow mocking in tests
var (
	findExistingProcesses = findExistingProcessesImpl
	killProcesses         = killProcessesImpl
	isAHEProcess          = isAHEProcessImpl
)

// procExePath returns the path to the /proc/<pid>/exe symlink for the given PID.
func procExePath(pid int) string {
	return filepath.Join("/proc", strconv.Itoa(pid), "exe")
}

// isAHEProcessImpl checks whether the given PID belongs to an Application Health Extension
// binary by reading /proc/<pid>/exe. Returns true if it matches a known AHE binary name.
func isAHEProcessImpl(pid int) bool {
	exePath, err := os.Readlink(procExePath(pid))
	if err != nil {
		return false
	}
	procName := filepath.Base(exePath)
	return procName == AppHealthBinaryNameAmd64 || procName == AppHealthBinaryNameArm64
}

// findExistingProcessesImpl scans /proc to find all other running instances of the
// Application Health Extension binary (excluding the current process).
// Uses /proc/<pid>/exe for binary identification.
// Returns a slice of PIDs of existing processes (empty if none found).
func findExistingProcessesImpl() ([]int, error) {
	myPid := os.Getpid()
	var pids []int

	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, fmt.Errorf("failed to read /proc: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid == myPid {
			continue
		}

		if isAHEProcess(pid) {
			pids = append(pids, pid)
		}
	}

	return pids, nil
}

// getLogFileLastWriteTimeFromEnv reads the handler log file's last write time from the
// HANDLER_LOG_LAST_WRITE_TIME environment variable. The shim captures this value
// before redirecting output to handler.log, so it reflects the previous process's
// last write time rather than the current invocation's writes.
// Returns the timestamp and nil on success, or zero time and an error if the env var
// is not set or cannot be parsed.
func getLogFileLastWriteTimeFromEnv() (time.Time, error) {
	envTime := os.Getenv("HANDLER_LOG_LAST_WRITE_TIME")
	if envTime == "" {
		return time.Time{}, fmt.Errorf("HANDLER_LOG_LAST_WRITE_TIME not set, log file may not exist")
	}
	epochSec, err := strconv.ParseInt(envTime, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse HANDLER_LOG_LAST_WRITE_TIME=%q: %w", envTime, err)
	}
	return time.Unix(epochSec, 0), nil
}

// killProcessesImpl sends SIGTERM to all specified processes and waits for each to exit.
// Logs a warning for any process that cannot be killed but continues with the rest.
func killProcessesImpl(pids []int) {
	for _, pid := range pids {
		if err := killProcess(pid); err != nil {
			telemetry.SendEvent(telemetry.WarningEvent, telemetry.AppHealthTask,
				fmt.Sprintf("Failed to terminate existing process %d: %v", pid, err),
				"pid", pid, "error", err)
		}
	}
}

// killProcess sends SIGTERM to the specified process and waits (bounded) for it
// to exit before returning. This prevents a race where two AHE instances run
// simultaneously during takeover.
func killProcess(pid int) error {
	// Revalidate that the PID still belongs to AHE before killing to guard
	// against PID reuse between discovery and kill time.
	if !isAHEProcess(pid) {
		telemetry.SendEvent(telemetry.WarningEvent, telemetry.AppHealthTask,
			fmt.Sprintf("PID %d is no longer an AHE process at kill time, skipping", pid),
			"pid", pid)
		return nil
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process %d: %w", pid, err)
	}

	if err := process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send SIGTERM to process %d: %w", pid, err)
	}

	telemetry.SendEvent(telemetry.InfoEvent, telemetry.AppHealthTask,
		fmt.Sprintf("Sent SIGTERM to existing AHE process with PID %d, waiting for exit", pid))

	// Wait up to 5 seconds for the process to exit
	for i := 0; i < 10; i++ {
		if err := process.Signal(syscall.Signal(0)); err != nil {
			// Process is gone
			telemetry.SendEvent(telemetry.InfoEvent, telemetry.AppHealthTask,
				fmt.Sprintf("Existing AHE process with PID %d has exited", pid))
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	// SIGTERM did not work — escalate to SIGKILL
	telemetry.SendEvent(telemetry.WarningEvent, telemetry.AppHealthTask,
		fmt.Sprintf("Existing AHE process with PID %d did not exit within 5 seconds after SIGTERM, sending SIGKILL", pid))
	if err := process.Signal(syscall.SIGKILL); err != nil {
		return fmt.Errorf("failed to send SIGKILL to process %d: %w", pid, err)
	}

	// Wait up to 2 seconds for the process to die after SIGKILL
	for i := 0; i < 4; i++ {
		time.Sleep(500 * time.Millisecond)
		if err := process.Signal(syscall.Signal(0)); err != nil {
			telemetry.SendEvent(telemetry.InfoEvent, telemetry.AppHealthTask,
				fmt.Sprintf("Existing AHE process with PID %d has exited after SIGKILL", pid))
			return nil
		}
	}

	return fmt.Errorf("process %d did not exit after SIGKILL within 2 seconds", pid)
}
