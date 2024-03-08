package cmdhandler

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/Azure/applicationhealth-extension-linux/internal/handlerenv"
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
	fakeEnv := handlerenv.HandlerEnvironment{}
	fakeEnv.HandlerEnvironment.StatusFolder = "/non-existing/dir/"
	lg, err := logging.NewExtensionLogger(&fakeEnv)
	require.NoError(t, err, "failed to create logger")

	h, err := NewCommandHandler()
	require.Nil(t, err)
	cmds := h.CommandMap()
	require.NotNil(t, cmds)

	err = ReportStatus(lg, fakeEnv, 1, status.StatusSuccess, cmds["enable"], "")
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "failed to save handler status")
}

func Test_reportStatus_fileExists(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "")
	require.Nil(t, err)
	defer os.RemoveAll(tmpDir)

	fakeEnv := handlerenv.HandlerEnvironment{}
	fakeEnv.HandlerEnvironment.StatusFolder = tmpDir
	lg, err := logging.NewExtensionLogger(&fakeEnv)
	require.NoError(t, err, "failed to create logger")

	// Setting Up CommandHandler
	h, err := NewCommandHandler()
	require.Nil(t, err)
	cmds := h.CommandMap()
	require.NotNil(t, cmds)

	require.Nil(t, ReportStatus(lg, fakeEnv, 1, status.StatusError, cmds["enable"], "FOO ERROR"))

	path := filepath.Join(tmpDir, "1.status")
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
		tmpDir, err := ioutil.TempDir("", "status-"+c.Name.String())
		require.Nil(t, err)
		defer os.RemoveAll(tmpDir)

		fakeEnv := handlerenv.HandlerEnvironment{}
		fakeEnv.HandlerEnvironment.StatusFolder = tmpDir
		lg, err := logging.NewExtensionLogger(&fakeEnv)
		require.NoError(t, err, "failed to create logger")

		require.Nil(t, ReportStatus(lg, fakeEnv, 2, status.StatusSuccess, c, ""))

		fp := filepath.Join(tmpDir, "2.status")
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
