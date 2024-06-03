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

func setupVMWatch(lg logging.Logger, attempt int, vmWatchSettings *VMWatchSettings, hEnv *handlerenv.HandlerEnvironment) (*exec.Cmd, *bytes.Buffer, error) {
	// Setup command
	cmd, err := setupVMWatchCommand(vmWatchSettings, hEnv)
	if err != nil {
		err = fmt.Errorf("[%v][PID -1] Attempt %d: VMWatch setup failed. Error: %w", time.Now().UTC().Format(time.RFC3339), attempt, err)
		lg.Error("VMWatch setup failed", slog.Any("error", err))
		return nil, nil, err
	}
	lg.Info(fmt.Sprintf("Attempt %d: Setup VMWatch command: %s\nArgs: %v\nDir: %s\nEnv: %v\n", attempt, cmd.Path, cmd.Args, cmd.Dir, cmd.Env))
	// TODO: Combined output may get excessively long, especially since VMWatch is a long running process
	// We should trim the output or only get from Stderr
	combinedOutput := &bytes.Buffer{}
	cmd.Stdout = combinedOutput
	cmd.Stderr = combinedOutput
	return cmd, combinedOutput, nil
}

func createAndAssignCgroups(lg logging.Logger, vmwatchSettings *VMWatchSettings, vmWatchPid int) error {
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
