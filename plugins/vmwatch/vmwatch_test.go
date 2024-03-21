package vmwatch

import (
	"testing"

	s "github.com/Azure/applicationhealth-extension-linux/pkg/status"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestGetStatusTypeReturnsCorrectValue(t *testing.T) {
	status := Failed
	require.Equal(t, s.StatusError, status.GetStatusType())
	status = Disabled
	require.Equal(t, s.StatusWarning, status.GetStatusType())
	status = NotRunning
	require.Equal(t, s.StatusSuccess, status.GetStatusType())
	status = Running
	require.Equal(t, s.StatusSuccess, status.GetStatusType())
}

func TestGetMessageCorrectValue(t *testing.T) {
	res := VMWatchResult{Status: Disabled}
	require.Equal(t, "VMWatch is disabled", res.GetMessage())
	res = VMWatchResult{Status: Failed}
	require.Equal(t, "VMWatch failed: <nil>", res.GetMessage())
	res = VMWatchResult{Status: Failed, Error: errors.New("this is an error")}
	require.Equal(t, "VMWatch failed: this is an error", res.GetMessage())
	res = VMWatchResult{Status: NotRunning}
	require.Equal(t, "VMWatch is not running", res.GetMessage())
	res = VMWatchResult{Status: Running}
	require.Equal(t, "VMWatch is running", res.GetMessage())
}
