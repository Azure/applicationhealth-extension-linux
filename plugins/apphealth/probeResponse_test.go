package apphealth

import (
	"os"
	"testing"

	"github.com/Azure/applicationhealth-extension-linux/pkg/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	logFileDir    = "/tmp/logs"
	logFileName   = "log_test"
	mockLogger, _ = logging.NewRotatingSlogLogger(logFileDir, logFileName)
)

var tearDownFunc = func() {
	os.RemoveAll(logFileDir)
}

func TestDefaultHealthProbe(t *testing.T) {
	// Test default health probe and validating response
	settings := &AppHealthPluginSettings{}
	probe := NewHealthProbe(mockLogger, settings)
	expectedResponse := ProbeResponse{ApplicationHealthState: Healthy}
	probeResponse, err := probe.Evaluate(mockLogger)
	require.NoErrorf(t, err, "No Error expected, but got %v", err)
	assert.Equal(t, expectedResponse, probeResponse)
	tearDownFunc()
}
