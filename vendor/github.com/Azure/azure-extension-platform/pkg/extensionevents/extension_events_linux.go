// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.
package extensionevents

import (
	"fmt"

	"golang.org/x/sys/unix"
)

func getThreadID() string {
	return fmt.Sprintf("%d", unix.Gettid())
}
