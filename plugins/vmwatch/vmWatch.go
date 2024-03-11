package vmwatch

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Azure/applicationhealth-extension-linux/internal/handlerenv"
	global "github.com/Azure/applicationhealth-extension-linux/internal/utils"
	"github.com/Azure/applicationhealth-extension-linux/internal/version"
	"github.com/Azure/applicationhealth-extension-linux/pkg/logging"
	"github.com/Azure/applicationhealth-extension-linux/pkg/status"
	"github.com/Azure/applicationhealth-extension-linux/pkg/utils"
	"github.com/containerd/cgroups/v3"
	"github.com/containerd/cgroups/v3/cgroup1"
	"github.com/containerd/cgroups/v3/cgroup2"
	"github.com/opencontainers/runtime-spec/specs-go"
)

type VMWatchStatus string

const (
	MaxCpuQuota               = 1        // 1% cpu
	MaxMemoryInBytes          = 40000000 // 40MB
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

func (p VMWatchStatus) GetStatusType() status.StatusType {
	switch p {
	case Disabled:
		return status.StatusWarning
	case Failed:
		return status.StatusError
	default:
		return status.StatusSuccess
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
func ExecuteVMWatch(lg logging.Logger, s *VMWatchSettings, hEnv *handlerenv.HandlerEnvironment, vmWatchResultChannel chan VMWatchResult) {
	var vmWatchErr error
	defer func() {
		if r := recover(); r != nil {
			vmWatchErr = fmt.Errorf("%w\n Additonal Details: %+v", vmWatchErr, r)
			lg.Error(fmt.Sprintf("VMWatch failed: %+v", r), slog.Any("error", vmWatchErr))
		}
		vmWatchResultChannel <- VMWatchResult{Status: Failed, Error: vmWatchErr}
		close(vmWatchResultChannel)
	}()

	// Best effort to start VMWatch process each time it fails start immediately up to VMWatchMaxProcessAttempts before waiting for
	// a longer time before trying again
	for !global.Shutdown {
		for i := 1; i <= VMWatchMaxProcessAttempts && !global.Shutdown; i++ {
			vmWatchResultChannel <- VMWatchResult{Status: Running}
			vmWatchErr = executeVMWatchHelper(lg, i, s, hEnv)
			vmWatchResultChannel <- VMWatchResult{Status: Failed, Error: vmWatchErr}
		}
		err := fmt.Errorf("VMWatch reached max %d retries, sleeping for %v hours before trying again", VMWatchMaxProcessAttempts, HoursBetweenRetryAttempts)
		lg.Error("VMWatch reached max retries", slog.Any("error", err))
		// we have exceeded the retries so now we go to sleep before starting again
		time.Sleep(time.Hour * HoursBetweenRetryAttempts)
	}
}

func executeVMWatchHelper(lg logging.Logger, attempt int, vmWatchSettings *VMWatchSettings, hEnv *handlerenv.HandlerEnvironment) (err error) {
	pid := -1
	var cmd *exec.Cmd
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("error: %w\n Additonal Details: %+v", err, r)
			lg.Error(fmt.Sprintf("VMWatch failed: Recovered %+v", r), slog.Any("error", err))
		}
	}()

	// Setup command
	cmd, err = setupVMWatchCommand(vmWatchSettings, hEnv)
	if err != nil {
		err = fmt.Errorf("[%v][PID -1] Attempt %d: VMWatch setup failed. Error: %w", time.Now().UTC().Format(time.RFC3339), attempt, err)
		lg.Error("VMWatch setup failed", slog.Any("error", err))
		return err
	}

	lg.Info(fmt.Sprintf("Attempt %d: VMWatch process started with pid %d", attempt, pid))
	// TODO: Combined output may get excessively long, especially since VMWatch is a long running process
	// We should trim the output or only get from Stderr
	combinedOutput := &bytes.Buffer{}
	cmd.Stdout = combinedOutput

	cmd.Stderr = combinedOutput

	// Start command
	if err := cmd.Start(); err != nil {
		err = fmt.Errorf("[%v][PID -1] Attempt %d: VMWatch failed to start. Error: %w\nOutput: %s", time.Now().UTC().Format(time.RFC3339), attempt, err, combinedOutput.String())
		lg.Error("VMWatch failed to start", slog.Any("error", err))
		return err
	}
	pid = cmd.Process.Pid // cmd.Process should be populated on success
	lg.Info(fmt.Sprintf("Attempt %d: VMWatch process started with pid %d", attempt, pid))
	err = createAndAssignCgroups(lg, pid)
	if err != nil {
		err = fmt.Errorf("[%v][PID %d] Failed to assign VMWatch process to cgroup. Error: %w", time.Now().UTC().Format(time.RFC3339), pid, err)
		lg.Error("Failed to assign VMWatch process to cgroup", slog.Any("error", err))
		// On real VMs we want this to stop vwmwatch from runing at all since we want to make sure we are protected
		// by resource governance but on dev machines, we may fail due to limitations of execution environment (ie on dev container
		// or in a github pipeline container we don't have permission to assign cgroups (also on WSL environments it doesn't
		// work at all because the base OS doesn't support it).
		// to allow us to run integration tests we will check the variables RUNING_IN_DEV_CONTAINER and
		// ALLOW_VMWATCH_GROUP_ASSIGNMENT_FAILURE and if they are both set we will just log and continue
		// this allows us to test both cases
		if os.Getenv(AllowVMWatchCgroupAssignmentFailureVariableName) == "" || os.Getenv(RunningInDevContainerVariableName) == "" {
			lg.Info("Killing VMWatch process as cgroup assigment failed")
			_ = KillVMWatch(lg, cmd)
			return err
		}
	}

	processDone := make(chan bool)

	// create a waitgroup to coordinate the goroutines
	var wg sync.WaitGroup
	// add a task to wait for process completion
	wg.Add(1)
	go func() {
		defer wg.Done()
		err = cmd.Wait()
		processDone <- true
		close(processDone)
	}()
	// add a task to monitor heartbeat
	wg.Add(1)
	go func() {
		defer wg.Done()
		monitorHeartBeat(lg, GetVMWatchHeartbeatFilePath(hEnv), processDone, cmd)
	}()
	wg.Wait()
	err = fmt.Errorf("[%v][PID %d] Attempt %d: VMWatch process exited. Error: %w\nOutput: %s", time.Now().UTC().Format(time.RFC3339), pid, attempt, err, combinedOutput.String())
	lg.Error("VMWatch process exited", slog.Any("error", err))
	return err
}

func monitorHeartBeat(lg logging.Logger, heartBeatFile string, processDone chan bool, cmd *exec.Cmd) {
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
				lg.Error("VMWatch process did not update heartbeat file within the time limit", slog.Any("error", err))
				err = KillVMWatch(lg, cmd)
				if err != nil {
					err = fmt.Errorf("[%v][PID %d] Failed to kill vmwatch process", time.Now().UTC().Format(time.RFC3339), cmd.Process.Pid)
					lg.Error("Failed to Kill VMWatch", slog.Any("error", err))
				}
			}
		case <-processDone:
			return
		}
	}
}

