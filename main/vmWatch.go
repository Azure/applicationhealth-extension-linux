package main

import (
	"fmt"
	"github.com/go-kit/kit/log"
	"os"
	"os/exec"
	"strings"
	"path/filepath"
	"time"
)

type VMWatchStatus string

const (
	Disabled 	VMWatchStatus = "Disabled"
	Running  	VMWatchStatus = "Running"
	Failed 		VMWatchStatus = "Failed"
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
	Status 	VMWatchStatus
	Error	error
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

func executeVMWatch(ctx *log.Context, cmd *exec.Cmd, vmWatchResultChannel chan VMWatchResult) {
	ctx.Log("event", fmt.Sprintf("Execute VMWatch %s", cmdToString(cmd)))

	output, err := cmd.CombinedOutput()
	
	pid := -1
	if cmd.Process != nil {
		pid = cmd.Process.Pid
	}

	defer func() {
		err = fmt.Errorf("[%v][PID %d] Err: %w\nOutput: %s", time.Now().UTC(), pid, err, string(output))
		vmWatchResultChannel <- VMWatchResult{Status:Failed, Error: err}
	}()
}

func killVMWatch(ctx *log.Context, cmd *exec.Cmd) error {
	if cmd == nil {
		ctx.Log("event", fmt.Sprintf("VMWatch is not running, not killing process."))
		return nil
	}

	if err := cmd.Process.Kill(); err != nil {
		ctx.Log("error", fmt.Sprintf("Failed to kill VMWatch process with PID %d. Error: %w", cmd.Process.Pid, err))
		return err
	}
	
	ctx.Log("event", fmt.Sprintf("Successfully killed VMWatch process with PID %d", cmd.Process.Pid))
	return nil
}

func cmdToString(cmd *exec.Cmd) string {
	return fmt.Sprintf("Command: %s\nArgs: %v\nDir: %s\nEnv: %v\n", cmd.Path, cmd.Args, cmd.Dir, cmd.Env)
}

func (s *vmWatchSettings) ToExecutableCommand() (*exec.Cmd, error) {
	processDirectory, err := GetProcessDirectory()
	if (err != nil){
		return nil, err
	}
	
	args := []string{"--config", GetVMWatchConfigFullPath(processDirectory)}

	if (s.Tests != nil && len(s.Tests) > 0) {
		args = append(args, "--input-filter")
		args = append(args, strings.Join(s.Tests, ":"))
	}

	cmd := exec.Command(GetVMWatchBinaryFullPath(processDirectory), args...)

	cmd.Env = GetVMWatchEnvironmentVariables(s.ParameterOverrides)

	return cmd, nil
}

func GetVMWatchEnvironmentVariables(parameterOverrides map[string]interface{}) []string {
	var arr []string
	for key, value := range parameterOverrides {
		arr = append(arr, fmt.Sprintf("%s=%s", key, value))
	}

	arr = append(arr, "SIGNAL_FOLDER=/var/log/azure/Microsoft.ManagedServices.ApplicationHealthLinux/events")
	arr = append(arr, "VERBOSE_LOG_FILE_FULL_PATH=/var/log/azure/Microsoft.ManagedServices.ApplicationHealthLinux/vmwatch.log")

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
	return filepath.Join(processDirectory, VMWatchConfigFileName)
}

func GetVMWatchBinaryFullPath(processDirectory string) string {
	binaryName := VMWatchBinaryNameAmd64
	if (strings.Contains(os.Args[0], AppHealthBinaryNameArm64)) {
		binaryName = VMWatchBinaryNameArm64
	}

	return filepath.Join(processDirectory, binaryName)
}