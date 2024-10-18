package vmwatch

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Azure/applicationhealth-extension-linux/internal/handlerenv"
	"github.com/Azure/applicationhealth-extension-linux/internal/telemetry"
	"github.com/containerd/cgroups/v3"
	"github.com/containerd/cgroups/v3/cgroup1"
	"github.com/containerd/cgroups/v3/cgroup2"
	"github.com/opencontainers/runtime-spec/specs-go"
)

func configureVMWatchProcess(lg *slog.Logger, attempt int, vmWatchSettings *VMWatchSettings, hEnv *handlerenv.HandlerEnvironment) (*exec.Cmd, bool, *bytes.Buffer, error) {
	// Setup command
	cmd, resourceGovernanceRequired, err := setupVMWatchCommand(lg, vmWatchSettings, hEnv)
	if err != nil {
		err = fmt.Errorf("[%v][PID -1] Attempt %d: VMWatch setup failed. Error: %w", time.Now().UTC().Format(time.RFC3339), attempt, err)
		telemetry.SendEvent(telemetry.ErrorEvent, telemetry.SetupVMWatchTask, err.Error())
		return nil, false, nil, err
	}
	// TODO: Combined output may get excessively long, especially since VMWatch is a long running process
	// We should trim the output or only get from Stderr
	combinedOutput := &bytes.Buffer{}
	cmd.Stdout = combinedOutput
	cmd.Stderr = combinedOutput
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGTERM}
	return cmd, resourceGovernanceRequired, combinedOutput, nil
}

func createVMWatchCommand(lg *slog.Logger, s *VMWatchSettings, hEnv *handlerenv.HandlerEnvironment, cmdPath string, args []string) (*exec.Cmd, bool) {
	var (
		cmd *exec.Cmd
		// flag to tell the caller that further resource governance is required by assigning to cgroups after the process is started
		// default to true to that if systemd-run is not available, we will assign cgroups
		resourceGovernanceRequired bool = true
	)

	if !isSystemdAvailable() {
		cmd = exec.Command(GetVMWatchBinaryFullPath(cmdPath), args...)
		cmd.Env = GetVMWatchEnvironmentVariables(lg, s.ParameterOverrides, hEnv)
		return cmd, resourceGovernanceRequired
	}

	// if we have systemd available, we will use that to launch the process, otherwise we will launch directly and manipulate our own cgroups
	systemdVersion := getSystemdVersion()
	systemdArgs := []string{"--scope", "-p", fmt.Sprintf("CPUQuota=%v%%", s.MaxCpuPercentage)}

	// systemd versions prior to 246 do not support MemoryMax, instead MemoryLimit should be used
	if systemdVersion < 246 {
		systemdArgs = append(systemdArgs, "-p", fmt.Sprintf("MemoryLimit=%v", s.MemoryLimitInBytes))
	} else {
		systemdArgs = append(systemdArgs, "-p", fmt.Sprintf("MemoryMax=%v", s.MemoryLimitInBytes))
	}

	// now append the env variables (--setenv is supported in all versions, -E only in newer versions)
	for _, v := range GetVMWatchEnvironmentVariables(lg, s.ParameterOverrides, hEnv) {
		systemdArgs = append(systemdArgs, "--setenv", v)
	}
	systemdArgs = append(systemdArgs, GetVMWatchBinaryFullPath(cmdPath))
	systemdArgs = append(systemdArgs, args...)

	// since systemd-run is in different paths on different distros, we will check for systemd but not use the full path
	// to systemd-run.  This is how guest agent handles it also so seems appropriate.
	cmd = exec.Command("systemd-run", systemdArgs...)
	// cgroup assignment not required since we are using systemd-run
	resourceGovernanceRequired = false
	return cmd, resourceGovernanceRequired
}

// Sets resource governance for VMWatch process, on linux, this is only to be used in the case where systemd-run is not available
func applyResourceGovernance(lg *slog.Logger, vmWatchSettings *VMWatchSettings, vmWatchCommand *exec.Cmd) error {
	// The default way to run vmwatch is via systemd-run.  There are some cases where system-run is not available
	// (in a container or in a distro without systemd).  In those cases we will manage the cgroups directly
	pid := vmWatchCommand.Process.Pid
	telemetry.SendEvent(telemetry.InfoEvent, telemetry.StartVMWatchTask, fmt.Sprintf("Applying resource governance to PID %d", pid))
	err := createAndAssignCgroups(lg, vmWatchSettings, pid)
	if err != nil {
		err = fmt.Errorf("[%v][PID %d] Failed to assign VMWatch process to cgroup. Error: %w", time.Now().UTC().Format(time.RFC3339), pid, err)
		telemetry.SendEvent(telemetry.ErrorEvent, telemetry.StartVMWatchTask, err.Error(), "error", err)
		// On real VMs we want this to stop vwmwatch from running at all since we want to make sure we are protected
		// by resource governance but on dev machines, we may fail due to limitations of execution environment (ie on dev container
		// or in a github pipeline container we don't have permission to assign cgroups (also on WSL environments it doesn't
		// work at all because the base OS doesn't support it)).
		// to allow us to run integration tests we will check the variables RUNING_IN_DEV_CONTAINER and
		// ALLOW_VMWATCH_GROUP_ASSIGNMENT_FAILURE and if they are both set we will just log and continue
		// this allows us to test both cases
		if os.Getenv(AllowVMWatchCgroupAssignmentFailureVariableName) == "" || os.Getenv(RunningInDevContainerVariableName) == "" {
			telemetry.SendEvent(telemetry.InfoEvent, telemetry.KillVMWatchTask, "Killing VMWatch process as cgroup assignment failed")
			_ = KillVMWatch(lg, vmWatchCommand)
			return err
		}
	}

	return nil
}

