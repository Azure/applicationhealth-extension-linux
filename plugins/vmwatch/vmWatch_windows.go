package vmwatch

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/Azure/applicationhealth-extension-linux/internal/handlerenv"
	"github.com/Azure/applicationhealth-extension-linux/pkg/logging"
)

func createAndAssignCgroups(lg logging.Logger, vmwatchSettings *VMWatchSettings, vmWatchPid int) error {
	return nil
}

