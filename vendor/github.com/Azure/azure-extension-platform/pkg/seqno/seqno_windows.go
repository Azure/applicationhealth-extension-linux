// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.
package seqno

import (
	"fmt"
	"github.com/Azure/azure-extension-platform/pkg/extensionerrors"
	"golang.org/x/sys/windows/registry"
	"path"
	"strconv"
)

const (
	sequenceNumberKeyName = "SequenceNumber"
)

// getSequenceNumberInternal is the Windows specific logic for reading the current
// sequence number for the extension from the registry
func getSequenceNumberInternal(name, version string) (uint, error) {
	extensionKeyName := getExtensionKeyName(name, version)
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, extensionKeyName, registry.QUERY_VALUE)
	if err != nil {
		if err == registry.ErrNotExist {
			// This may happen if the extension isn't installed. Return a uniform error indicating this.
			return 0, extensionerrors.ErrNotFound
		}

		return 0, fmt.Errorf("VmExtension: Cannot open sequence registry key due to '%v'", err)
	}
	defer k.Close()

	buff := make([]byte, 32)

	_, valType, err := k.GetValue(sequenceNumberKeyName, buff)
	if err != nil {
		if err == registry.ErrNotExist {
			return 0, extensionerrors.ErrNotFound
		}
		return 0, fmt.Errorf("VmExtension: Cannot read sequence registry key due to '%v'", err)
	}
	var value uint
	switch valType {
	case registry.SZ, registry.EXPAND_SZ:
		stringVal, _, err := k.GetStringValue(sequenceNumberKeyName)
		if err != nil {
			return 0, err
		}
		val, err := strconv.ParseUint(stringVal, 10, 64)
		if err != nil {
			return 0, err
		}
		value = uint(val)
	case registry.DWORD:
		val, _, err := k.GetIntegerValue(sequenceNumberKeyName)
		if err != nil {
			return 0, err
		}
		value = uint(val)
	default:
		return 0, fmt.Errorf("value of registry key %s is of unexpected type", path.Join("HKEY_LOCAL_MACHINE", extensionKeyName, sequenceNumberKeyName))
	}

	return uint(value), nil
}

func getExtensionKeyName(name string, version string) (keyName string) {
	return fmt.Sprintf("Software\\Microsoft\\Windows Azure\\HandlerState\\%s_%s", name, version)
}

// setSequenceNumberInternal writes the sequence number for the extension to the registry
func setSequenceNumberInternal(extName, extVersion string, seqNo uint) error {
	extensionKeyName := getExtensionKeyName(extName, extVersion)
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, extensionKeyName, registry.WRITE)
	if err != nil {
		return fmt.Errorf("VmExtension: Cannot write sequence registry key due to '%v'", err)
	}
	defer k.Close()

	err = k.SetDWordValue(sequenceNumberKeyName, uint32(seqNo))
	return err
}
