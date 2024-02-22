package version

import (
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/Azure/applicationhealth-extension-linux/pkg/utils"
)

// These fields are populated by govvv at compile-time.
var (
	Version   string
	GitCommit string
	BuildDate string
	GitState  string
)

// VersionString builds a compact version string in format:
// vVERSION/git@GitCommit[-State].
func VersionString() string {
	return fmt.Sprintf("v%s/git@%s-%s", Version, GitCommit, GitState)

}

// DetailedVersionString returns a detailed version string including version
// number, git commit, build date, source tree state and the go runtime version.
func DetailedVersionString() string {
	// e.g. v2.2.0 git:03669cef-clean build:2016-07-22T16:22:26.556103000+00:00 go:go1.6.2
	return fmt.Sprintf("v%s git:%s-%s build:%s %s", Version, GitCommit, GitState, BuildDate, runtime.Version())
}

func GetExtensionVersionFromBuild() string {
	return Version
}

// TODO: Use mode generic name
// Get Extension Version set at build time or from manifest file.
func getExtensionVersionFromManifest() (string, error) {
	// If the version is not set during build time, then reading it from the manifest file as fallback.
	processDirectory, err := utils.GetProcessDirectory()
	if err != nil {
		return "", err
	}
	processDirectory = filepath.Dir(processDirectory)
	fp := filepath.Join(processDirectory, ExtensionManifestFileName)

	manifest, err := getExtensionManifest(fp)
	if err != nil {
		return "", err
	}
	return manifest.Version, nil
}

func GetExtensionVersion() (string, error) {
	// First attempting to read the version set during build time.
	v := GetExtensionVersionFromBuild()
	if v != "" {
		return v, nil
	}

	// If the version is not set during build time, then reading it from the manifest file as fallback.

	v, err := getExtensionVersionFromManifest()

	if err != nil {
		return "", err
	}

	return v, err
}