func KillVMWatch(lg logging.Logger, cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil || cmd.ProcessState != nil {
		lg.Info("VMWatch is not running, killing process is not necessary.")
		return nil
	}

	if err := cmd.Process.Kill(); err != nil {
		lg.Error(fmt.Sprintf("Failed to kill VMWatch process with PID %d. Error: %v", cmd.Process.Pid, err))
		return err
	}

	lg.Info(fmt.Sprintf("Successfully killed VMWatch process with PID %d", cmd.Process.Pid))
	return nil
}

func setupVMWatchCommand(s *VMWatchSettings, hEnv *handlerenv.HandlerEnvironment) (*exec.Cmd, error) {
	processDirectory, err := utils.GetProcessDirectory()
	if err != nil {
		return nil, err
	}

	args := []string{"--config", GetVMWatchConfigFullPath(processDirectory)}
	args = append(args, "--debug")
	args = append(args, "--heartbeat-file", GetVMWatchHeartbeatFilePath(hEnv))
	args = append(args, "--execution-environment", GetExecutionEnvironment(hEnv))

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

	extVersion, err := version.GetExtensionVersion()
	if err == nil {
		args = append(args, "--apphealth-version", extVersion)
	}

	cmd := exec.Command(GetVMWatchBinaryFullPath(processDirectory), args...)

	cmd.Env = GetVMWatchEnvironmentVariables(s.ParameterOverrides, hEnv)

	return cmd, nil
}

func createAndAssignCgroups(lg logging.Logger, vmWatchPid int) error {
	// get our process and use this to determine the appropriate mount points for the cgroups
	myPid := os.Getpid()
	memoryLimitInBytes := int64(MaxMemoryInBytes)

	// check cgroups mode
	if cgroups.Mode() == cgroups.Unified {
		lg.Info("cgroups v2 detected")
		// in cgroup v2, we need to set the period and quota relative to one another.
		// Quota is the number of microseconds in the period that process can run
		// Period is the length of the period in microseconds
		period := uint64(CGroupV2PeriodMs)
		cpuQuota := int64(MaxCpuQuota * 10000)
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
		lg.Info("cgroups v1 detected")
		p := cgroup1.PidPath(myPid)

		cpuPath, err := p("cpu")
		if err != nil {
			return err
		}

		// in cgroup v1, the interval is implied, 1000 == 1 %
		cpuQuota := int64(MaxCpuQuota * 1000)
		memoryLimitInBytes := int64(MaxMemoryInBytes)

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

func GetVMWatchHeartbeatFilePath(hEnv *handlerenv.HandlerEnvironment) string {
	return filepath.Join(hEnv.LogFolder, "vmwatch-heartbeat.txt")
}

func GetExecutionEnvironment(hEnv *handlerenv.HandlerEnvironment) string {
	if strings.Contains(hEnv.LogFolder, AppHealthPublisherNameTest) {
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

func GetVMWatchEnvironmentVariables(parameterOverrides map[string]interface{}, hEnv *handlerenv.HandlerEnvironment) []string {
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

	arr = append(arr, fmt.Sprintf("SIGNAL_FOLDER=%s", hEnv.EventsFolder))
	arr = append(arr, fmt.Sprintf("VERBOSE_LOG_FILE_FULL_PATH=%s", filepath.Join(hEnv.LogFolder, VMWatchVerboseLogFileName)))

	return arr
}
