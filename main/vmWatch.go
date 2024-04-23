package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/containerd/cgroups/v3"
	"github.com/containerd/cgroups/v3/cgroup1"
	"github.com/containerd/cgroups/v3/cgroup2"
	"github.com/go-kit/kit/log"
	"github.com/opencontainers/runtime-spec/specs-go"
)

type VMWatchStatus string

const (
	DefaultMaxCpuPercentage   = 1        // 1% cpu
	DefaultMaxMemoryInBytes   = 80000000 // 80MB
	HoursBetweenRetryAttempts = 3
	CGroupV2PeriodMs          = 1000000 // 1 second
)

const (
	NotRunning VMWatchStatus = "NotRunning"
	Disabled   VMWatchStatus = "Disabled"
	Running    VMWatchStatus = "Running"
	Failed     VMWatchStatus = "Failed"
)

const (
	AllowVMWatchCgroupAssignmentFailureVariableName string = "ALLOW_VMWATCH_CGROUP_ASSIGNMENT_FAILURE"
	RunningInDevContainerVariableName               string = "RUNNING_IN_DEV_CONTAINER"
	AppHealthExecutionEnvironmentProd               string = "Prod"
	AppHealthExecutionEnvironmentTest               string = "Test"
	AppHealthPublisherNameTest                      string = "Microsoft.ManagedServices.Edp"
)

func (p VMWatchStatus) GetStatusType() StatusType {
	switch p {
	case Disabled:
		return StatusWarning
	case Failed:
		return StatusError
	default:
		return StatusSuccess
	}
}

type VMWatchResult struct {
	Status VMWatchStatus
	Error  error
}

func (r *VMWatchResult) GetMessage() string {
	switch r.Status {
	case Disabled:
		return "VMWatch is disabled"
	case Failed:
		return fmt.Sprintf("VMWatch failed: %v", r.Error)
	case NotRunning:
		return "VMWatch is not running"
	default:
		return "VMWatch is running"
	}
}

// We will setup and execute VMWatch as a separate process. Ideally VMWatch should run indefinitely,
// but as a best effort we will attempt at most 3 times to run the process
func executeVMWatch(ctx *log.Context, s *vmWatchSettings, hEnv HandlerEnvironment, vmWatchResultChannel chan VMWatchResult) {
	var vmWatchErr error
	defer func() {
		if r := recover(); r != nil {
			vmWatchErr = fmt.Errorf("%w\n Additonal Details: %+v", vmWatchErr, r)
			ctx.Log("error", "Recovered %+v", r)
		}
		vmWatchResultChannel <- VMWatchResult{Status: Failed, Error: vmWatchErr}
		close(vmWatchResultChannel)
	}()

	// Best effort to start VMWatch process each time it fails start immediately up to VMWatchMaxProcessAttempts before waiting for
	// a longer time before trying again
	for !shutdown {
		for i := 1; i <= VMWatchMaxProcessAttempts && !shutdown; i++ {
			vmWatchResultChannel <- VMWatchResult{Status: Running}
			vmWatchErr = executeVMWatchHelper(ctx, i, s, hEnv)
			vmWatchResultChannel <- VMWatchResult{Status: Failed, Error: vmWatchErr}
		}
		ctx.Log("error", fmt.Sprintf("VMWatch reached max %d retries, sleeping for %v hours before trying again", VMWatchMaxProcessAttempts, HoursBetweenRetryAttempts))
		// we have exceeded the retries so now we go to sleep before starting again
		time.Sleep(time.Hour * HoursBetweenRetryAttempts)
	}
}

