package vmwatch

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Azure/applicationhealth-extension-linux/internal/handlerenv"
	"github.com/Azure/applicationhealth-extension-linux/pkg/logging"
)

func createAndAssignCgroups(lg logging.Logger, vmwatchSettings *VMWatchSettings, vmWatchPid int) error {
	return nil
}

func addVMWatchEnviromentVariables(arr *[]string, hEnv *handlerenv.HandlerEnvironment) {
	*arr = append(*arr, fmt.Sprintf("SIGNAL_FOLDER=%s", strings.ReplaceAll(hEnv.EventsFolder, `\`, `\\`)))
	*arr = append(*arr, fmt.Sprintf("VERBOSE_LOG_FILE_FULL_PATH=%s", strings.ReplaceAll(filepath.Join(hEnv.LogFolder, VMWatchVerboseLogFileName), `\`, `\\`)))
}
