package globalsearch

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEverythingCommandOutputIsBoundedAndCancelsOnce(t *testing.T) {
	called := 0
	output := searchCommandOutput{limit: 4, onLimit: func() { called++ }}
	n, err := output.Write([]byte("1234"))
	require.NoError(t, err)
	require.Equal(t, 4, n)
	require.False(t, output.exceeded)
	require.Zero(t, called)
	_, err = output.Write([]byte("5678"))
	require.NoError(t, err)
	_, err = output.Write([]byte("more"))
	require.NoError(t, err)
	require.Equal(t, "1234", string(output.data))
	require.True(t, output.exceeded)
	require.Equal(t, 1, called)
}
