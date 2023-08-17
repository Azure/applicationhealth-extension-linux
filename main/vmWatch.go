package main

import (
	"fmt"
	"github.com/go-kit/kit/log"
	"os"
	"os/exec"
	"strings"
	"path/filepath"
)

type VMWatchStatus string

const (
	Disabled 	VMWatchStatus = "Disabled"
	Failed 		VMWatchStatus = "Failed"
	Running  	VMWatchStatus = "Running"
	Terminated 	VMWatchStatus = "Terminated"
)

func (p VMWatchStatus) GetStatusType() StatusType {
	switch p {
	case Disabled:
		return StatusWarning
	case Failed, Terminated:
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
		return "VMWatch is disabled."
	case Failed:
		return fmt.Sprintf("VMWatch process failed to start: %s.", r.Error.Error())
	case Terminated:
		return fmt.Sprintf("VMWatch process terminated: %s.", r.Error.Error())
	default:
		return "VMWatch is running."
	}
}

func executeVMWatch(ctx *log.Context, cmd *exec.Cmd, vmWatchStatusChannel chan VMWatchResult) {
	ctx.Log("event", fmt.Sprintf("Executing VMWatch %s", cmdToString(cmd)))

	vmWatchStatusChannel <- VMWatchResult{Status:Running, Error: nil}
	
	err := cmd.Run()

	defer func() {
		vmWatchStatusChannel <- VMWatchResult{Status:Terminated, Error: err}
	}()
}

func cmdToString(cmd *exec.Cmd) string {
	return fmt.Sprintf("Command: %s\nArgs: %v\nDir: %s\nEnv: %v\n", cmd.Path, cmd.Args, cmd.Dir, cmd.Env)
}

func (s *vmWatchSettings) ToExecutableCommand() (*exec.Cmd, error) {
	processDirectory, err := GetProcessDirectory()
	if (err != nil){
		return nil, err
	}
	
	binaryFullPath, err := GetVMWatchBinaryFullPath(processDirectory)
	if err != nil {
		return nil, err
	}

	configFullPath, err := GetVMWatchConfigFullPath(processDirectory)
	if err != nil {
		return nil, err
	} 
	
	args := []string{fmt.Sprintf("--config %s", configFullPath)}

	if (s.Tests != nil && len(s.Tests) > 0) {
		args = append(args, fmt.Sprintf("--input-filter %s", strings.Join(s.Tests, ":")))
	}

	cmd := exec.Command(fmt.Sprintf("./%s", binaryFullPath), args...)

	cmd.Dir = processDirectory
	cmd.Env = GetVMWatchEnvironmentVariables(s.ParameterOverrides)

	return cmd, nil
	// fmt.Sprintf("%s ./%s --config %s --input-filter %s", environmentVariables, binaryFullPath, configFullPath, inputFilter)
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

func GetVMWatchBinaryFullPath(processDirectory string) (string, error) {
	var binaryFullPath string

	if (strings.Contains(os.Args[0], AppHealthBinaryNameArm64)) {
		binaryFullPath = filepath.Join(processDirectory, VMWatchBinaryNameArm64)
	}

	binaryFullPath = filepath.Join(processDirectory, VMWatchBinaryNameAmd64)

	if _, err := os.Stat(binaryFullPath); err != nil {
		return "", err
	}

	return binaryFullPath, nil
}

func GetVMWatchConfigFullPath(processDirectory string) (string, error) {
	configFullPath := filepath.Join(processDirectory, VMWatchConfigFileName)

	if _, err := os.Stat(configFullPath); err != nil {
		return "", err
	}

	return configFullPath, nil
}