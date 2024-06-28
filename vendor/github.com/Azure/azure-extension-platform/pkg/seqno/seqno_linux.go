// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.
package seqno

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	"github.com/Azure/azure-extension-platform/pkg/constants"
	"github.com/Azure/azure-extension-platform/pkg/extensionerrors"
	"github.com/Azure/azure-extension-platform/pkg/utils"
)

var mostRecentSequenceFileName = "mrseq"

// sequence number for the extension from the registry
func getSequenceNumberInternal(name, version string) (uint, error) {
	mrseqPath, err := getMrseqFilePath()
	if err != nil {
		return 0, err
	}
	mrseqStr, err := ioutil.ReadFile(mrseqPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, extensionerrors.ErrNoMrseqFile
		}
		return 0, fmt.Errorf("failed to read mrseq file : %s", err)
	}

	seqNum, err := strconv.Atoi(string(mrseqStr))
	if err != nil {
		return 0, err
	}
	return uint(seqNum), nil

}

func setSequenceNumberInternal(extName, extVersion string, seqNo uint) error {
	b := []byte(fmt.Sprintf("%v", seqNo))

	mrseqPath, err := getMrseqFilePath()
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(mrseqPath, b, constants.FilePermissions_UserOnly_ReadWrite)
	if err != nil {
		return fmt.Errorf("could not write sequence number file %s, error: %v", mostRecentSequenceFileName, err)
	}
	return nil
}

func getMrseqFilePath() (string, error) {
	// mrseq file path must always be present in the same directory as the extension executable
	currentDir, err := utils.GetCurrentProcessWorkingDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(currentDir, mostRecentSequenceFileName), nil
}
