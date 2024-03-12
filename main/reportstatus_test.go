package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/Azure/applicationhealth-extension-linux/internal/handlerenv"
	"github.com/Azure/applicationhealth-extension-linux/pkg/logging"
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
	fakeEnv := handlerenv.HandlerEnvironment{}
	fakeEnv.StatusFolder = "/non-existing/dir/"
	lg, err := logging.NewExtensionLogger(&fakeEnv)
	require.NoError(t, err, "Failed to create logger")

	err = reportStatus(lg, fakeEnv, 1, StatusSuccess, cmdEnable, "")
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "failed to save handler status")
}

func Test_reportStatus_fileExists(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "")
	require.Nil(t, err)
	defer os.RemoveAll(tmpDir)

	fakeEnv := handlerenv.HandlerEnvironment{}
	fakeEnv.StatusFolder = tmpDir
	lg, err = logging.NewExtensionLogger(&fakeEnv)
	require.NoError(t, err, "Failed to create logger")

	require.Nil(t, reportStatus(lg, fakeEnv, 1, StatusError, cmdEnable, "FOO ERROR"))

	path := filepath.Join(tmpDir, "1.status")
	b, err := ioutil.ReadFile(path)
	require.Nil(t, err, ".status file exists")
	require.NotEqual(t, 0, len(b), ".status file not empty")
}

func Test_reportStatus_checksIfShouldBeReported(t *testing.T) {
	for _, c := range cmds {
		tmpDir, err := ioutil.TempDir("", "status-"+c.name)
		require.Nil(t, err)
		defer os.RemoveAll(tmpDir)

		fakeEnv := handlerenv.HandlerEnvironment{}
		fakeEnv.StatusFolder = tmpDir
		lg, err = logging.NewExtensionLogger(&fakeEnv)
		require.NoError(t, err, "Failed to create logger")

		require.Nil(t, reportStatus(lg, fakeEnv, 2, StatusSuccess, c, ""))

		fp := filepath.Join(tmpDir, "2.status")
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
