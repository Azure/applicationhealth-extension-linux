package cmdhandler

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseCmd(t *testing.T) {
	tests := []struct {
		args     []string
		expected CommandKey
		errMsg   string
	}{
		{[]string{"bin/applicationhealth-shim"}, "", "Incorrect usage"},
		{[]string{"bin/applicationhealth-shim", "noop"}, "", fmt.Sprintf("Incorrect command: %q\n", "noop")},
		{[]string{"bin/applicationhealth-shim", "install"}, InstallKey, ""},
		{[]string{"bin/applicationhealth-shim", "uninstall"}, UninstallKey, ""},
		{[]string{"bin/applicationhealth-shim", "enable"}, EnableKey, ""},
		{[]string{"bin/applicationhealth-shim", "disable"}, DisableKey, ""},
		{[]string{"bin/applicationhealth-shim", "update"}, UpdateKey, ""},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("Args: %v", test.args), func(t *testing.T) {
			// Save original os.Args and restore it after the test
			originalArgs := os.Args
			defer func() { os.Args = originalArgs }()

			os.Args = test.args
			cmdKey, err := ParseCmd()

			if test.errMsg != "" {
				require.EqualError(t, err, test.errMsg)
				require.Equal(t, "", cmdKey.String())
			} else {
				require.NoError(t, err)
				require.Equal(t, test.expected, cmdKey)
			}
		})
	}
}
