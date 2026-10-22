package savedsearch

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStoreLifecycleAndDeterministicOrdering(t *testing.T) {
	store := New(filepath.Join(t.TempDir(), "state", "saved-searches.json"))
	list, err := store.List()
	require.NoError(t, err)
	assert.Empty(t, list)

	require.NoError(t, store.Save("Reports", `type:file ext:pdf content:"revenue"`))
	require.NoError(t, store.Save("archives", "ext:zip"))
	require.NoError(t, store.Save("REPORTS", "ext:xlsx"))
	list, err = store.List()
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, "archives", list[0].Name)
	assert.Equal(t, "REPORTS", list[1].Name)

	search, err := store.Get("reports")
	require.NoError(t, err)
	assert.Equal(t, "ext:xlsx", search.Query)
	require.NoError(t, store.Delete("Reports"))
	_, err = store.Get("reports")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestStoreValidationAndCorruption(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "saved.json")
	store := New(path)
	for _, name := range []string{"", " leading", "bad/name", "line\nbreak"} {
		require.ErrorIs(t, store.Save(name, "query"), ErrInvalidName)
	}
	require.ErrorIs(t, store.Save("valid", "  "), ErrInvalidQuery)

	require.NoError(t, os.WriteFile(path, []byte(`{"version":99,"searches":[]}`), 0o600))
	_, err := store.List()
	require.ErrorIs(t, err, ErrVersion)
	require.NoError(t, os.WriteFile(path, []byte(`not json`), 0o600))
	_, err = store.List()
	assert.ErrorIs(t, err, ErrCorrupt)
}

func TestDeleteMissingDoesNotCreateState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "saved.json")
	store := New(path)
	err := store.Delete("none")
	require.ErrorIs(t, err, ErrNotFound)
	_, statErr := os.Stat(path)
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}
