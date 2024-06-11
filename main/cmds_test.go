package main

import (
	"testing"

	"github.com/Azure/applicationhealth-extension-linux/internal/seqno"
	"github.com/Azure/azure-extension-platform/pkg/extensionerrors"
	"github.com/go-kit/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		logger = log.NewNopLogger()
		seqNum = 5
	)

	t.Run("SequenceNumberAlreadyProcessed", func(t *testing.T) {
		mockSeqNumManager := &seqno.SeqNumManager{
			GetSequenceNumberFunc:    seqno.GetSequenceNumberFunc,
			SequenceNumberSetterFunc: seqno.SetSequenceNumber,
			FindSeqNumFunc:           seqno.FindSeqNum,
			GetCurrentSequenceNumberFunc: func(lg log.Logger, name, version string) (int, error) {
				return 5, nil
			},
		}
		seqnoManager = mockSeqNumManager
		err := enablePre(logger, seqNum)
		assert.Error(t, err)
		assert.EqualError(t, err, "most recent sequence number 5 is greater than or equal to requested sequence number 5")
	})

	t.Run("SaveSequenceNumberError_ShouldFail", func(t *testing.T) {
		seqNum = 0
		mockSeqNumManager := &seqno.SeqNumManager{
			GetSequenceNumberFunc:    seqno.GetSequenceNumberFunc,
			SequenceNumberSetterFunc: seqno.SetSequenceNumber,
			FindSeqNumFunc:           seqno.FindSeqNum,
			GetCurrentSequenceNumberFunc: func(lg log.Logger, name, version string) (int, error) {
				return 1, nil
			},
		}
		seqnoManager = mockSeqNumManager
		err := enablePre(logger, seqNum)
		assert.Error(t, err)
		assert.EqualError(t, err, "most recent sequence number 1 is greater than or equal to requested sequence number 0")
	})

	t.Run("SequenceNumberisZero_ShouldPass", func(t *testing.T) {
		seqNum = 0
		mockSeqNumManager := &seqno.SeqNumManager{
			GetSequenceNumberFunc:    seqno.GetSequenceNumberFunc,
			SequenceNumberSetterFunc: seqno.SetSequenceNumber,
			FindSeqNumFunc:           seqno.FindSeqNum,
			GetCurrentSequenceNumberFunc: func(lg log.Logger, name, version string) (int, error) {
				return 0, nil
			},
		}
		seqnoManager = mockSeqNumManager
		err := enablePre(logger, seqNum)
		assert.NoError(t, err)
	})
	t.Run("MrSeqFileNotFound_ShouldPass", func(t *testing.T) {
		seqNum = 0
		mockGetSequenceNumberFunc := func(name, version string) (int, error) {
			return 0, extensionerrors.ErrNoMrseqFile
		}
		mockSeqNumManager := &seqno.SeqNumManager{
			GetSequenceNumberFunc:        mockGetSequenceNumberFunc,
			SequenceNumberSetterFunc:     seqno.SetSequenceNumber,
			FindSeqNumFunc:               seqno.FindSeqNum,
			GetCurrentSequenceNumberFunc: seqno.GetCurrentSequenceNumberFunc(mockGetSequenceNumberFunc),
		}
		seqnoManager = mockSeqNumManager
		err := enablePre(logger, seqNum)
		assert.NoError(t, err)
	})
	t.Run("GetSequenceNumberIsGreaterThanRequestedSequenceNumber_ShouldFail", func(t *testing.T) {
		seqNum = 4
		mockSeqNumManager := &seqno.SeqNumManager{
			GetSequenceNumberFunc:    seqno.GetSequenceNumberFunc,
			SequenceNumberSetterFunc: seqno.SetSequenceNumber,
			FindSeqNumFunc:           seqno.FindSeqNum,
			GetCurrentSequenceNumberFunc: func(lg log.Logger, name, version string) (int, error) {
				return 8, nil
			},
		}
		seqnoManager = mockSeqNumManager
		err := enablePre(logger, seqNum)
		assert.Error(t, err)
		assert.EqualError(t, err, "most recent sequence number 8 is greater than or equal to requested sequence number 4")
	})
	t.Run("GetSequenceNumberIsEqualRequestedSequenceNumber_ShouldFail", func(t *testing.T) {
		seqNum = 4
		mockSeqNumManager := &seqno.SeqNumManager{
			GetSequenceNumberFunc:    seqno.GetSequenceNumberFunc,
			SequenceNumberSetterFunc: seqno.SetSequenceNumber,
			FindSeqNumFunc:           seqno.FindSeqNum,
			GetCurrentSequenceNumberFunc: func(lg log.Logger, name, version string) (int, error) {
				return 4, nil
			},
		}
		seqnoManager = mockSeqNumManager
		err := enablePre(logger, seqNum)
		assert.Error(t, err)
		assert.EqualError(t, err, "most recent sequence number 4 is greater than or equal to requested sequence number 4")
	})
}
