package version

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVersionString(t *testing.T) {
	defer resetStrings()

	Version = "1.0.0"
	GitCommit = "03669cef"
	GitState = "dirty"
	require.Equal(t, "v1.0.0/git@03669cef-dirty", VersionString())
}

func TestDetailedVersionString(t *testing.T) {
	defer resetStrings()

	Version = "1.0.0"
	GitCommit = "03669cef"
	GitState = "dirty"
	BuildDate = "DATE"
	goVersion := runtime.Version()
	require.Equal(t, "v1.0.0 git:03669cef-dirty build:DATE "+goVersion, DetailedVersionString())
}

func TestVersion_GetVersionFromBuild(t *testing.T) {
	Version = "2.0.8"
	GitState = "dirty"
	GitCommit = "03669cef"

	version, err := GetExtensionVersion()
	require.Nil(t, err)
	require.Equal(t, "2.0.8", version)
}

func resetStrings() { Version, GitState, GitCommit, BuildDate = "", "", "", "" }
