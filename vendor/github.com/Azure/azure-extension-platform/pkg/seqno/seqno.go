// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.
package seqno

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/azure-extension-platform/pkg/extensionerrors"
	"github.com/Azure/azure-extension-platform/pkg/logging"
)

const configSequenceNumber = "ConfigSequenceNumber"

type ISequenceNumberRetriever interface {
	GetSequenceNumber(name, version string) (uint, error)
}

type ProdSequenceNumberRetriever struct {
}

func (*ProdSequenceNumberRetriever) GetSequenceNumber(name, version string) (uint, error) {
	return getSequenceNumberInternal(name, version)
}

// GetCurrentSequenceNumber returns the current sequence number the extension is using
func GetCurrentSequenceNumber(el logging.ILogger, retriever ISequenceNumberRetriever, name, version string) (sn uint, _ error) {
	sequenceNumber, err := retriever.GetSequenceNumber(name, version)
	if err == extensionerrors.ErrNotFound || err == extensionerrors.ErrNoMrseqFile {
		// If we can't find the sequence number, then it's possible that the extension
		// hasn't been installed yet. Go back to 0.
		el.Info("Couldn't find current sequence number, likely first execution of the extension, returning sequence number 0")
		return 0, nil
	}

	return sequenceNumber, err
}

func SetSequenceNumber(extName, extVersion string, seqNo uint) error {
	return setSequenceNumberInternal(extName, extVersion, seqNo)
}

// findSeqnum finds the most recently used file under the config folder
// Note that this is different than just choosing the highest number, which may be incorrect
func FindSeqNum(el logging.ILogger, configFolder string) (uint, error) {
	// try getting the sequence number from the environment first
	seqNoString := os.Getenv(configSequenceNumber)
	if seqNoString == "" {
		el.Info("could not read environment variable %s for getting sequence number", configSequenceNumber)
	} else {
		seqNo, err := strconv.ParseUint(seqNoString, 10, 64)
		if err != nil {
			el.Info("could not read sequence number string %s into unsigned integer", seqNoString)
		} else {
			el.Info("using sequence number %d from environment variable %s", seqNo, configSequenceNumber)
			return uint(seqNo), nil
		}
	}

	g, err := filepath.Glob(configFolder + "/*.settings")
	if err != nil {
		return 0, err
	}

	// Start by finding the file with the latest time
	files, err := ioutil.ReadDir(configFolder)
	if err != nil {
		return 0, err
	}

	var modTime time.Time
	var names []string
	for _, fi := range files {
		if fi.Mode().IsRegular() && strings.HasSuffix(fi.Name(), ".settings") {
			if !fi.ModTime().Before(modTime) {
				if fi.ModTime().After(modTime) {
					modTime = fi.ModTime()
					names = names[:0]
				}

				// Unlikely - but just in case the files have the same time
				names = append(names, fi.Name())
			}
		}
	}

	if len(names) == 0 {
		el.Error("Cannot find the seqNo from %s. Not enough files", configFolder)
		return 0, extensionerrors.ErrNoSettingsFiles
	} else if len(names) == 1 {
		i, err := strconv.Atoi(strings.Replace(names[0], ".settings", "", 1))
		if err != nil {
			el.Error("Can't parse int from filename: %s", names[0])
			return 0, extensionerrors.ErrInvalidSettingsFileName
		}

		return uint(i), nil
	} else {
		// For some reason we have two or more files with the same time stamp.
		// Revert to choosing the highest number.
		seqs := make([]int, len(g))
		for _, v := range g {
			f := filepath.Base(v)
			i, err := strconv.Atoi(strings.Replace(f, ".settings", "", 1))
			if err != nil {
				el.Error("Can't parse int from filename: %s", f)
				return 0, extensionerrors.ErrInvalidSettingsFileName
			}
			seqs = append(seqs, i)
		}

		sort.Sort(sort.Reverse(sort.IntSlice(seqs)))
		return uint(seqs[0]), nil
	}
}
