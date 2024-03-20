package manifest

import (
	"fmt"
	"io"
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

	currVersion := "2.0.8"
	expectedManifest := ExtensionManifest{
		ProviderNameSpace:   "Microsoft.ManagedServices",
		Type:                "ApplicationHealthLinux",
		Version:             currVersion,
		Label:               "Microsoft Azure Application Health Extension for Linux Virtual Machines",
		HostingResources:    "VmRole",
		MediaLink:           "",
		Description:         "Microsoft Azure Application Health Extension is an extension installed on a VM to periodically determine configured application health.",
		IsInternalExtension: true,
		IsJsonExtension:     true,
		SupportedOS:         "Linux",
		CompanyName:         "Microsoft",
	}
	// Override the getDir function to return a mock directory
	getDir = func() (string, error) {
		return "../../misc", nil
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
		src              = "../../misc"
		dst              = "/tmp/lib/waagent/Microsoft.ManagedServices-ApplicationHealthLinux/"
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

func copyFileNewDirectory(src, dst, fileName string) error {
	// This function is used to copy the manifest file to a new directory
	// The function is not implemented as it is not relevant to the test case
	if src == "" || dst == "" {
		return fmt.Errorf("invalid source or destination path")
	}
	src, err := filepath.Abs(src)
	if err != nil {
		return err
	}

	err = os.MkdirAll(filepath.Dir(dst), 0755)
	if err != nil {
		return err
	}

	// Open the source file
	srcFile, err := os.Open(filepath.Join(src, fileName))
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Create the destination file
	dstFile, err := os.Create(filepath.Join(dst, fileName))
	if err != nil {
		return err
	}
	defer dstFile.Close()

	// Copy the contents of the source file to the destination file
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}

	return nil
}
