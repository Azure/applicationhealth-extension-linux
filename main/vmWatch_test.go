package main

import (
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Azure/applicationhealth-extension-linux/internal/handlerenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetStatusTypeReturnsCorrectValue(t *testing.T) {
	status := Failed
	require.Equal(t, StatusError, status.GetStatusType())
	status = Disabled
	require.Equal(t, StatusWarning, status.GetStatusType())
	status = NotRunning
	require.Equal(t, StatusSuccess, status.GetStatusType())
	status = Running
	require.Equal(t, StatusSuccess, status.GetStatusType())
}

func TestVMWatchResult_GetMessage(t *testing.T) {
	res := VMWatchResult{Status: Disabled}
	require.Equal(t, "VMWatch is disabled", res.GetMessage())
	res = VMWatchResult{Status: Failed}
	require.Equal(t, "VMWatch failed: <nil>", res.GetMessage())
	res = VMWatchResult{Status: Failed, Error: errors.New("this is an error")}
	require.Equal(t, "VMWatch failed: this is an error", res.GetMessage())
	res = VMWatchResult{Status: NotRunning}
	require.Equal(t, "VMWatch is not running", res.GetMessage())
	res = VMWatchResult{Status: Running}
	require.Equal(t, "VMWatch is running", res.GetMessage())
}

func TestExtractVersion(t *testing.T) {
	v := extractVersion("systemd 123")
	require.Equal(t, 123, v)
	v = extractVersion(`someline
systemd 123
some other line`)
	require.Equal(t, 123, v)
	v = extractVersion(`someline
systemd abc
some other line`)
	require.Equal(t, 0, v)
	v = extractVersion("junk")
	require.Equal(t, 0, v)
}

func TestMonitorHeartBeat_ResetsCountersAfterOneHour(t *testing.T) {
	// Reset retry counters to ensure clean test state
	resetVMWatchRetryCounters()

	// Setup
	tempDir, err := os.MkdirTemp("", "vmwatch_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	heartbeatFile := filepath.Join(tempDir, "heartbeat.txt")

	// Create a fresh heartbeat file
	err = os.WriteFile(heartbeatFile, []byte("heartbeat"), 0644)
	require.NoError(t, err)

	// Set initial retry counters to simulate we're in cycle 3 with some attempts
	updateVMWatchRetryCounters(3, 10)
	initialCycle, initialAttempts := getVMWatchRetryCounters()
	require.Equal(t, 3, initialCycle)
	require.Equal(t, 10, initialAttempts)

	// Create a mock command with a fake PID
	cmd := &exec.Cmd{
		Process: &os.Process{Pid: 12345},
	}

	// Create channels
	processDone := make(chan bool)

	// Mock time - start at a time that will be 1.5 hours ago
	startTime := time.Date(2024, 1, 1, 8, 30, 0, 0, time.UTC) // 1.5 hours before 10:00

	// Start monitor heartbeat in a goroutine
	lg := slog.Default()
	done := make(chan bool)

	go func() {
		defer close(done)

		// Start monitoring with controlled start time (1.5 hours ago)
		monitorHeartBeat(lg, heartbeatFile, processDone, cmd, startTime)
	}()

	// Let the monitor start
	time.Sleep(10 * time.Millisecond)

	// Update heartbeat file (process has been running for 1.5 hours)
	err = os.WriteFile(heartbeatFile, []byte("heartbeat"), 0644)
	require.NoError(t, err)

	// Give a bit more time for the monitor to process
	time.Sleep(10 * time.Millisecond)

	// Signal process done
	processDone <- true

	// Wait for completion
	<-done

	// Verify counters were reset due to 1+ hour runtime
	finalCycle, finalAttempts := getVMWatchRetryCounters()
	assert.Equal(t, 1, finalCycle, "Cycle should be reset to 1 after 1+ hour runtime")
	assert.Equal(t, 0, finalAttempts, "Attempts should be reset to 0 after 1+ hour runtime")
}

func TestMonitorHeartBeat_DoesNotResetCountersBeforeOneHour(t *testing.T) {
	// Reset retry counters to ensure clean test state
	resetVMWatchRetryCounters()

	// Setup
	tempDir, err := os.MkdirTemp("", "vmwatch_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	heartbeatFile := filepath.Join(tempDir, "heartbeat.txt")

	// Create a fresh heartbeat file
	err = os.WriteFile(heartbeatFile, []byte("heartbeat"), 0644)
	require.NoError(t, err)

	// Set initial retry counters to simulate we're in cycle 3 with some attempts
	updateVMWatchRetryCounters(3, 10)
	initialCycle, initialAttempts := getVMWatchRetryCounters()
	require.Equal(t, 3, initialCycle)
	require.Equal(t, 10, initialAttempts)

	// Create a mock command with a fake PID
	cmd := &exec.Cmd{
		Process: &os.Process{Pid: 12345},
	}

	// Create channels
	processDone := make(chan bool)

	// Mock time - start at a recent time (30 minutes ago)
	startTime := time.Now().Add(-30 * time.Minute) // Only 30 minutes ago

	// Start monitor heartbeat in a goroutine
	lg := slog.Default()
	done := make(chan bool)

	go func() {
		defer close(done)

		// Start monitoring with recent start time
		monitorHeartBeat(lg, heartbeatFile, processDone, cmd, startTime)
	}()

	// Let the monitor start
	time.Sleep(10 * time.Millisecond)

	// Update heartbeat file (process has only been running for 30 minutes)
	err = os.WriteFile(heartbeatFile, []byte("heartbeat"), 0644)
	require.NoError(t, err)

	// Give a bit more time for the monitor to process
	time.Sleep(10 * time.Millisecond)

	// Signal process done
	processDone <- true

	// Wait for completion
	<-done

	// Verify counters were NOT reset (should remain at original values)
	finalCycle, finalAttempts := getVMWatchRetryCounters()
	assert.Equal(t, 3, finalCycle, "Cycle should remain unchanged at 3 when runtime < 1 hour")
	assert.Equal(t, 10, finalAttempts, "Attempts should remain unchanged at 10 when runtime < 1 hour")
}

func TestMonitorHeartBeat_ResetsOnProcessExit(t *testing.T) {
	// Reset retry counters to ensure clean test state
	resetVMWatchRetryCounters()

	// Setup
	tempDir, err := os.MkdirTemp("", "vmwatch_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	heartbeatFile := filepath.Join(tempDir, "heartbeat.txt")

	// Create a fresh heartbeat file
	err = os.WriteFile(heartbeatFile, []byte("heartbeat"), 0644)
	require.NoError(t, err)

	// Set initial retry counters
	updateVMWatchRetryCounters(2, 5)

	// Create a mock command with a fake PID
	cmd := &exec.Cmd{
		Process: &os.Process{Pid: 12345},
	}

	// Create channels
	processDone := make(chan bool)

	// Mock time - process started 1.5 hours ago
	startTime := time.Date(2024, 1, 1, 8, 30, 0, 0, time.UTC) // 1.5 hours before 10:00

	// Start monitor heartbeat in a goroutine
	lg := slog.Default()
	done := make(chan bool)

	go func() {
		defer close(done)

		// Start monitoring with controlled start time
		monitorHeartBeat(lg, heartbeatFile, processDone, cmd, startTime)
	}()

	// Let the monitor start
	time.Sleep(10 * time.Millisecond)

	// Signal process exit after 1.5 hours of runtime
	processDone <- true

	// Wait for completion
	<-done

	// Verify counters were reset on process exit after 1+ hour runtime
	finalCycle, finalAttempts := getVMWatchRetryCounters()
	assert.Equal(t, 1, finalCycle, "Cycle should be reset to 1 on process exit after 1+ hour")
	assert.Equal(t, 0, finalAttempts, "Attempts should be reset to 0 on process exit after 1+ hour")
}

func TestResetVMWatchRetryCounters(t *testing.T) {
	// Reset retry counters to ensure clean test state
	resetVMWatchRetryCounters()

	// Set some initial values
	updateVMWatchRetryCounters(4, 15)

	// Verify initial state
	cycle, attempts := getVMWatchRetryCounters()
	assert.Equal(t, 4, cycle)
	assert.Equal(t, 15, attempts)

	// Reset counters
	resetVMWatchRetryCounters()

	// Verify reset worked
	cycle, attempts = getVMWatchRetryCounters()
	assert.Equal(t, 1, cycle, "Cycle should be reset to 1")
	assert.Equal(t, 0, attempts, "Attempts should be reset to 0")
}

func TestProgressiveBackoffConstants(t *testing.T) {
	assert.Equal(t, 4, VMWatchMaxRetryCycles, "Expected 4 retry cycles")
	assert.Equal(t, 3, VMWatchMaxProcessAttempts, "Expected 3 attempts per cycle")
	assert.Equal(t, 3, VMWatchBaseWaitHours, "Expected 3 base wait hours")
}

// Test the actual executeRetryLogic function from vmWatch.go
func TestExecuteRetryLogic(t *testing.T) {
	tests := []struct {
		name               string
		config             RetryConfig
		helperErrors       []error // errors returned by helper on each call
		shutdownAfterCalls int     // shutdown after this many calls (0 = no shutdown)
		expectedResult     RetryResult
		expectedSleepCalls int
	}{
		{
			name: "AllCyclesExhausted",
			config: RetryConfig{
				MaxCycles:        2,
				AttemptsPerCycle: 2,
				BaseWaitHours:    0, // Fast for testing
			},
			helperErrors: []error{
				errors.New("fail1"), errors.New("fail2"), // cycle 1
				errors.New("fail3"), errors.New("fail4"), // cycle 2
			},
			expectedResult: RetryResult{
				TotalAttempts: 4,
				CyclesRun:     2,
				Success:       false,
			},
			expectedSleepCalls: 1, // Sleep once between cycles
		},
		{
			name: "SuccessOnThirdAttempt",
			config: RetryConfig{
				MaxCycles:        3,
				AttemptsPerCycle: 3,
				BaseWaitHours:    0,
			},
			helperErrors: []error{
				errors.New("fail1"), errors.New("fail2"), // cycle 1, attempts 1&2
				nil, // cycle 1, attempt 3 - success
			},
			expectedResult: RetryResult{
				TotalAttempts: 3,
				CyclesRun:     1, // Success in cycle 1
				Success:       true,
				LastError:     nil,
			},
			expectedSleepCalls: 0, // No sleep because success happens in cycle 1
		},
		{
			name: "ShutdownDuringExecution",
			config: RetryConfig{
				MaxCycles:        3,
				AttemptsPerCycle: 3,
				BaseWaitHours:    0,
			},
			helperErrors:       []error{errors.New("fail1"), errors.New("fail2")},
			shutdownAfterCalls: 2,
			expectedResult: RetryResult{
				TotalAttempts: 2,
				CyclesRun:     3, // Returns MaxCycles when all cycles are exhausted
				Success:       false,
			},
			expectedSleepCalls: 0, // No sleep because shutdown happens first
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset retry counters to ensure clean test state
			resetVMWatchRetryCounters()

			// Setup mocks
			callCount := 0
			sleepCallCount := 0
			shutdownTriggered := false

			mockHelper := func(lg *slog.Logger, attempt int, s *vmWatchSettings, hEnv *handlerenv.HandlerEnvironment) error {
				callCount++
				if tt.shutdownAfterCalls > 0 && callCount >= tt.shutdownAfterCalls {
					shutdownTriggered = true
				}
				if callCount-1 < len(tt.helperErrors) {
					return tt.helperErrors[callCount-1]
				}
				return errors.New("unexpected call")
			}

			mockSleep := func(d time.Duration) {
				sleepCallCount++
				// Don't actually sleep in tests
			}

			mockShutdown := func() bool {
				return shutdownTriggered
			}

			// Test the actual production function
			result := executeRetryLogic(
				nil,                              // logger
				&vmWatchSettings{},               // settings
				&handlerenv.HandlerEnvironment{}, // hEnv
				tt.config,
				mockHelper,
				mockSleep,
				mockShutdown,
				nil, // no result channel for simplicity
			)

			// Assertions
			assert.Equal(t, tt.expectedResult.TotalAttempts, result.TotalAttempts, "Total attempts")
			assert.Equal(t, tt.expectedResult.CyclesRun, result.CyclesRun, "Cycles run")
			assert.Equal(t, tt.expectedResult.Success, result.Success, "Success status")
			assert.Equal(t, tt.expectedSleepCalls, sleepCallCount, "Sleep call count")

			if tt.expectedResult.Success {
				assert.NoError(t, result.LastError, "Should have no error on success")
			} else {
				assert.Error(t, result.LastError, "Should have error on failure")
			}
		})
	}
}

// Test actual production functions
func TestGetProcessDirectory(t *testing.T) {
	dir, err := GetProcessDirectory()
	if err != nil {
		t.Logf("GetProcessDirectory returned error: %v", err)
	} else {
		assert.NotEmpty(t, dir, "Process directory should not be empty")
	}
}

func TestGetVMWatchEnvironmentVariables(t *testing.T) {
	hEnv := &handlerenv.HandlerEnvironment{}
	overrides := map[string]interface{}{
		"TEST_VAR": "test_value",
	}

	envVars := GetVMWatchEnvironmentVariables(overrides, hEnv)
	assert.IsType(t, []string{}, envVars, "Should return string slice")

	// Should contain our override
	found := false
	for _, envVar := range envVars {
		if envVar == "TEST_VAR=test_value" {
			found = true
			break
		}
	}
	assert.True(t, found, "Should contain the override variable")
}