func executeVMWatchHelper(ctx *log.Context, attempt int, vmWatchSettings *vmWatchSettings, hEnv HandlerEnvironment) (err error) {
	pid := -1
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("Error: %w\n Additonal Details: %+v", err, r)
			ctx.Log("error", "Recovered %+v", r)
		}
	}()

	// Setup command
	runningInSystemd := false
	vmWatchCommand, runningInSystemd, err = setupVMWatchCommand(vmWatchSettings, hEnv)
	if err != nil {
		err = fmt.Errorf("[%v][PID -1] Attempt %d: VMWatch setup failed. Error: %w", time.Now().UTC().Format(time.RFC3339), attempt, err)
		ctx.Log("error", err.Error())
		return err
	}

	ctx.Log("event", fmt.Sprintf("Attempt %d: Setup VMWatch command: %s\nArgs: %v\nDir: %s\nEnv: %v\n", attempt, vmWatchCommand.Path, vmWatchCommand.Args, vmWatchCommand.Dir, vmWatchCommand.Env))

	// TODO: Combined output may get excessively long, especially since VMWatch is a long running process
	// We should trim the output or only get from Stderr
	combinedOutput := &bytes.Buffer{}
	vmWatchCommand.Stdout = combinedOutput
	vmWatchCommand.Stderr = combinedOutput
	vmWatchCommand.SysProcAttr = &syscall.SysProcAttr{ Pdeathsig: syscall.SIGTERM }

	// Start command
	if err := vmWatchCommand.Start(); err != nil {
		err = fmt.Errorf("[%v][PID -1] Attempt %d: VMWatch failed to start. Error: %w\nOutput: %s", time.Now().UTC().Format(time.RFC3339), attempt, err, combinedOutput.String())
		ctx.Log("error", err.Error())
		return err
	}
	pid = vmWatchCommand.Process.Pid // cmd.Process should be populated on success
	ctx.Log("event", fmt.Sprintf("Attempt %d: VMWatch process started with pid %d", attempt, pid))

	// The default way to run vmwatch is via systemd-run.  There are some cases where system-run is not available
	// (in a container or in a distro without systemd).  In those cases we will manage the cgroups directly
	if (!runningInSystemd) {
		err = createAndAssignCgroups(ctx, vmWatchSettings, pid)
		if err != nil {
			err = fmt.Errorf("[%v][PID %d] Failed to assign VMWatch process to cgroup. Error: %w", time.Now().UTC().Format(time.RFC3339), pid, err)
			ctx.Log("error", err.Error())
			// On real VMs we want this to stop vwmwatch from runing at all since we want to make sure we are protected
			// by resource governance but on dev machines, we may fail due to limitations of execution environment (ie on dev container
			// or in a github pipeline container we don't have permission to assign cgroups (also on WSL environments it doesn't
			// work at all because the base OS doesn't support it).
			// to allow us to run integration tests we will check the variables RUNING_IN_DEV_CONTAINER and
			// ALLOW_VMWATCH_GROUP_ASSIGNMENT_FAILURE and if they are both set we will just log and continue
			// this allows us to test both cases
			if os.Getenv(AllowVMWatchCgroupAssignmentFailureVariableName) == "" || os.Getenv(RunningInDevContainerVariableName) == "" {
				ctx.Log("event", fmt.Sprintf("Killing VMWatch process as cgroup assignment failed"))
				_ = killVMWatch(ctx, vmWatchCommand)
				return err
			}
		}
	}

	processDone := make(chan bool)

	// create a waitgroup to coordinate the goroutines
	var wg sync.WaitGroup
	// add a task to wait for process completion
	wg.Add(1)
	go func() {
		defer wg.Done()
		err = vmWatchCommand.Wait()
		processDone <- true
		close(processDone)
	}()
	// add a task to monitor heartbeat
	wg.Add(1)
	go func() {
		defer wg.Done()
		monitorHeartBeat(ctx, GetVMWatchHeartbeatFilePath(hEnv), processDone, vmWatchCommand)
	}()
	wg.Wait()
	err = fmt.Errorf("[%v][PID %d] Attempt %d: VMWatch process exited. Error: %w\nOutput: %s", time.Now().UTC().Format(time.RFC3339), pid, attempt, err, combinedOutput.String())
	ctx.Log("error", err.Error())
	return err
}

func monitorHeartBeat(ctx *log.Context, heartBeatFile string, processDone chan bool, cmd *exec.Cmd) {
	maxTimeBetweenHeartBeatsInSeconds := 60

	timer := time.NewTimer(time.Second * time.Duration(maxTimeBetweenHeartBeatsInSeconds))

	for {
		select {
		case <-timer.C:
			info, err := os.Stat(heartBeatFile)
			if err == nil && time.Since(info.ModTime()).Seconds() < float64(maxTimeBetweenHeartBeatsInSeconds) {
				// heartbeat was updated
			} else {
				// heartbeat file was not updated within 60 seconds, process is hung
				err = fmt.Errorf("[%v][PID %d] VMWatch process did not update heartbeat file within the time limit, killing the process", time.Now().UTC().Format(time.RFC3339), cmd.Process.Pid)
				ctx.Log("error", err.Error())
				err = killVMWatch(ctx, cmd)
				if err != nil {
					err = fmt.Errorf("[%v][PID %d] Failed to kill vmwatch process", time.Now().UTC().Format(time.RFC3339), cmd.Process.Pid)
					ctx.Log("error", err.Error())
				}
			}
		case <-processDone:
			return
		}
	}
}

