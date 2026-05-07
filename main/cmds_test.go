package main

import (
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/Azure/applicationhealth-extension-linux/internal/seqno"
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

// saveAndRestoreIdempotencyMocks saves original function variables and restores
// them after the test to prevent test pollution.
func saveAndRestoreIdempotencyMocks(t *testing.T) {
	origFindExistingProcesses := findExistingProcesses
	origKillProcesses := killProcesses
	t.Cleanup(func() {
		findExistingProcesses = origFindExistingProcesses
		killProcesses = origKillProcesses
	})
	// Default to no-op kill in tests to avoid sending real signals
	killProcesses = func(lg *slog.Logger, pids []int) {}
}

// mockNoExistingProcess sets up mocks so idempotency check finds no existing process
func mockNoExistingProcess(t *testing.T) {
	saveAndRestoreIdempotencyMocks(t)
	findExistingProcesses = func() ([]int, error) { return nil, nil }
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
		mockNoExistingProcess(t)
		mockSeqNumManager.EXPECT().GetCurrentSequenceNumber(gomock.Any(), gomock.Any(), gomock.Any()).Return(uint(0), nil)
		mockSeqNumManager.EXPECT().SetSequenceNumber(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		seqnoManager = mockSeqNumManager
		err := enablePre(logger, seqNumToProcess)
		assert.NoError(t, err)
	})
	t.Run("SequenceNumberAlreadyProcessed_NoExistingProcess", func(t *testing.T) {
		// seqNumToProcess = 5, mrSeqNum = 5, no existing process → should continue
		seqNumToProcess = 5
		mockNoExistingProcess(t)
		seqnoManager = mockSeqNumManager
		mockSeqNumManager.EXPECT().GetCurrentSequenceNumber(gomock.Any(), gomock.Any(), gomock.Any()).Return(uint(5), nil)
		mockSeqNumManager.EXPECT().SetSequenceNumber(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		err := enablePre(logger, seqNumToProcess)
		assert.NoError(t, err)
	})
	t.Run("MostRecentSeqNumIsSmaller_ShouldPass", func(t *testing.T) {
		// seqNumToProcess = 4, mrSeqNum = 2
		seqNumToProcess = 4
		mockNoExistingProcess(t)
		mockSeqNumManager.EXPECT().GetCurrentSequenceNumber(gomock.Any(), gomock.Any(), gomock.Any()).Return(uint(2), nil)
		mockSeqNumManager.EXPECT().SetSequenceNumber(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		seqnoManager = mockSeqNumManager
		err := enablePre(logger, seqNumToProcess)
		assert.NoError(t, err)
	})
}

func Test_enablePre_Idempotency(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctrl := gomock.NewController(t)
	mockSeqNumManager := seqno.NewMockSequenceNumberManager(ctrl)

	// Test Case 1: Same seq + healthy process → returns errIdempotentExit
	t.Run("SameSeq_HealthyProcess_ShouldReturnIdempotentExit", func(t *testing.T) {
		saveAndRestoreIdempotencyMocks(t)
		seqnoManager = mockSeqNumManager
		mockSeqNumManager.EXPECT().GetCurrentSequenceNumber(gomock.Any(), gomock.Any(), gomock.Any()).Return(uint(20), nil)

		t.Setenv("HANDLER_LOG_LAST_WRITE_TIME", fmt.Sprintf("%d", time.Now().Add(-1*time.Minute).Unix())) // 1 minute ago = fresh
		findExistingProcesses = func() ([]int, error) {
			return []int{1234}, nil // existing process found
		}

		err := enablePre(logger, 20) // same sequence number
		assert.ErrorIs(t, err, errIdempotentExit, "should return errIdempotentExit when healthy process exists")
	})

	// Test Case 2: Same seq + unhealthy process → new process takes over
	t.Run("SameSeq_UnhealthyProcess_ShouldTakeOver", func(t *testing.T) {
		saveAndRestoreIdempotencyMocks(t)
		seqnoManager = mockSeqNumManager
		mockSeqNumManager.EXPECT().GetCurrentSequenceNumber(gomock.Any(), gomock.Any(), gomock.Any()).Return(uint(21), nil)
		mockSeqNumManager.EXPECT().SetSequenceNumber(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

		t.Setenv("HANDLER_LOG_LAST_WRITE_TIME", fmt.Sprintf("%d", time.Now().Add(-15*time.Minute).Unix())) // 15 minutes ago = stale (threshold is 6)
		findExistingProcesses = func() ([]int, error) {
			return []int{5678}, nil // existing process found
		}

		err := enablePre(logger, 21) // same sequence number
		assert.NoError(t, err, "should take over unhealthy process")
	})

	// Test Case 3: Higher seq than existing → new process should continue and kill old
	t.Run("HigherSeq_ExistingProcess_ShouldKillAndContinue", func(t *testing.T) {
		saveAndRestoreIdempotencyMocks(t)
		seqnoManager = mockSeqNumManager
		// mrSeqNum = 36, seqNum = 37 → seqNum > mrSeqNum
		mockSeqNumManager.EXPECT().GetCurrentSequenceNumber(gomock.Any(), gomock.Any(), gomock.Any()).Return(uint(36), nil)
		mockSeqNumManager.EXPECT().SetSequenceNumber(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

		// Single call — PIDs are discovered once and reused for both idempotency and kill
		findExistingProcesses = func() ([]int, error) {
			return []int{5492}, nil
		}

		err := enablePre(logger, 37)
		assert.NoError(t, err, "new process with higher seq should continue")
	})

	// Test Case 4: New seq + existing running process → should kill old and continue
	t.Run("HigherSeq_RunningProcess_ShouldKillOld", func(t *testing.T) {
		saveAndRestoreIdempotencyMocks(t)
		seqnoManager = mockSeqNumManager
		// mrSeqNum = 10, seqNum = 11 → new config
		mockSeqNumManager.EXPECT().GetCurrentSequenceNumber(gomock.Any(), gomock.Any(), gomock.Any()).Return(uint(10), nil)
		mockSeqNumManager.EXPECT().SetSequenceNumber(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

		findProcessCalled := false
		findExistingProcesses = func() ([]int, error) {
			findProcessCalled = true
			return []int{4321}, nil // existing process from old seq
		}

		err := enablePre(logger, 11) // new seq > mrSeq
		assert.NoError(t, err, "should succeed with new sequence number")
		assert.True(t, findProcessCalled, "should have checked for existing process to kill")
	})

	// Test Case 5: Lower seq (existing) + healthy process → new process exits with error
	t.Run("LowerSeq_HealthyExistingProcess_ShouldFail", func(t *testing.T) {
		saveAndRestoreIdempotencyMocks(t)
		seqnoManager = mockSeqNumManager
		// mrSeqNum = 26, seqNum = 25 → seqNum < mrSeqNum
		mockSeqNumManager.EXPECT().GetCurrentSequenceNumber(gomock.Any(), gomock.Any(), gomock.Any()).Return(uint(26), nil)

		err := enablePre(logger, 25)
		assert.Error(t, err, "lower sequence number should fail")
		assert.NotErrorIs(t, err, errIdempotentExit, "should not be idempotent exit, should be regular error")
	})

	// Test Case 6: Lower seq (existing) + unhealthy process → still fails (seq takes precedence)
	t.Run("LowerSeq_UnhealthyExistingProcess_ShouldFail", func(t *testing.T) {
		saveAndRestoreIdempotencyMocks(t)
		seqnoManager = mockSeqNumManager
		// mrSeqNum = 26, seqNum = 25 → seqNum < mrSeqNum
		mockSeqNumManager.EXPECT().GetCurrentSequenceNumber(gomock.Any(), gomock.Any(), gomock.Any()).Return(uint(26), nil)

		err := enablePre(logger, 25)
		assert.Error(t, err, "lower sequence number should fail regardless of health")
	})

	// Test Case 7: No existing AHE process → normal startup
	t.Run("SameSeq_NoExistingProcess_ShouldContinue", func(t *testing.T) {
		saveAndRestoreIdempotencyMocks(t)
		seqnoManager = mockSeqNumManager
		mockSeqNumManager.EXPECT().GetCurrentSequenceNumber(gomock.Any(), gomock.Any(), gomock.Any()).Return(uint(10), nil)
		mockSeqNumManager.EXPECT().SetSequenceNumber(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

		t.Setenv("HANDLER_LOG_LAST_WRITE_TIME", fmt.Sprintf("%d", time.Now().Add(-20*time.Minute).Unix())) // stale
		findExistingProcesses = func() ([]int, error) {
			return nil, nil // no existing process
		}

		err := enablePre(logger, 10)
		assert.NoError(t, err, "should not exit when no existing process")
	})

	// Edge case: findExistingProcesses fails → should continue gracefully
	t.Run("SameSeq_ProcessDiscoveryError_ShouldContinue", func(t *testing.T) {
		saveAndRestoreIdempotencyMocks(t)
		seqnoManager = mockSeqNumManager
		mockSeqNumManager.EXPECT().GetCurrentSequenceNumber(gomock.Any(), gomock.Any(), gomock.Any()).Return(uint(15), nil)
		mockSeqNumManager.EXPECT().SetSequenceNumber(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

		findExistingProcesses = func() ([]int, error) {
			return nil, fmt.Errorf("proc filesystem error")
		}

		err := enablePre(logger, 15)
		assert.NoError(t, err, "should continue gracefully on process discovery error")
	})

	// Edge case: No log files (file does not exist) + existing process →
	// should take over execution. If no log file exists, no previous process
	// was writing heartbeats, so the existing process is treated as unresponsive.
	// If no log file exists, no previous process was writing heartbeats,
	// so the existing process is treated as unresponsive.
	t.Run("SameSeq_NoLogFiles_ShouldTakeOver", func(t *testing.T) {
		saveAndRestoreIdempotencyMocks(t)
		seqnoManager = mockSeqNumManager
		mockSeqNumManager.EXPECT().GetCurrentSequenceNumber(gomock.Any(), gomock.Any(), gomock.Any()).Return(uint(10), nil)
		mockSeqNumManager.EXPECT().SetSequenceNumber(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

		t.Setenv("HANDLER_LOG_LAST_WRITE_TIME", "") // not set = log file doesn't exist
		findExistingProcesses = func() ([]int, error) {
			return []int{9999}, nil // process exists
		}

		err := enablePre(logger, 10)
		assert.NoError(t, err, "should take over when log file is missing")
	})
}

func Test_isLogFileFresh(t *testing.T) {
	t.Run("FreshLogFile_ShouldReturnTrue", func(t *testing.T) {
		origGetLogFileLastWriteTime := getLogFileLastWriteTime
		defer func() { getLogFileLastWriteTime = origGetLogFileLastWriteTime }()

		getLogFileLastWriteTime = func() (time.Time, error) {
			return time.Now().Add(-3 * time.Minute), nil // 3 minutes ago
		}

		fresh, lastUpdate, err := isLogFileFresh()
		assert.True(t, fresh)
		assert.False(t, lastUpdate.IsZero())
		assert.NoError(t, err)
	})

	t.Run("StaleLogFile_ShouldReturnFalse", func(t *testing.T) {
		origGetLogFileLastWriteTime := getLogFileLastWriteTime
		defer func() { getLogFileLastWriteTime = origGetLogFileLastWriteTime }()

		getLogFileLastWriteTime = func() (time.Time, error) {
			return time.Now().Add(-15 * time.Minute), nil // 15 minutes ago (threshold is 6)
		}

		fresh, lastUpdate, err := isLogFileFresh()
		assert.False(t, fresh)
		assert.False(t, lastUpdate.IsZero())
		assert.NoError(t, err)
	})

	t.Run("ErrorGettingLogFile_ShouldReturnFalse", func(t *testing.T) {
		origGetLogFileLastWriteTime := getLogFileLastWriteTime
		defer func() { getLogFileLastWriteTime = origGetLogFileLastWriteTime }()

		getLogFileLastWriteTime = func() (time.Time, error) {
			return time.Time{}, fmt.Errorf("error reading log files")
		}

		fresh, lastUpdate, err := isLogFileFresh()
		assert.False(t, fresh)
		assert.True(t, lastUpdate.IsZero())
		assert.Error(t, err)
	})
}