func createAndAssignCgroups(lg *slog.Logger, vmwatchSettings *VMWatchSettings, vmWatchPid int) error {
	// get our process and use this to determine the appropriate mount points for the cgroups
	myPid := os.Getpid()
	memoryLimitInBytes := int64(vmwatchSettings.MemoryLimitInBytes)
	telemetry.SendEvent(telemetry.InfoEvent, telemetry.StartVMWatchTask, "Assigning VMWatch process to cgroup")

	// check cgroups mode
	if cgroups.Mode() == cgroups.Unified {
		telemetry.SendEvent(telemetry.InfoEvent, telemetry.StartVMWatchTask, "cgroups v2 detected")
		// in cgroup v2, we need to set the period and quota relative to one another.
		// Quota is the number of microseconds in the period that process can run
		// Period is the length of the period in microseconds
		period := uint64(CGroupV2PeriodMs)
		cpuQuota := int64(vmwatchSettings.MaxCpuPercentage * 10000)
		resources := cgroup2.Resources{
			CPU: &cgroup2.CPU{
				Max: cgroup2.NewCPUMax(&cpuQuota, &period),
			},
			Memory: &cgroup2.Memory{
				Max: &memoryLimitInBytes,
			},
		}

		// in cgroup v2, it appears that a process already in a cgroup can't create a sub group that limits the same
		// kind of resources so we have to do it at the root level.  Reference https://manpath.be/f35/7/cgroups#L557
		manager, err := cgroup2.NewManager("/sys/fs/cgroup", "/vmwatch.slice", &resources)
		if err != nil {
			return err
		}
		err = manager.AddProc(uint64(vmWatchPid))
		if err != nil {
			return err
		}
	} else {
		telemetry.SendEvent(telemetry.InfoEvent, telemetry.StartVMWatchTask, "cgroups v1 detected")
		p := cgroup1.PidPath(myPid)

		cpuPath, err := p("cpu")
		if err != nil {
			return err
		}

		// in cgroup v1, the interval is implied, 1000 == 1 %
		cpuQuota := int64(vmwatchSettings.MaxCpuPercentage * 1000)
		memoryLimitInBytes := int64(vmwatchSettings.MemoryLimitInBytes)

		s := specs.LinuxResources{
			CPU: &specs.LinuxCPU{
				Quota: &cpuQuota,
			},
			Memory: &specs.LinuxMemory{
				Limit: &memoryLimitInBytes,
			},
		}

		control, err := cgroup1.New(cgroup1.StaticPath(cpuPath+"/vmwatch.slice"), &s)
		if err != nil {
			return err
		}
		err = control.AddProc(uint64(vmWatchPid))
		if err != nil {
			return err
		}

		defer control.Delete()
	}

	return nil
}

func generateEnvVarsForVMWatch(hEnv *handlerenv.HandlerEnvironment) []string {
	var (
		arr []string = make([]string, 0, 2)
	)
	arr = append(arr, fmt.Sprintf("SIGNAL_FOLDER=%s", hEnv.EventsFolder))
	arr = append(arr, fmt.Sprintf("VERBOSE_LOG_FILE_FULL_PATH=%s", filepath.Join(hEnv.LogFolder, VMWatchVerboseLogFileName)))
	return arr
}

func isSystemdAvailable() bool {
	// check if /run/systemd/system exists, if so we have systemd
	info, err := os.Stat("/run/systemd/system")
	return err == nil && info.IsDir()
}

func getSystemdVersion() int {
	cmd := exec.Command("systemd-run", "--version")

	// Execute the command and capture the output
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0
	}

	// Convert output bytes to string
	outputStr := string(output)

	// Find the version information in the output
	return extractVersion(outputStr)
}

// Function to extract the version information from the output
// returns the version or 0 if not found
func extractVersion(output string) int {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "systemd") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				ret, err := strconv.Atoi(parts[1])
				if err == nil {
					return ret
				}
				return 0
			}
		}
	}
	return 0
}