func killVMWatch(ctx *log.Context, cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil || cmd.ProcessState != nil {
		ctx.Log("event", "VMWatch is not running, killing process is not necessary.")
		return nil
	}

	if err := cmd.Process.Kill(); err != nil {
		ctx.Log("error", fmt.Sprintf("Failed to kill VMWatch process with PID %d. Error: %v", cmd.Process.Pid, err))
		return err
	}

	ctx.Log("event", fmt.Sprintf("Successfully killed VMWatch process with PID %d", cmd.Process.Pid))
	return nil
}

// setupVMWatchCommand sets up the command to run VMWatch
// Returns the command and a boolean indicating if the command is run with systemd-run or directly
// if the boolean is true, the command is run with systemd-run, otherwise it is run directly
func setupVMWatchCommand(s *vmWatchSettings, hEnv HandlerEnvironment) (*exec.Cmd, bool, error) {
	processDirectory, err := GetProcessDirectory()
	if err != nil {
		return nil, false, err
	}

	args := []string{"--config", GetVMWatchConfigFullPath(processDirectory)}
	args = append(args, "--debug")
	args = append(args, "--heartbeat-file", GetVMWatchHeartbeatFilePath(hEnv))
	args = append(args, "--execution-environment", GetExecutionEnvironment(hEnv))

	// 0 is the default so allow that but any value below 30MB is not allowed
	if s.MemoryLimitInBytes == 0 {
		s.MemoryLimitInBytes = DefaultMaxMemoryInBytes
	
	}
	if s.MemoryLimitInBytes < 30000000 {
		err = fmt.Errorf("[%v] Invalid MemoryLimitInBytes specified must be at least 30000000", time.Now().UTC().Format(time.RFC3339))
		return nil, false, err
	}

	// check cpu, if 0 (default) set to the default value
	if (s.MaxCpuPercentage == 0) {
		s.MaxCpuPercentage = DefaultMaxCpuPercentage
	}

	if s.MaxCpuPercentage < 0 || s.MaxCpuPercentage > 100 {
		err = fmt.Errorf("[%v] Invalid maxCpuPercentage specified must be between 0 and 100", time.Now().UTC().Format(time.RFC3339))
		return nil, false, err
	}

	args = append(args, "--memory-limit-bytes", strconv.FormatInt(s.MemoryLimitInBytes, 10))

	if s.SignalFilters != nil {
		if s.SignalFilters.DisabledSignals != nil && len(s.SignalFilters.DisabledSignals) > 0 {
			args = append(args, "--disabled-signals")
			args = append(args, strings.Join(s.SignalFilters.DisabledSignals, ":"))
		}

		if s.SignalFilters.DisabledTags != nil && len(s.SignalFilters.DisabledTags) > 0 {
			args = append(args, "--disabled-tags")
			args = append(args, strings.Join(s.SignalFilters.DisabledTags, ":"))
		}

		if s.SignalFilters.EnabledTags != nil && len(s.SignalFilters.EnabledTags) > 0 {
			args = append(args, "--enabled-tags")
			args = append(args, strings.Join(s.SignalFilters.EnabledTags, ":"))
		}

		if s.SignalFilters.EnabledOptionalSignals != nil && len(s.SignalFilters.EnabledOptionalSignals) > 0 {
			args = append(args, "--enabled-optional-signals")
			args = append(args, strings.Join(s.SignalFilters.EnabledOptionalSignals, ":"))
		}
	}

	if len(strings.TrimSpace(s.GlobalConfigUrl)) > 0 {
		args = append(args, "--global-config-url", s.GlobalConfigUrl)
	}

	args = append(args, "--disable-config-reader", strconv.FormatBool(s.DisableConfigReader))

	if s.EnvironmentAttributes != nil {
		if len(s.EnvironmentAttributes) > 0 {
			args = append(args, "--env-attributes")
			var envAttributes []string
			for k, v := range s.EnvironmentAttributes {
				envAttributes = append(envAttributes, fmt.Sprintf("%v=%v", k, v))
			}
			args = append(args, strings.Join(envAttributes, ":"))
		}
	}

	// if we are running in a dev container don't call IMDS endpoint
	if os.Getenv("RUNNING_IN_DEV_CONTAINER") != "" {
		args = append(args, "--local")
	}

	extVersion, err := GetExtensionManifestVersion()
	if err == nil {
		args = append(args, "--apphealth-version", extVersion)
	}
	runningWithSystemd := false
	var cmd *exec.Cmd
	// if we have systemd available, we will use that to launch the process, otherwise we will launch directly and manipulate our own cgroups
	// check if /run/systemd/system exists, if so we have systemd and can use systemd-run
	// since systemd-run is in different paths on different distros, we will check for systemd but not use the full path
	// to systemd-run.  This is how guest agent handles it also so seems appropriate.
	if info, err := os.Stat("/run/systemd/system"); err == nil && info.IsDir() {
		systemdArgs := []string{"--scope", "-p", fmt.Sprintf("CPUQuota=%v%%", s.MaxCpuPercentage), "-p", fmt.Sprintf("MemoryMax=%v", s.MemoryLimitInBytes)}
		// now append the env variables
		for _, v := range GetVMWatchEnvironmentVariables(s.ParameterOverrides, hEnv) {
			systemdArgs = append(systemdArgs, "-E", v)
		}
		systemdArgs = append(systemdArgs, GetVMWatchBinaryFullPath(processDirectory))
		systemdArgs = append(systemdArgs, args...)

		cmd = exec.Command("systemd-run", systemdArgs...)
		runningWithSystemd = true
	} else {
		cmd = exec.Command(GetVMWatchBinaryFullPath(processDirectory), args...)
		cmd.Env = GetVMWatchEnvironmentVariables(s.ParameterOverrides, hEnv)
	}

	return cmd, runningWithSystemd, nil
}

