package main

import (
	"errors"
	"log/slog"
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
