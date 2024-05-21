package cmdhandler

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_extCommands(t *testing.T) {
	commands := extCommands

	// Test install command
	installCmd, ok := commands[InstallKey]
	require.True(t, ok, "install command should exist")
	require.Equal(t, "Install", installCmd.Name.String(), "install command name should be 'Install'")
	require.False(t, installCmd.ShouldReportStatus, "install command should not report status")
	require.Equal(t, 52, installCmd.failExitCode, "install command failExitCode should be 52")

	// Test uninstall command
	uninstallCmd, ok := commands[UninstallKey]
	require.True(t, ok, "uninstall command should exist")
	require.Equal(t, "Uninstall", uninstallCmd.Name.String(), "uninstall command name should be 'Uninstall'")
	require.False(t, uninstallCmd.ShouldReportStatus, "uninstall command should not report status")
	require.Equal(t, 3, uninstallCmd.failExitCode, "uninstall command failExitCode should be 3")

	// Test enable command
	enableCmd, ok := commands[EnableKey]
	require.True(t, ok, "enable command should exist")
	require.Equal(t, "Enable", enableCmd.Name.String(), "enable command name should be Enable'")
	require.True(t, enableCmd.ShouldReportStatus, "enable command should report status")
	require.Equal(t, 3, enableCmd.failExitCode, "enable command failExitCode should be 3")

	// Test update command
	updateCmd, ok := commands[UpdateKey]
	require.True(t, ok, "update command should exist")
	require.Equal(t, "Update", updateCmd.Name.String(), "update command name should be 'Update'")
	require.True(t, updateCmd.ShouldReportStatus, "update command should report status")
	require.Equal(t, 3, updateCmd.failExitCode, "update command failExitCode should be 3")

	// Test disable command
	disableCmd, ok := commands[DisableKey]
	require.True(t, ok, "disable command should exist")
	require.Equal(t, "Disable", disableCmd.Name.String(), "disable command name should be 'Disable'")
	require.True(t, disableCmd.ShouldReportStatus, "disable command should report status")
	require.Equal(t, 3, disableCmd.failExitCode, "disable command failExitCode should be 3")
}
