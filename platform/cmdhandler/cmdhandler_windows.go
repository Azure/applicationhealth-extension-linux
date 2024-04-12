package cmdhandler

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows/registry"

	"github.com/Azure/applicationhealth-extension-linux/internal/handlerenv"
	global "github.com/Azure/applicationhealth-extension-linux/internal/utils"
	"github.com/Azure/applicationhealth-extension-linux/pkg/logging"
	"github.com/Azure/applicationhealth-extension-linux/pkg/status"
	"github.com/Azure/applicationhealth-extension-linux/plugins/vmwatch"
	"github.com/pkg/errors"
)

const (
	regKeyPath              string = `SOFTWARE\Microsoft\Windows Azure\AppHealthExtension`
	enabledRegKeyValueName  string = "IsEnabled"
	updatingRegKeyValueName string = "IsUpdating"

	// Windows Specific Commands
	ResetStateKey  CommandKey  = "resetState"
	ResetStateName CommandName = "ResetState"
)

var (
	ErrFailedToCreateRegKey       error = errors.New("Failed to create Windows Registry SubKey")
	ErrFailedToOpenRegKey         error = errors.New("Failed to create or open Windows Registry SubKey")
	ErrFailedToGetValueFromRegKey error = errors.New("Failed to get value from Windows Registry SubsKey")
)

var extCommands = CommandMap{
	InstallKey:    cmd{f: noop, Name: InstallName, ShouldReportStatus: false, pre: installHandler, failExitCode: 52},    // TODO: Implement
	UninstallKey:  cmd{f: noop, Name: UninstallName, ShouldReportStatus: false, pre: uninstallHandler, failExitCode: 3}, // TODO: Implement
	EnableKey:     cmd{f: enable, Name: EnableName, ShouldReportStatus: true, pre: enableHandler, failExitCode: 3},
	UpdateKey:     cmd{f: noop, Name: UpdateName, ShouldReportStatus: true, pre: updateHandler, failExitCode: 3},
	DisableKey:    cmd{f: noop, Name: DisableName, ShouldReportStatus: true, pre: disableHandler, failExitCode: 3},
	ResetStateKey: cmd{f: noop, Name: ResetStateName, ShouldReportStatus: true, pre: resetStateHandler, failExitCode: 3},
}

type WindowsCommandHandler struct {
	commands CommandMap
}

func newOSCommandHandler() (CommandHandler, error) {
	return &WindowsCommandHandler{
		commands: extCommands,
	}, nil
}

func (ch *WindowsCommandHandler) Execute(c CommandKey, h *handlerenv.HandlerEnvironment, seqNum int, lg logging.Logger) error {
	// TODO: Implement command execution
	lg.Info(fmt.Sprintf("WindowsCommandHandler was Created, with command: %s and sequenceNumber: %d", c, seqNum))

	// Getting command to execute
	cmd, ok := ch.commands[c]
	if !ok {
		return errors.Errorf("unknown command: %s", c)
	}

	lg.With("operation", strings.ToLower(cmd.Name.String()))
	lg.With("seq", strconv.Itoa(seqNum))

	lg.Info("Starting AppHealth Extension")
	lg.Info(fmt.Sprintf("HandlerEnvironment: %v", h))
	// subscribe to cleanly shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		lg.Info("Received shutdown request")
		global.Shutdown = true
		err := vmwatch.KillVMWatch(lg, vmwatch.VMWatchCommand)
		if err != nil {
			lg.Error("error when killing vmwatch", slog.Any("error", err))
		}
	}()

	ticker := time.NewTicker(10 * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				isEnabled, err := isExtensionEnabled()
				if err != nil {
					lg.Error("error when checking if extension is enabled", slog.Any("error", err))
				}
				if !isEnabled {
					lg.Info("Windows Registry Key was set to disabled, shutting down extension")
					lg.Info("Sending shutdown signal")
					sigs <- syscall.SIGTERM
					ticker.Stop()
				}
			}
		}
	}()

	if cmd.pre != nil {
		lg.Info("pre-check")
		if err := cmd.pre(lg, seqNum); err != nil {
			lg.Error("pre-check failed", slog.Any("error", err))
			os.Exit(cmd.failExitCode)
		}
	}

	// execute the subcommand
	ReportStatus(lg, h, seqNum, status.StatusTransitioning, cmd, "")
	msg, err := cmd.f(lg, h, seqNum)
	if err != nil {
		lg.Error("failed to handle", slog.Any("error", err))
		ReportStatus(lg, h, seqNum, status.StatusError, cmd, err.Error()+msg)
		os.Exit(cmd.failExitCode)
	}
	ReportStatus(lg, h, seqNum, status.StatusSuccess, cmd, msg)
	lg.Info("end")

	return nil
}

func (ch *WindowsCommandHandler) CommandMap() CommandMap {
	return ch.commands
}

func installHandler(lg logging.Logger, seqNum int) error {
	lg.Info("Installing Handler")
	lg.Info(`Creating Windows Registry Key "HKLM\%s"`, regKeyPath)
	// Create a new registry key with all access permissions.
	k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, regKeyPath, registry.ALL_ACCESS)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrFailedToCreateRegKey, err) // Wrap the original error with your predefined error
	}
	defer k.Close()
	lg.Info("Handler successfully installed")
	return nil
}

