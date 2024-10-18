package cmdhandler

import (
	"io/ioutil"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/Azure/applicationhealth-extension-linux/internal/handlerenv"
	"github.com/Azure/applicationhealth-extension-linux/internal/telemetry"
	"github.com/Azure/applicationhealth-extension-linux/pkg/logging"
	"github.com/Azure/applicationhealth-extension-linux/pkg/status"
	"github.com/stretchr/testify/require"
)

func Test_statusMsg(t *testing.T) {
	h, err := NewCommandHandler()
	require.Nil(t, err)
	cmds := h.CommandMap()
	require.NotNil(t, cmds)

	require.Equal(t, "Enable succeeded", statusMsg(cmds["enable"], status.StatusSuccess, ""))
	require.Equal(t, "Enable succeeded: msg", statusMsg(cmds["enable"], status.StatusSuccess, "msg"))

	require.Equal(t, "Enable failed", statusMsg(cmds["enable"], status.StatusError, ""))
	require.Equal(t, "Enable failed: msg", statusMsg(cmds["enable"], status.StatusError, "msg"))

	require.Equal(t, "Enable in progress", statusMsg(cmds["enable"], status.StatusTransitioning, ""))
	require.Equal(t, "Enable in progress: msg", statusMsg(cmds["enable"], status.StatusTransitioning, "msg"))
}

func Test_reportStatus_fails(t *testing.T) {
	fakeEnv := &handlerenv.HandlerEnvironment{}
	fakeEnv.StatusFolder = "/non-existing/dir/" // intentionally set to a non-existing directory
	fakeEnv.EventsFolder = "/non-existing/dir/" // intentionally set to a non-existing directory

	_, err := telemetry.NewTelemetry(fakeEnv)
	require.NoError(t, err, "failed to initialize telemetry")

	lg, err := logging.NewSlogLogger(fakeEnv, "")
	require.NoError(t, err, "failed to create logger")
	slog.SetDefault(lg)

	h, err := NewCommandHandler()
	require.Nil(t, err)
	cmds := h.CommandMap()
	require.NotNil(t, cmds)

	err = ReportStatus(lg, fakeEnv, 1, status.StatusSuccess, cmds["enable"], "")
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

	lg, err := logging.NewSlogLogger(fakeEnv, "")
	require.NoError(t, err, "failed to create logger")
	slog.SetDefault(lg)

	// Setting Up CommandHandler
	h, err := NewCommandHandler()
	require.Nil(t, err)
	cmds := h.CommandMap()
	require.NotNil(t, cmds)

	require.Nil(t, ReportStatus(lg, fakeEnv, 1, status.StatusError, cmds["enable"], "FOO ERROR"))

	path := filepath.Join(statusDir, "1.status")
	b, err := ioutil.ReadFile(path)
	require.Nil(t, err, ".status file exists")
	require.NotEqual(t, 0, len(b), ".status file not empty")
}

func Test_reportStatus_checksIfShouldBeReported(t *testing.T) {
	// Setting Up CommandHandler
	h, err := NewCommandHandler()
	require.Nil(t, err)
	cmds := h.CommandMap()
	require.NotNil(t, cmds)

	for _, c := range cmds.Values() {
		statusDir, err := os.MkdirTemp("", "status-"+c.Name.String())
		require.Nil(t, err)
		eventsDir, err := os.MkdirTemp("", "events-"+c.Name.String())
		require.Nil(t, err)
		defer os.RemoveAll(statusDir)
		defer os.RemoveAll(eventsDir)

		fakeEnv := &handlerenv.HandlerEnvironment{}
		fakeEnv.StatusFolder = statusDir
		fakeEnv.EventsFolder = eventsDir
		lg, err := logging.NewSlogLogger(fakeEnv, "")
		require.NoError(t, err, "failed to create logger")
		slog.SetDefault(lg)

		require.Nil(t, ReportStatus(lg, fakeEnv, 2, status.StatusSuccess, c, ""))

		fp := filepath.Join(statusDir, "2.status")
		_, err = os.Stat(fp) // check if the .status file is there
		if c.ShouldReportStatus && err != nil {
			t.Fatalf("cmd=%q should have reported status file=%q err=%v", c.Name, fp, err)
		}
		if !c.ShouldReportStatus {
			if err == nil {
				t.Fatalf("cmd=%q should not have reported status file. file=%q", c.Name, fp)
			} else if !os.IsNotExist(err) {
				t.Fatalf("cmd=%q some other error occurred. file=%q err=%q", c.Name, fp, err)
			}
		}
	}
}
