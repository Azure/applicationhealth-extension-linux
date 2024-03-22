package manifest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_ExtensionManifestVersion(t *testing.T) {
	// Save the original function and restore it after the test
	originalGetDir := getDir
	defer func() { getDir = originalGetDir }()

	currVersion := "2.0.10"
	expectedManifest := ExtensionManifest{
		ProviderNameSpace:   "Microsoft.ManagedServices",
		Type:                "ApplicationHealthWindows",
		Version:             currVersion,
		Label:               "Microsoft Azure Application Health Extension for Windows Virtual Machines",
		HostingResources:    "VmRole",
		MediaLink:           "",
		Description:         "Microsoft Azure Application Health Extension is an extension installed on a VM to report the health of the application running on the VM.",
		IsInternalExtension: true,
		IsJsonExtension:     true,
		SupportedOS:         "Windows",
		CompanyName:         "Microsoft",
	}
	// Override the getDir function to return a mock directory
	getDir = func() (string, error) {
		return "../../misc/windows", nil
	}

	currentManifest, err := GetExtensionManifest()
	require.Nil(t, err)
	require.Equal(t, expectedManifest.Type, currentManifest.Type)
	require.Equal(t, expectedManifest.Label, currentManifest.Label)
	require.Equal(t, expectedManifest.HostingResources, currentManifest.HostingResources)
	require.Equal(t, expectedManifest.MediaLink, currentManifest.MediaLink)
	require.Equal(t, expectedManifest.Description, currentManifest.Description)
	require.Equal(t, expectedManifest.IsInternalExtension, currentManifest.IsInternalExtension)
	require.Equal(t, expectedManifest.IsJsonExtension, currentManifest.IsJsonExtension)
	require.Equal(t, expectedManifest.SupportedOS, currentManifest.SupportedOS)
	require.Equal(t, expectedManifest.CompanyName, currentManifest.CompanyName)
}

func Test_FindManifestFilePath(t *testing.T) {
	var (
		manifestFileName = "manifest.xml"
		src              = "../../misc/windows"                                                 // Replace with the actual directory path
		dst              = "/tmp/lib/waagent/Microsoft.ManagedServices-ApplicationHealthLinux/" // Replace with the actual directory path
	)

	err := copyFileNewDirectory(src, dst, manifestFileName)
	require.NoError(t, err, "failed to copy manifest file to a new directory")
	defer os.RemoveAll(dst)

	// Test case 1: Manifest file exists in the current directory
	workingDir := filepath.Join(dst, "bin")
	path, err := findManifestFilePath(workingDir)
	require.NoError(t, err)
	require.Equalf(t, filepath.Join(dst, manifestFileName), path, "failed to find manifest file from bin directory: %s", workingDir)

	// Test case 2: Manifest file exists in the parent directory
	workingDir = filepath.Join(workingDir, "..")
	path, err = findManifestFilePath(workingDir)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(dst, manifestFileName), path, "failed to find manifest file from parent directory: %s", workingDir)

	os.Remove(filepath.Join(dst, manifestFileName))
	// Test case 3: Manifest file does not exist
	workingDir = filepath.Join(dst, "bin")
	path, err = findManifestFilePath(workingDir)
	require.Error(t, err)
	require.EqualError(t, err, fmt.Sprintf("cannot find HandlerEnvironment at paths: %s", strings.Join([]string{filepath.Join(workingDir, manifestFileName), filepath.Join(workingDir, "..", manifestFileName)}, ", ")))
	require.Equal(t, "", path)
}
