// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.
package utils

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
)

// agentDir is where the agent is located, a subdirectory of which we use as the data directory
const agentDir = "/var/lib/waagent"

func GetDataFolder(name string, version string) string {
	return path.Join(agentDir, name)
}

// Try clear files whose file names match with a regular expression except the filename passed in exceptFileName argument.
// If deleteFiles is true, files will be deleted, else they will be emptied without deleting.
func TryClearRegexMatchingFilesExcept(directory string, regexFileNamePattern string,
	exceptFileName string, deleteFiles bool) error {

	if regexFileNamePattern == "" {
		return errors.New("Empty regexFileNamePattern argument.")
	}

	// Check if the directory exists
	directoryFDRef, err := os.Open(directory)
	if err != nil {
		return err
	}

	regex, err := regexp.Compile(regexFileNamePattern)
	if err != nil {
		return err
	}

	dirEntries, err := directoryFDRef.ReadDir(0)
	if err == nil {
		for _, dirEntry := range dirEntries {
			fileName := dirEntry.Name()

			if fileName != exceptFileName && regex.MatchString(fileName) {
				fullFilePath := filepath.Join(directory, fileName)
				if deleteFiles {
					os.Remove(fullFilePath)
				} else {
					os.Truncate(fullFilePath, 0) // Calling create on existing file truncates file
				}
			}
		}
		return nil
	}

	return err
}

// Try delete all directories in parentDirectory excepth directory by name 'exceptDirectoryName'
func TryDeleteDirectoriesExcept(parentDirectory string, exceptDirectoryName string) error {
	// Check if the directory exists
	directoryFDRef, err := os.Open(parentDirectory)
	if err != nil {
		return err
	}

	dirEntries, err := directoryFDRef.ReadDir(0)
	if err == nil && dirEntries != nil {
		for _, dirEntry := range dirEntries {
			entryName := dirEntry.Name()
			if dirEntry.IsDir() && entryName != exceptDirectoryName {
				fullDirectoryPath := filepath.Join(parentDirectory, entryName)
				os.RemoveAll(fullDirectoryPath)
			}
		}
		return nil
	}
	return err
}

//Try empty runtime settings files for an extension except last, delete scripts except last.
// runtimeSettingsRegexFormatWithAnyExtName - regex identifying all settings files- example. "\\d+.settings", "RunCommandName.\\d+.settings"
// runtimeSettingsLastSeqNumFormatWithAnyExtName -  example. "%s.settings", "RunCommandName.%s.settings"
func TryClearExtensionScriptsDirectoriesAndSettingsFilesExceptMostRecent(scriptsDirectory string,
	runtimeSettingsDirectory string,
	extensionName string,
	mostRecentSequenceNumberFinished uint64,
	runtimeSettingsRegexFormatWithAnyExtName string,
	runtimeSettingsLastSeqNumFormatWithAnyExtName string) error {

	recentSeqNumberString := strconv.FormatUint(mostRecentSequenceNumberFinished, 10)

	// Delete scripts belonging to previous sequence numbers.
	err := TryDeleteDirectoriesExcept(filepath.Join(scriptsDirectory, extensionName), recentSeqNumberString)
	if err != nil {
		return err
	}

	mostRecentRuntimeSetting := fmt.Sprintf(runtimeSettingsLastSeqNumFormatWithAnyExtName, mostRecentSequenceNumberFinished)

	// Empty Runtimesettings files belonging to previous sequence numbers.
	err = TryClearRegexMatchingFilesExcept(runtimeSettingsDirectory,
		runtimeSettingsRegexFormatWithAnyExtName,
		mostRecentRuntimeSetting,
		false)
	if err != nil {
		return err
	}
	return nil
}