func createAndAssignCgroups(ctx *log.Context, vmwatchSettings *vmWatchSettings, vmWatchPid int) error {
	// get our process and use this to determine the appropriate mount points for the cgroups
	myPid := os.Getpid()
	memoryLimitInBytes := int64(vmwatchSettings.MemoryLimitInBytes)

	// check cgroups mode
	if cgroups.Mode() == cgroups.Unified {
		ctx.Log("event", "cgroups v2 detected")
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
		ctx.Log("event", "cgroups v1 detected")
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

func GetProcessDirectory() (string, error) {
	p, err := filepath.Abs(os.Args[0])
	if err != nil {
		return "", err
	}

	return filepath.Dir(p), nil
}

func GetVMWatchHeartbeatFilePath(hEnv HandlerEnvironment) string {
	return filepath.Join(hEnv.HandlerEnvironment.LogFolder, "vmwatch-heartbeat.txt")
}

func GetExecutionEnvironment(hEnv HandlerEnvironment) string {
	if strings.Contains(hEnv.HandlerEnvironment.LogFolder, AppHealthPublisherNameTest) {
		return AppHealthExecutionEnvironmentTest
	}
	return AppHealthExecutionEnvironmentProd
}

func GetVMWatchConfigFullPath(processDirectory string) string {
	return filepath.Join(processDirectory, "VMWatch", VMWatchConfigFileName)
}

func GetVMWatchBinaryFullPath(processDirectory string) string {
	binaryName := VMWatchBinaryNameAmd64
	if strings.Contains(os.Args[0], AppHealthBinaryNameArm64) {
		binaryName = VMWatchBinaryNameArm64
	}

	return filepath.Join(processDirectory, "VMWatch", binaryName)
}

func GetVMWatchEnvironmentVariables(parameterOverrides map[string]interface{}, hEnv HandlerEnvironment) []string {
	var arr []string
	// make sure we get the keys out in order
	keys := make([]string, 0, len(parameterOverrides))

	for k := range parameterOverrides {
		keys = append(keys, k)
	}

	sort.Strings(keys)
	for _, k := range keys {
		arr = append(arr, fmt.Sprintf("%s=%s", k, parameterOverrides[k]))
		fmt.Println(k, parameterOverrides[k])
	}

	arr = append(arr, fmt.Sprintf("SIGNAL_FOLDER=%s", hEnv.HandlerEnvironment.EventsFolder))
	arr = append(arr, fmt.Sprintf("VERBOSE_LOG_FILE_FULL_PATH=%s", filepath.Join(hEnv.HandlerEnvironment.LogFolder, VMWatchVerboseLogFileName)))

	return arr
}
