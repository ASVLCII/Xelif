package globalsearch

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEverythingCSVNumericAttributesAndUnicode(t *testing.T) {
	t.Parallel()

	input := "\"C:\\日本語-é.txt\",32\r\n\"C:\\Documents\",16\r\n\"C:\\Hidden Folder\",18\r\n\"C:\\normal\",0\r\n"
	results, err := parseEverythingCSV(strings.NewReader(input), 10)
	require.NoError(t, err)
	require.Equal(t, []Result{
		{Path: `C:\日本語-é.txt`},
		{Path: `C:\Documents`, Directory: true},
		{Path: `C:\Hidden Folder`, Directory: true},
		{Path: `C:\normal`},
	}, results)
}

func TestEverythingCSVRejectsMalformedResults(t *testing.T) {
	t.Parallel()

	for _, input := range []string{
		"path\n", "path,\n", "path,4294967296\n", "path,-1\n",
		"path,garbage\n", "path,16,unexpected\n", "bad\x00path,32\n", "bad\xffpath,32\n",
		"path,INVALID\n", "path,D\n", "path,A\n", "path,AD\n",
	} {
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			results, err := parseEverythingCSV(strings.NewReader(input), 10)
			require.Error(t, err)
			require.Empty(t, results)
		})
	}
}
