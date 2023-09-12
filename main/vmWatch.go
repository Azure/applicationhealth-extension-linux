package main

import (
	"fmt"
	"github.com/Azure/azure-docker-extension/pkg/vmextension"
	"github.com/go-kit/kit/log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"bytes"
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
		return fmt.Sprintf("VMWatch failed: %s", r.Error.Error())
	default:
		return "VMWatch is running"
	}
}

func executeVMWatch(ctx *log.Context, s vmWatchSettings, h vmextension.HandlerEnvironment, vmWatchResultChannel chan VMWatchResult) {
	pid := -1
	combinedOutput := &bytes.Buffer{}
	var vmWatchErr error

	// Best effort to start VMWatch process 3 times
	for i := 1; i <= 3; i++ {
		// Setup command
		cmd, err := setupVMWatchCommand(s, h)
		if err != nil {
			vmWatchErr = fmt.Errorf("[%v][PID %d] Err: %w", time.Now().UTC().Format(time.RFC3339), pid, err)
			ctx.Log("error", fmt.Sprintf("Attempt %d: VMWatch setup failed: %s", i, vmWatchErr.Error()))
			continue;
		} 

		ctx.Log("event", fmt.Sprintf("Attempt %d: Setup VMWatch command: %s\nArgs: %v\nDir: %s\nEnv: %v\n", i, cmd.Path, cmd.Args, cmd.Dir, cmd.Env))

		combinedOutput.Reset()
		cmd.Stdout = combinedOutput
		cmd.Stderr = combinedOutput

		// Start command
		err = cmd.Start()
		if cmd.Process != nil {
			pid = cmd.Process.Pid
		}
		if err != nil {
			vmWatchErr = fmt.Errorf("[%v][PID %d] Err: %w\nOutput: %s", time.Now().UTC().Format(time.RFC3339), pid, err, combinedOutput.String())
			ctx.Log("error", fmt.Sprintf("Attempt %d: VMWatch failed to start: %s", i, vmWatchErr.Error()))
			continue;
		}
		ctx.Log("event", fmt.Sprintf("Attempt %d: VMWatch process started with pid %d", i, pid))

		// VMWatch should run indefinitely, if process exists we capture error
		err = cmd.Wait()
		if err != nil {
			vmWatchErr = fmt.Errorf("[%v][PID %d] Err: %w\nOutput: %s", time.Now().UTC().Format(time.RFC3339), pid, err, combinedOutput.String())
			ctx.Log("error", fmt.Sprintf("Attempt %d: VMWatch process exited: %s", i, vmWatchErr.Error()))
			continue;
		}
	}

	defer func() {
		ctx.Log("error", fmt.Sprintf("Signaling VMWatch process has failed after 3 retries"))
		vmWatchResultChannel <- VMWatchResult{Status: Failed, Error: vmWatchErr}
	}()
}

func killVMWatch(ctx *log.Context, cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		ctx.Log("event", fmt.Sprintf("VMWatch is not running, not killing process."))
		return nil
	}

	if err := cmd.Process.Kill(); err != nil {
		ctx.Log("error", fmt.Sprintf("Failed to kill VMWatch process with PID %d. Error: %v", cmd.Process.Pid, err))
		return err
	}

	ctx.Log("event", fmt.Sprintf("Successfully killed VMWatch process with PID %d", cmd.Process.Pid))
	return nil
}

func setupVMWatchCommand(s vmWatchSettings, h vmextension.HandlerEnvironment) (*exec.Cmd, error) {
	processDirectory, err := GetProcessDirectory()
	if err != nil {
		return nil, err
	}

	args := []string{"--config", GetVMWatchConfigFullPath(processDirectory)}

	if s.Tests != nil && len(s.Tests) > 0 {
		args = append(args, "--input-filter")
		args = append(args, strings.Join(s.Tests, ":"))
	}

	cmd := exec.Command(GetVMWatchBinaryFullPath(processDirectory), args...)

	cmd.Env = GetVMWatchEnvironmentVariables(s.ParameterOverrides, h)

	return cmd, nil
}

func GetVMWatchEnvironmentVariables(parameterOverrides map[string]interface{}, h vmextension.HandlerEnvironment) []string {
	var arr []string
	for key, value := range parameterOverrides {
		arr = append(arr, fmt.Sprintf("%s=%s", key, value))
	}

	arr = append(arr, fmt.Sprintf("SIGNAL_FOLDER=%s", HandlerEnvironmentEventsFolderPath))
	arr = append(arr, fmt.Sprintf("VERBOSE_LOG_FILE_FULL_PATH=%s", filepath.Join(h.HandlerEnvironment.LogFolder, VMWatchVerboseLogFileName)))

	return arr
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
