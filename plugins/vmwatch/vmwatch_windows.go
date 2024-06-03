package vmwatch

import (
	"bytes"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/applicationhealth-extension-linux/internal/handlerenv"
	"github.com/Azure/applicationhealth-extension-linux/pkg/logging"
)

func configureVMWatchProcess(lg logging.Logger, attempt int, vmWatchSettings *VMWatchSettings, hEnv *handlerenv.HandlerEnvironment) (*exec.Cmd, bool, *bytes.Buffer, error) {
	// Setup command
	cmd, resourceGovernanceRequired, err := setupVMWatchCommand(vmWatchSettings, hEnv)
	if err != nil {
		err = fmt.Errorf("[%v][PID -1] Attempt %d: VMWatch setup failed. Error: %w", time.Now().UTC().Format(time.RFC3339), attempt, err)
		lg.Error("VMWatch setup failed", slog.Any("error", err))
		// sendTelemetry(lg, telemetry.EventLevelError, telemetry.SetupVMWatchTask, err.Error())
		return nil, false, nil, err
	}
	lg.Info(fmt.Sprintf("Attempt %d: Setup VMWatch command: %s\nArgs: %v\nDir: %s\nEnv: %v\n", attempt, cmd.Path, cmd.Args, cmd.Dir, cmd.Env))
	// 	fmt.Sprintf("Attempt %d: Setup VMWatch command: %s\nArgs: %v\nDir: %s\nEnv: %v\n",
	// 		attempt, vmWatchCommand.Path, vmWatchCommand.Args, vmWatchCommand.Dir, vmWatchCommand.Env),
	// TODO: Combined output may get excessively long, especially since VMWatch is a long running process
	// We should trim the output or only get from Stderr
	combinedOutput := &bytes.Buffer{}
	cmd.Stdout = combinedOutput
	cmd.Stderr = combinedOutput
	return cmd, resourceGovernanceRequired, combinedOutput, nil
}

// createCommandForOS creates a new command for the operating system with specific settings and environment variables.
// It takes in a VMWatchSettings object, a HandlerEnvironment object, the path to the command, and a slice of arguments.
// It returns the created command and a boolean flag indicating whether further resource governance is required.
func createCommandForOS(s *VMWatchSettings, hEnv *handlerenv.HandlerEnvironment, cmdPath string, args []string) (*exec.Cmd, bool) {
	var (
		cmd *exec.Cmd
		// flag to tell the caller that further resource governance is required.
		// Default to false for Windows for now.
		// TODO: Implement resource governance for Windows
		resourceGovernanceRequired bool = false
	)

	cmd = exec.Command(GetVMWatchBinaryFullPath(cmdPath), args...)
	cmd.Env = GetVMWatchEnvironmentVariables(s.ParameterOverrides, hEnv)
	return cmd, resourceGovernanceRequired
}

// TODO: Implement resource governance for Windows
func applyResourceGovernance(lg logging.Logger, vmWatchSettings *VMWatchSettings, vmWatchCommand *exec.Cmd) error {
	return nil
}

func generateEnvVarsForVMWatch(hEnv *handlerenv.HandlerEnvironment) []string {
	var (
		arr []string = make([]string, 0, 2)
	)
	arr = append(arr, fmt.Sprintf("SIGNAL_FOLDER=%s", strings.ReplaceAll(hEnv.EventsFolder, `\`, `\\`)))
	arr = append(arr, fmt.Sprintf("VERBOSE_LOG_FILE_FULL_PATH=%s", strings.ReplaceAll(filepath.Join(hEnv.LogFolder, VMWatchVerboseLogFileName), `\`, `\\`)))
	return arr
}
