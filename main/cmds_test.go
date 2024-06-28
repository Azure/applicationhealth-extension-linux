package main

import (
	"testing"

	"github.com/Azure/applicationhealth-extension-linux/internal/seqno"
	"github.com/go-kit/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func Test_commandsExist(t *testing.T) {
	// we expect these subcommands to be handled
	expect := []string{"install", "enable", "disable", "uninstall", "update"}
	for _, c := range expect {
		_, ok := cmds[c]
		if !ok {
			t.Fatalf("cmd '%s' is not handled", c)
		}
	}
}

func Test_commands_shouldReportStatus(t *testing.T) {
	// - certain extension invocations are supposed to write 'N.status' files and some do not.

	// these subcommands should NOT report status
	require.False(t, cmds["install"].shouldReportStatus, "install should not report status")
	require.False(t, cmds["uninstall"].shouldReportStatus, "uninstall should not report status")

	// these subcommands SHOULD report status
	require.True(t, cmds["enable"].shouldReportStatus, "enable should report status")
	require.True(t, cmds["disable"].shouldReportStatus, "disable should report status")
	require.True(t, cmds["update"].shouldReportStatus, "update should report status")
}

func Test_enablePre(t *testing.T) {
	var (
		logger          = log.NewNopLogger()
		seqNumToProcess uint
		ctrl            = gomock.NewController(t)
	)

	mockSeqNumManager := seqno.NewMockSequenceNumberManager(ctrl)
	t.Run("SaveSequenceNumberError_ShouldFail", func(t *testing.T) {
		// seqNumToProcess = 0, mrSeqNum = 1
		seqNumToProcess = 0
		mockSeqNumManager.EXPECT().GetCurrentSequenceNumber(gomock.Any(), gomock.Any(), gomock.Any()).Return(uint(1), nil)
		seqnoManager = mockSeqNumManager
		err := enablePre(logger, seqNumToProcess)
		assert.Error(t, err)
		assert.EqualError(t, err, "most recent sequence number 1 is greater than the requested sequence number 0")
	})
	t.Run("GetSequenceNumberIsGreaterThanRequestedSequenceNumber_ShouldFail", func(t *testing.T) {
		// seqNumToProcess = 4, mrSeqNum = 8
		seqNumToProcess = 4
		mockSeqNumManager.EXPECT().GetCurrentSequenceNumber(gomock.Any(), gomock.Any(), gomock.Any()).Return(uint(8), nil)
		seqnoManager = mockSeqNumManager
		err := enablePre(logger, seqNumToProcess)
		assert.Error(t, err)
		assert.EqualError(t, err, "most recent sequence number 8 is greater than the requested sequence number 4")
	})
	t.Run("SequenceNumberisZero_Startup", func(t *testing.T) {
		// seqNumToProcess = 0, mrSeqNum = 0
		seqNumToProcess = 0
		mockSeqNumManager.EXPECT().GetCurrentSequenceNumber(gomock.Any(), gomock.Any(), gomock.Any()).Return(uint(0), nil)
		mockSeqNumManager.EXPECT().SetSequenceNumber(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		seqnoManager = mockSeqNumManager
		err := enablePre(logger, seqNumToProcess)
		assert.NoError(t, err)
	})
	t.Run("SequenceNumberAlreadyProcessed", func(t *testing.T) {
		// seqNumToProcess = 5, mrSeqNum = 5
		seqNumToProcess = 5
		seqnoManager = mockSeqNumManager
		mockSeqNumManager.EXPECT().GetCurrentSequenceNumber(gomock.Any(), gomock.Any(), gomock.Any()).Return(uint(5), nil)
		mockSeqNumManager.EXPECT().SetSequenceNumber(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		err := enablePre(logger, seqNumToProcess)
		assert.NoError(t, err)
	})
	t.Run("MostRecentSeqNumIsSmaller_ShouldPass", func(t *testing.T) {
		// seqNumToProcess = 4, mrSeqNum = 2
		seqNumToProcess = 4
		mockSeqNumManager.EXPECT().GetCurrentSequenceNumber(gomock.Any(), gomock.Any(), gomock.Any()).Return(uint(2), nil)
		mockSeqNumManager.EXPECT().SetSequenceNumber(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		seqnoManager = mockSeqNumManager
		err := enablePre(logger, seqNumToProcess)
		assert.NoError(t, err)
	})
}
