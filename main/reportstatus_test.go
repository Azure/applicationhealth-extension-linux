package main

import (
	"io/ioutil"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/Azure/applicationhealth-extension-linux/internal/handlerenv"
	"github.com/Azure/applicationhealth-extension-linux/internal/telemetry"
	"github.com/stretchr/testify/require"
)

func Test_statusMsg(t *testing.T) {
	require.Equal(t, "Enable succeeded", statusMsg(cmdEnable, StatusSuccess, ""))
	require.Equal(t, "Enable succeeded: msg", statusMsg(cmdEnable, StatusSuccess, "msg"))

	require.Equal(t, "Enable failed", statusMsg(cmdEnable, StatusError, ""))
	require.Equal(t, "Enable failed: msg", statusMsg(cmdEnable, StatusError, "msg"))

	require.Equal(t, "Enable in progress", statusMsg(cmdEnable, StatusTransitioning, ""))
	require.Equal(t, "Enable in progress: msg", statusMsg(cmdEnable, StatusTransitioning, "msg"))
}

func Test_reportStatus_fails(t *testing.T) {
	fakeEnv := &handlerenv.HandlerEnvironment{}
	fakeEnv.StatusFolder = "/non-existing/dir/" // intentionally set to a non-existing directory
	fakeEnv.EventsFolder = "/non-existing/dir/" // intentionally set to a non-existing directory

	_, err := telemetry.NewTelemetry(fakeEnv)
	require.NoError(t, err, "failed to initialize telemetry")

	fakeLogger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(fakeLogger)

	err = reportStatus(fakeLogger, fakeEnv, 1, StatusSuccess, cmdEnable, "")
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "failed to save handler status")
}

func Test_reportStatus_fileExists(t *testing.T) {
	statusDir, err := os.MkdirTemp("", "status-")
	require.Nil(t, err)
	eventsDir, err := os.MkdirTemp("", "events-")
	require.Nil(t, err)
	defer os.RemoveAll(statusDir)
	defer os.RemoveAll(eventsDir)

	fakeEnv := &handlerenv.HandlerEnvironment{}
	fakeEnv.StatusFolder = statusDir
	fakeEnv.EventsFolder = eventsDir

	_, err = telemetry.NewTelemetry(fakeEnv)
	require.NoError(t, err, "failed to initialize telemetry")
	fakeLogger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(fakeLogger)

	require.Nil(t, reportStatus(fakeLogger, fakeEnv, 1, StatusError, cmdEnable, "FOO ERROR"))

	path := filepath.Join(statusDir, "1.status")
	b, err := ioutil.ReadFile(path)
	require.Nil(t, err, ".status file exists")
	require.NotEqual(t, 0, len(b), ".status file not empty")
}

func Test_reportStatus_checksIfShouldBeReported(t *testing.T) {
	for _, c := range cmds {
		statusDir, err := os.MkdirTemp("", "status-"+c.name)
		require.Nil(t, err)
		eventsDir, err := os.MkdirTemp("", "events-"+c.name)
		require.Nil(t, err)
		defer os.RemoveAll(statusDir)
		defer os.RemoveAll(eventsDir)

		fakeEnv := &handlerenv.HandlerEnvironment{}
		fakeEnv.StatusFolder = statusDir
		fakeEnv.EventsFolder = eventsDir
		_, err = telemetry.NewTelemetry(fakeEnv)
		require.NoError(t, err, "failed to initialize telemetry")
		fakeLogger := slog.New(slog.NewTextHandler(os.Stdout, nil))
		slog.SetDefault(fakeLogger)
		require.Nil(t, reportStatus(fakeLogger, fakeEnv, 2, StatusSuccess, c, ""))

		fp := filepath.Join(statusDir, "2.status")
		_, err = os.Stat(fp) // check if the .status file is there
		if c.shouldReportStatus && err != nil {
			t.Fatalf("cmd=%q should have reported status file=%q err=%v", c.name, fp, err)
		}
		if !c.shouldReportStatus {
			if err == nil {
				t.Fatalf("cmd=%q should not have reported status file. file=%q", c.name, fp)
			} else if !os.IsNotExist(err) {
				t.Fatalf("cmd=%q some other error occurred. file=%q err=%q", c.name, fp, err)
			}
		}
	}
}
