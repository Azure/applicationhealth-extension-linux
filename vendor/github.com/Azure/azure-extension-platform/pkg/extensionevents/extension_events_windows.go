// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.
package extensionevents

import (
	"fmt"
	"golang.org/x/sys/windows"
)

func getThreadID() string {
	return fmt.Sprintf("%v", windows.GetCurrentThreadId())
}
