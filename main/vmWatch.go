package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
)

type VMWatchStatus string

const (
	Disabled VMWatchStatus = "Disabled"
	Running  VMWatchStatus = "Running"
	Failed   VMWatchStatus = "Failed"
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
		ctx.Log("error", fmt.Sprintf("Signaling VMWatchStatus is Failed due to reaching max of %d retries", VMWatchMaxProcessAttempts))
		vmWatchResultChannel <- VMWatchResult{Status: Failed, Error: vmWatchErr}
		close(vmWatchResultChannel)
	}()

	// Best effort to start VMWatch process each time it fails
	for i := 1; i <= VMWatchMaxProcessAttempts; i++ {
		vmWatchErr = executeVMWatchHelper(ctx, i, s, hEnv)
	}
}

func executeVMWatchHelper(ctx *log.Context, attempt int, vmWatchSettings *vmWatchSettings, hEnv HandlerEnvironment) (err error) {
	pid := -1
	var cmd *exec.Cmd
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("Error: %w\n Additonal Details: %+v", err, r)
			ctx.Log("error", "Recovered %+v", r)
		}
		killVMWatch(ctx, cmd)
	}()

	// Setup command
	cmd, err = setupVMWatchCommand(vmWatchSettings, hEnv)
	if err != nil {
		err = fmt.Errorf("[%v][PID -1] Attempt %d: VMWatch setup failed. Error: %w", time.Now().UTC().Format(time.RFC3339), attempt, err)
		ctx.Log("error", err.Error())
		return err
	}

	ctx.Log("event", fmt.Sprintf("Attempt %d: Setup VMWatch command: %s\nArgs: %v\nDir: %s\nEnv: %v\n", attempt, cmd.Path, cmd.Args, cmd.Dir, cmd.Env))

	// TODO: Combined output may get excessively long, especially since VMWatch is a long running process
	// We should trim the output or only get from Stderr
	combinedOutput := &bytes.Buffer{}
	cmd.Stdout = combinedOutput
	cmd.Stderr = combinedOutput

	// Start command
	if err := cmd.Start(); err != nil {
		err = fmt.Errorf("[%v][PID -1] Attempt %d: VMWatch failed to start. Error: %w\nOutput: %s", time.Now().UTC().Format(time.RFC3339), attempt, err, combinedOutput.String())
		ctx.Log("error", err.Error())
		return err
	}
	pid = cmd.Process.Pid // cmd.Process should be populated on success
	ctx.Log("event", fmt.Sprintf("Attempt %d: VMWatch process started with pid %d", attempt, pid))

	// VMWatch should run indefinitely, if process exists we expect an error
	err = cmd.Wait()
	err = fmt.Errorf("[%v][PID %d] Attempt %d: VMWatch process exited. Error: %w\nOutput: %s", time.Now().UTC().Format(time.RFC3339), pid, attempt, err, combinedOutput.String())
	ctx.Log("error", err.Error())
	return err
}

func killVMWatch(ctx *log.Context, cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil || cmd.ProcessState != nil {
		ctx.Log("event", fmt.Sprintf("VMWatch is not running, killing process is not necessary."))
		return nil
	}

	if err := cmd.Process.Kill(); err != nil {
		ctx.Log("error", fmt.Sprintf("Failed to kill VMWatch process with PID %d. Error: %v", cmd.Process.Pid, err))
		return err
	}

	ctx.Log("event", fmt.Sprintf("Successfully killed VMWatch process with PID %d", cmd.Process.Pid))
	return nil
}

func setupVMWatchCommand(s *vmWatchSettings, hEnv HandlerEnvironment) (*exec.Cmd, error) {
	processDirectory, err := GetProcessDirectory()
	if err != nil {
		return nil, err
	}

	args := []string{"--config", GetVMWatchConfigFullPath(processDirectory)}

	args = append(args, "--input-filter")
	if s.Tests != nil && len(s.Tests) > 0 {
		args = append(args, strings.Join(s.Tests, ":"))
	} else {
		args = append(args, VMWatchDefaultTests)
	}

	// if we are running in a dev container don't call IMDS endpoint
	if os.Getenv("RUNNING_IN_DEV_CONTAINER") != "" {
		args = append(args, "--local")
	}

	cmd := exec.Command(GetVMWatchBinaryFullPath(processDirectory), args...)

	cmd.Env = GetVMWatchEnvironmentVariables(s.ParameterOverrides, hEnv)

	return cmd, nil
}

func GetProcessDirectory() (string, error) {
	p, err := filepath.Abs(os.Args[0])
	if err != nil {
		return "", err
	}

	return filepath.Dir(p), nil
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
	for key, value := range parameterOverrides {
		arr = append(arr, fmt.Sprintf("%s=%s", key, value))
	}

	arr = append(arr, fmt.Sprintf("SIGNAL_FOLDER=%s", hEnv.HandlerEnvironment.EventsFolder))
	arr = append(arr, fmt.Sprintf("VERBOSE_LOG_FILE_FULL_PATH=%s", filepath.Join(hEnv.HandlerEnvironment.LogFolder, VMWatchVerboseLogFileName)))

	return arr
}
