package metadata

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBinaryFormatGateRejectsNonExecutablePreviewInputs(t *testing.T) {
	for _, data := range [][]byte{
		nil, []byte("a"), []byte("a plain text document"), make([]byte, 1024),
		{0, 0, 0, 24, 'f', 't', 'y', 'p', 'm', 'p', '4', '2'},
	} {
		path := filepath.Join(t.TempDir(), "input")
		require.NoError(t, os.WriteFile(path, data, 0o600))
		assert.Equal(t, formatUnknown, detectBinaryFormat(path))
		_, err := GetBinaryArchitecture(path)
		require.ErrorIs(t, err, errNotBinary)
	}
}

func TestBinaryFormatGateRecognizesSupportedFormats(t *testing.T) {
	for _, test := range []struct {
		data []byte
		want binaryFormat
	}{
		{[]byte{0x7f, 'E', 'L', 'F'}, formatELF},
		{[]byte{'M', 'Z', 0, 0}, formatPE},
		{[]byte{0x64, 0x86, 0, 0}, formatPE},
		{[]byte{0xfe, 0xed, 0xfa, 0xce}, formatMachO},
		{[]byte{0xcf, 0xfa, 0xed, 0xfe}, formatMachO},
		{[]byte{0xca, 0xfe, 0xba, 0xbe}, formatMachO},
	} {
		path := filepath.Join(t.TempDir(), "binary")
		require.NoError(t, os.WriteFile(path, test.data, 0o600))
		assert.Equal(t, test.want, detectBinaryFormat(path))
	}
}