func enableHandler(lg logging.Logger, seqNum int) error {
	lg.Info("Enabling Handler")
	// Create or open registry key with all access permissions.
	k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, regKeyPath, registry.ALL_ACCESS)

	if err != nil {
		return fmt.Errorf("%w: %v", ErrFailedToCreateRegKey, err)
	}
	defer k.Close()

	lg.Info(fmt.Sprintf(`Updating value of Windows Registry SubKey "HKLM\%s\%s"`, regKeyPath, enabledRegKeyValueName))
	// Get the current value of the registry key.
	isEnabled, _, err := k.GetStringValue(enabledRegKeyValueName)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrFailedToGetValueFromRegKey, err)
	}
	lg.Info(fmt.Sprintf(`Windows Registry SubKey "HKLM\%s\%s" has value: "%s"`, regKeyPath, enabledRegKeyValueName, isEnabled))

	// Set the value of the registry key.
	err = k.SetStringValue(enabledRegKeyValueName, "True")
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf(`Failed to set registry key value "HKLM\%s\%s" to "True"`, regKeyPath, enabledRegKeyValueName))
	}

	lg.Info(fmt.Sprintf(`Successfully set the registry key value "HKLM\%s\%s" to "True"`, regKeyPath, enabledRegKeyValueName))
	return nil
}

func disableHandler(lg logging.Logger, seqNum int) error {
	lg.Info("Disabling Handler")

	k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, regKeyPath, registry.ALL_ACCESS)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrFailedToCreateRegKey, err)
	}
	defer k.Close()

	lg.Info(fmt.Sprintf(`Updating value of Windows Registry SubKey "HKLM\%s\%s"`, regKeyPath, enabledRegKeyValueName))
	// Get the current value of the registry key.
	isEnabled, _, err := k.GetStringValue(enabledRegKeyValueName)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrFailedToGetValueFromRegKey, err)
	}
	lg.Info(fmt.Sprintf(`Windows Registry SubKey "HKLM\%s\%s" has value: "%s"`, regKeyPath, enabledRegKeyValueName, isEnabled))

	err = k.SetStringValue(enabledRegKeyValueName, "False")
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf(`Failed to set registry subkey value "HKLM\%s\%s" to "False"`, regKeyPath, enabledRegKeyValueName))
	}
	lg.Info(fmt.Sprintf(`Successfully set the registry key value "HKLM\%s\%s" to "False"`, regKeyPath, enabledRegKeyValueName))
	return nil
}

func updateHandler(lg logging.Logger, seqNum int) error {
	lg.Info("Updating Handler")
	k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, regKeyPath, registry.ALL_ACCESS)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrFailedToCreateRegKey, err)
	}
	defer k.Close()

	lg.Info(fmt.Sprintf(`Setting the value of Windows Registry SubKey "HKLM\%s\%s"`, regKeyPath, updatingRegKeyValueName))
	err = k.SetStringValue(updatingRegKeyValueName, "True")
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf(`Failed to set registry subkey value "HKLM\%s\%s" to "True"`, regKeyPath, updatingRegKeyValueName))
	}

	lg.Info(fmt.Sprintf(`Successfully set the registry subkey value "HKLM\%s\%s" to "True"`, regKeyPath, updatingRegKeyValueName))
	return nil
}

func uninstallHandler(lg logging.Logger, seqNum int) error {
	lg.Info("Uninstalling Handler")

	k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, regKeyPath, registry.ALL_ACCESS)
	if err != nil {
		return fmt.Errorf(`Unable to open registry subkey "HKLM\%s".`, regKeyPath)
	}
	defer k.Close()

	isUpdating, _, err := k.GetStringValue(updatingRegKeyValueName)
	if err != nil {
		lg.Info(fmt.Sprintf(`Registry Value "%s" was not found under the %s key beneath the LOCAL_MACHINE root. Deleting subkey tree anyways.`, updatingRegKeyValueName, regKeyPath))
	}

	if isUpdating == "True" {
		lg.Info(fmt.Sprintf(`Resetting registry value "HKLM\%s\%s" to "False"`, regKeyPath, updatingRegKeyValueName))
		err = k.SetStringValue(updatingRegKeyValueName, "False")
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf(`Failed to set registry subkey value "HKLM\%s\%s" to "False"`, regKeyPath, updatingRegKeyValueName))
		}
	}

	lg.Info("Deleting Subkey Tree beneath the LOCAL_MACHINE root with the path %s", regKeyPath)
	err = registry.DeleteKey(registry.LOCAL_MACHINE, regKeyPath)
	if err != nil {
		return fmt.Errorf(`Unable to delete registry subkey tree rooted at "HKLM\%s". Exception is: %v`, regKeyPath, err)
	}

	return nil
}

func resetStateHandler(lg logging.Logger, seqNum int) error {
	lg.Info("Reset State Handler")

	err := registry.DeleteKey(registry.LOCAL_MACHINE, regKeyPath)
	if err != nil {
		return fmt.Errorf(`Failed to delete registry subkey tree rooted at "HKLM\%s". Exception is: %v`, regKeyPath, err)
	}

	return nil
}

func isExtensionEnabled() (bool, error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, regKeyPath, registry.READ)
	if err != nil {
		return false, errors.Wrap(err, "failed to open registry key")
	}
	defer k.Close()

	isEnabled, _, err := k.GetStringValue(enabledRegKeyValueName)
	if err != nil {
		return false, errors.Wrap(err, fmt.Sprintf("Registry Value %s not found under the %s key beneath the %s root", enabledRegKeyValueName, regKeyPath, fmt.Sprint(registry.LOCAL_MACHINE)))
	}
	return isEnabled == "True", nil
}
