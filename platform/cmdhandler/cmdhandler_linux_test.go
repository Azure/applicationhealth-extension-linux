package cmdhandler

import (
	"log/slog"
	"os"
	"testing"

	"github.com/Azure/applicationhealth-extension-linux/internal/seqno"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
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

func Test_enablePre(t *testing.T) {
	var (
		logger          = slog.New(slog.NewJSONHandler(os.Stdout, nil))
		seqNumToProcess uint
		ctrl            = gomock.NewController(t)
	)

	mockSeqNumManager := seqno.NewMockSequenceNumberManager(ctrl)
	t.Run("SaveSequenceNumberError_ShouldFail", func(t *testing.T) {
		// seqNumToProcess = 0, mrSeqNum = 1
		seqNumToProcess = 0
		mockSeqNumManager.EXPECT().GetCurrentSequenceNumber(gomock.Any(), gomock.Any()).Return(uint(1), nil)
		seqno.SetInstance(mockSeqNumManager)
		err := enablePre(logger, seqNumToProcess)
		assert.Error(t, err)
		assert.EqualError(t, err, "most recent sequence number 1 is greater than the requested sequence number 0")
	})
	t.Run("GetSequenceNumberIsGreaterThanRequestedSequenceNumber_ShouldFail", func(t *testing.T) {
		// seqNumToProcess = 4, mrSeqNum = 8
		seqNumToProcess = 4
		mockSeqNumManager.EXPECT().GetCurrentSequenceNumber(gomock.Any(), gomock.Any()).Return(uint(8), nil)
		seqno.SetInstance(mockSeqNumManager)
		err := enablePre(logger, seqNumToProcess)
		assert.Error(t, err)
		assert.EqualError(t, err, "most recent sequence number 8 is greater than the requested sequence number 4")
	})
	t.Run("SequenceNumberisZero_Startup", func(t *testing.T) {
		// seqNumToProcess = 0, mrSeqNum = 0
		seqNumToProcess = 0
		mockSeqNumManager.EXPECT().GetCurrentSequenceNumber(gomock.Any(), gomock.Any()).Return(uint(0), nil)
		mockSeqNumManager.EXPECT().SetSequenceNumber(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		seqno.SetInstance(mockSeqNumManager)
		err := enablePre(logger, seqNumToProcess)
		assert.NoError(t, err)
	})
	t.Run("SequenceNumberAlreadyProcessed", func(t *testing.T) {
		// seqNumToProcess = 5, mrSeqNum = 5
		seqNumToProcess = 5
		seqno.SetInstance(mockSeqNumManager)
		mockSeqNumManager.EXPECT().GetCurrentSequenceNumber(gomock.Any(), gomock.Any()).Return(uint(5), nil)
		mockSeqNumManager.EXPECT().SetSequenceNumber(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		err := enablePre(logger, seqNumToProcess)
		assert.NoError(t, err)
	})
	t.Run("MostRecentSeqNumIsSmaller_ShouldPass", func(t *testing.T) {
		// seqNumToProcess = 4, mrSeqNum = 2
		seqNumToProcess = 4
		mockSeqNumManager.EXPECT().GetCurrentSequenceNumber(gomock.Any(), gomock.Any()).Return(uint(2), nil)
		mockSeqNumManager.EXPECT().SetSequenceNumber(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		seqno.SetInstance(mockSeqNumManager)
		err := enablePre(logger, seqNumToProcess)
		assert.NoError(t, err)
	})
}
