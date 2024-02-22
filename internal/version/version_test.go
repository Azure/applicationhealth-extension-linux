package version

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVersionString(t *testing.T) {
	defer resetStrings()

	Version = "1.0.0"
	GitState = "dirty"
	GitCommit = "03669cef"
	require.Equal(t, "v1.0.0/git@03669cef-dirty", VersionString())
}

func TestDetailedVersionString(t *testing.T) {
	defer resetStrings()

	Version = "1.0.0"
	GitState = "dirty"
	GitCommit = "03669cef"
	BuildDate = "DATE"
	goVersion := runtime.Version()
	require.Equal(t, "v1.0.0 git:03669cef-dirty build:DATE "+goVersion, DetailedVersionString())
}

func resetStrings() { Version, GitCommit, BuildDate, GitState = "", "", "", "" }

func Test_ExtensionManifestVersion(t *testing.T) {

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
	v := GetExtensionVersionFromBuild()
	require.Empty(t, v)

	currentManifest, err := getExtensionManifest("../../misc/manifest.xml")
	require.Nil(t, err)
	require.Equal(t, expectedManifest.Version, currentManifest.Version)
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
