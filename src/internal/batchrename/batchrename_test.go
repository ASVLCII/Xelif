package batchrename

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeFiles(t *testing.T, directory string, names ...string) []string {
	t.Helper()
	paths := make([]string, 0, len(names))
	for _, name := range names {
		path := filepath.Join(directory, name)
		require.NoError(t, os.WriteFile(path, []byte(name), 0o600))
		paths = append(paths, path)
	}
	return paths
}

func noReplaceRename(source, destination string) error {
	if _, err := os.Lstat(destination); err == nil {
		return os.ErrExist
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Rename(source, destination)
}

func TestNewPlanAppliesLiteralRuleDeterministically(t *testing.T) {
	directory := t.TempDir()
	paths := writeFiles(t, directory, "beta draft.txt", "alpha draft.txt")

	plan, err := NewPlan(paths, Rule{Find: " draft", Replace: "", Prefix: "final-"})
	require.NoError(t, err)
	require.Len(t, plan.Entries, 2)
	assert.Equal(t, filepath.Join(directory, "alpha draft.txt"), plan.Entries[0].Source)
	assert.Equal(t, filepath.Join(directory, "final-alpha.txt"), plan.Entries[0].Destination)
	assert.Equal(t, filepath.Join(directory, "final-beta.txt"), plan.Entries[1].Destination)

	require.NoError(t, Apply(context.Background(), plan, noReplaceRename))
	for _, name := range []string{"final-alpha.txt", "final-beta.txt"} {
		_, err := os.Stat(filepath.Join(directory, name))
		assert.NoError(t, err)
	}
}

func TestNewPlanRejectsUnsafeOrAmbiguousPlans(t *testing.T) {
	directory := t.TempDir()
	paths := writeFiles(t, directory, "a.txt", "aa.txt", "taken.txt")

	tests := []struct {
		name  string
		paths []string
		rule  Rule
		want  error
	}{
		{name: "no transform", paths: paths[:1], rule: Rule{}, want: ErrNoTransformation},
		{name: "no changes", paths: paths[:1], rule: Rule{Find: "missing", Replace: "x"}, want: ErrNoChanges},
		{
			name:  "duplicate source",
			paths: []string{paths[0], paths[0]},
			rule:  Rule{Prefix: "x"},
			want:  ErrDuplicateSource,
		},
		{
			name:  "existing destination",
			paths: paths[:1],
			rule:  Rule{Find: "a", Replace: "taken"},
			want:  ErrDestinationExists,
		},
		{
			name:  "destination conflict",
			paths: paths[:2],
			rule:  Rule{Find: "a", Replace: ""},
			want:  ErrDestinationConflict,
		},
		{
			name:  "overlapping source",
			paths: paths[:2],
			rule:  Rule{Find: "a", Replace: "aa"},
			want:  ErrSourceDestinationOverlap,
		},
		{name: "path separator", paths: paths[:1], rule: Rule{Prefix: "../"}, want: nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewPlan(test.paths, test.rule)
			if test.name == "path separator" {
				assert.Error(t, err)
				return
			}
			assert.ErrorIs(t, err, test.want)
		})
	}
}

func TestApplyRollsBackCompletedRenames(t *testing.T) {
	directory := t.TempDir()
	paths := writeFiles(t, directory, "a.txt", "b.txt")
	plan, err := NewPlan(paths, Rule{Prefix: "new-"})
	require.NoError(t, err)
	failed := errors.New("injected failure")
	calls := 0
	rename := func(source, destination string) error {
		calls++
		if calls == 2 {
			return failed
		}
		return noReplaceRename(source, destination)
	}

	err = Apply(context.Background(), plan, rename)
	require.ErrorIs(t, err, failed)
	for _, path := range paths {
		_, statErr := os.Stat(path)
		assert.NoError(t, statErr)
	}
}

func TestApplyHonorsCancellation(t *testing.T) {
	directory := t.TempDir()
	paths := writeFiles(t, directory, "a.txt")
	plan, err := NewPlan(paths, Rule{Prefix: "new-"})
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = Apply(ctx, plan, noReplaceRename)
	require.ErrorIs(t, err, context.Canceled)
	_, statErr := os.Stat(paths[0])
	assert.NoError(t, statErr)
}
