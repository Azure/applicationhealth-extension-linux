package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// spawnDetachedProcess starts a process via bash double-fork so it's not a direct
// child of the test process. This avoids zombie issues with Signal(0) checks.
// Returns the PID of the spawned process.
func spawnDetachedProcess(t *testing.T, shellCmd string) int {
	t.Helper()
	// Use bash to double-fork: the inner process writes its PID to a temp file
	pidFile := t.TempDir() + "/pid"
	cmd := exec.Command("bash", "-c",
		"("+shellCmd+" & echo $! > "+pidFile+")")
	require.NoError(t, cmd.Run(), "failed to spawn detached process")

	// Read the PID
	pidBytes, err := os.ReadFile(pidFile)
	require.NoError(t, err, "failed to read PID file")
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	require.NoError(t, err, "failed to parse PID")
	return pid
}

func Test_killProcess_SIGTERM(t *testing.T) {
	pid := spawnDetachedProcess(t, "sleep 60")

	err := killProcess(pid)
	assert.NoError(t, err, "killProcess should succeed for a process that handles SIGTERM")
}

func Test_killProcess_SIGKILL_escalation(t *testing.T) {
	// Spawn a process that ignores SIGTERM
	pid := spawnDetachedProcess(t, "trap '' TERM; sleep 60")

	startTime := time.Now()
	err := killProcess(pid)
	elapsed := time.Since(startTime)

	assert.NoError(t, err, "killProcess should succeed after escalating to SIGKILL")
	assert.True(t, elapsed >= 5*time.Second, "should have waited for SIGTERM timeout before SIGKILL")
}

func Test_killProcess_NonExistentPID(t *testing.T) {
	err := killProcess(9999999)
	assert.Error(t, err, "killProcess should return error for non-existent PID")
}

func Test_killProcess_AlreadyExited(t *testing.T) {
	cmd := exec.Command("true")
	require.NoError(t, cmd.Start(), "failed to start test process")
	pid := cmd.Process.Pid
	cmd.Wait()

	err := killProcess(pid)
	assert.Error(t, err, "killProcess should return error for already-exited process")
}

func Test_getLogFileLastWriteTimeFromEnv(t *testing.T) {
	t.Run("ReturnsTimestampWhenSet", func(t *testing.T) {
		expected := time.Now().Add(-3 * time.Minute)
		t.Setenv("HANDLER_LOG_LAST_WRITE_TIME", fmt.Sprintf("%d", expected.Unix()))

		result, err := getLogFileLastWriteTimeFromEnv()
		assert.NoError(t, err)
		assert.Equal(t, expected.Unix(), result.Unix())
	})

	t.Run("ReturnsErrorWhenNotSet", func(t *testing.T) {
		t.Setenv("HANDLER_LOG_LAST_WRITE_TIME", "")

		_, err := getLogFileLastWriteTimeFromEnv()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not set")
	})

	t.Run("ReturnsErrorWhenInvalidValue", func(t *testing.T) {
		t.Setenv("HANDLER_LOG_LAST_WRITE_TIME", "not-a-number")

		_, err := getLogFileLastWriteTimeFromEnv()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse")
	})
}

// Test_shimAndConstantsInSync reads the shim script and verifies that the
// LOG_DIR and LOG_FILE values match the Go constants. This test will fail
// if someone changes one side without updating the other.
func Test_shimAndConstantsInSync(t *testing.T) {
	shimPath := "../misc/applicationhealth-shim"
	shimBytes, err := os.ReadFile(shimPath)
	require.NoError(t, err, "failed to read shim file at %s", shimPath)
	shimContent := string(shimBytes)

	// Extract LOG_DIR value from: readonly LOG_DIR="..."
	logDirMatch := extractShimVariable(shimContent, "LOG_DIR")
	require.NotEmpty(t, logDirMatch, "could not find LOG_DIR in shim")
	assert.Equal(t, HandlerLogDir, logDirMatch,
		"HandlerLogDir constant does not match LOG_DIR in misc/applicationhealth-shim")

	// Extract LOG_FILE value from: readonly LOG_FILE=...
	logFileMatch := extractShimVariable(shimContent, "LOG_FILE")
	require.NotEmpty(t, logFileMatch, "could not find LOG_FILE in shim")
	assert.Equal(t, HandlerLogFile, logFileMatch,
		"HandlerLogFile constant does not match LOG_FILE in misc/applicationhealth-shim")
}

// extractShimVariable parses a bash variable assignment from the shim content.
// Handles both quoted (readonly VAR="value") and unquoted (readonly VAR=value) forms.
func extractShimVariable(content, varName string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		// Match: readonly VAR="value" or readonly VAR=value
		for _, prefix := range []string{
			"readonly " + varName + "=\"",
			"readonly " + varName + "=",
			varName + "=\"",
			varName + "=",
		} {
			if strings.HasPrefix(line, prefix) {
				value := strings.TrimPrefix(line, prefix)
				value = strings.TrimSuffix(value, "\"")
				value = strings.TrimSpace(value)
				return value
			}
		}
	}
	return ""
}
