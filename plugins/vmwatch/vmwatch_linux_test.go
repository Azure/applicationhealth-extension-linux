package vmwatch

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractVersion(t *testing.T) {
	v := extractVersion("systemd 123")
	require.Equal(t, 123, v)
	v = extractVersion(`someline
systemd 123
some other line`)
	require.Equal(t, 123, v)
	v = extractVersion(`someline
systemd abc
some other line`)
	require.Equal(t, 0, v)
	v = extractVersion("junk")
	require.Equal(t, 0, v)
}
