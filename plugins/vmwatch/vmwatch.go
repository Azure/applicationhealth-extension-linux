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
	"github.com/Azure/azure-extension-platform/pkg/utils"
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

var (
	VMWatchCommand *exec.Cmd // We need a reference to the command here so that we can cleanly shutdown VMWatch process
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
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("error: %w\n Additonal Details: %+v", err, r)
			lg.Error(fmt.Sprintf("VMWatch failed: Recovered %+v", r), slog.Any("error", err))
		}
	}()

	// Setup command
	var combinedOutput *bytes.Buffer = nil
	VMWatchCommand, combinedOutput, err = setupVMWatch(lg, attempt, vmWatchSettings, hEnv)
	if err != nil {
		return err
	}

	// Start command
	if err := VMWatchCommand.Start(); err != nil {
		err = fmt.Errorf("[%v][PID -1] Attempt %d: VMWatch failed to start. Error: %w\nOutput: %s", time.Now().UTC().Format(time.RFC3339), attempt, err, combinedOutput.String())
		lg.Error("VMWatch failed to start", slog.Any("error", err))
		return err
	}
	pid = VMWatchCommand.Process.Pid // cmd.Process should be populated on success
	lg.Info(fmt.Sprintf("Attempt %d: VMWatch process started with pid %d", attempt, pid))

	err = createAndAssignCgroups(lg, vmWatchSettings, pid)
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
			_ = KillVMWatch(lg, VMWatchCommand)
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
		err = VMWatchCommand.Wait()
		processDone <- true
		close(processDone)
	}()
	// add a task to monitor heartbeat
	wg.Add(1)
	go func() {
		defer wg.Done()
		monitorHeartBeat(lg, GetVMWatchHeartbeatFilePath(hEnv), processDone, VMWatchCommand)
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
				lg.Error(fmt.Sprintf("[%v][PID %d] VMWatch process did not update heartbeat file within the time limit, killing the process", time.Now().UTC().Format(time.RFC3339), cmd.Process.Pid), slog.Any("error", err))
				err = KillVMWatch(lg, VMWatchCommand)
				if err != nil {
					err = fmt.Errorf("[%v][PID %d] Failed to kill vmwatch process", time.Now().UTC().Format(time.RFC3339), cmd.Process.Pid)
					lg.Error(fmt.Sprintf("[%v][PID %d] Failed to kill vmwatch process", time.Now().UTC().Format(time.RFC3339), cmd.Process.Pid), slog.Any("error", err))
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
		s := fmt.Sprintf("Failed to kill VMWatch process with PID %d. Error: %v", cmd.Process.Pid, err)
		lg.Error(s, slog.Any("error", err))
		return err
	}

	lg.Info(fmt.Sprintf("Successfully killed VMWatch process with PID %d", cmd.Process.Pid))
	return nil
}

func setupVMWatchCommand(s *VMWatchSettings, hEnv *handlerenv.HandlerEnvironment) (*exec.Cmd, error) {
	processDirectory, err := utils.GetCurrentProcessWorkingDir()
	if err != nil {
		return nil, err
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
		return nil, err
	}

	// check cpu, if 0 (default) set to the default value
	if s.MaxCpuPercentage == 0 {
		s.MaxCpuPercentage = DefaultMaxCpuPercentage
	}

	if s.MaxCpuPercentage < 0 || s.MaxCpuPercentage > 100 {
		err = fmt.Errorf("[%v] Invalid maxCpuPercentage specified must be between 0 and 100", time.Now().UTC().Format(time.RFC3339))
		return nil, err
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

	extVersion, err := version.GetExtensionVersion()
	if err == nil {
		args = append(args, "--apphealth-version", extVersion)
	}

	cmd := exec.Command(GetVMWatchBinaryFullPath(processDirectory), args...)

	cmd.Env = GetVMWatchEnvironmentVariables(s.ParameterOverrides, hEnv)

	return cmd, nil
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

	addVMWatchEnviromentVariables(&arr, hEnv)

	return arr
}
