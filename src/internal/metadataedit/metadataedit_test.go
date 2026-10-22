package metadataedit

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestPreviewDoesNotMutate(t *testing.T) {
	target := writeTestFile(t, "unicodé file.txt", "contents stay untouched")
	before, err := Inspect(context.Background(), target)
	if err != nil {
		t.Fatal(err)
	}

	modified := time.Date(2024, 4, 5, 6, 7, 8, 0, time.UTC)
	patch, err := BuildPatch(target, Changes{ModifiedTime: &modified})
	if err != nil {
		t.Fatal(err)
	}
	preview, err := Preview(context.Background(), patch)
	if err != nil {
		t.Fatal(err)
	}

	if !preview.Before.ModifiedTime.Equal(before.ModifiedTime) {
		t.Fatalf("preview before modified time = %v, want %v", preview.Before.ModifiedTime, before.ModifiedTime)
	}
	if !preview.After.ModifiedTime.Equal(modified) {
		t.Fatalf("preview after modified time = %v, want %v", preview.After.ModifiedTime, modified)
	}
	if !preview.After.AccessTime.Equal(before.AccessTime) {
		t.Fatalf(
			"unspecified access time changed in preview: got %v want %v",
			preview.After.AccessTime,
			before.AccessTime,
		)
	}
	after, err := Inspect(context.Background(), target)
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModifiedTime.Equal(before.ModifiedTime) || !after.AccessTime.Equal(before.AccessTime) {
		t.Fatalf("Preview mutated timestamps: before=%+v after=%+v", before, after)
	}
	assertContents(t, target, "contents stay untouched")
}

func TestApplyAndRestoreTimes(t *testing.T) {
	target := writeTestFile(t, "space and 日本語.txt", "payload")
	originalAccess := time.Date(2022, 1, 2, 3, 4, 5, 0, time.UTC)
	originalModified := time.Date(2022, 2, 3, 4, 5, 6, 0, time.UTC)
	if err := os.Chtimes(target, originalAccess, originalModified); err != nil {
		t.Fatal(err)
	}
	newAccess := time.Date(2025, 3, 4, 5, 6, 7, 0, time.UTC)
	newModified := time.Date(2025, 4, 5, 6, 7, 8, 0, time.UTC)
	patch, err := BuildPatch(target, Changes{AccessTime: &newAccess, ModifiedTime: &newModified})
	if err != nil {
		t.Fatal(err)
	}
	journalPath := filepath.Join(t.TempDir(), "backup journal.json")

	applied, err := Apply(context.Background(), patch, journalPath)
	if err != nil {
		t.Fatal(err)
	}
	if !applied.After.AccessTime.Equal(newAccess) || !applied.After.ModifiedTime.Equal(newModified) {
		t.Fatalf("applied metadata = %+v", applied.After)
	}
	assertContents(t, target, "payload")
	journalInfo, err := os.Stat(journalPath)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && journalInfo.Mode().Perm() != 0o600 {
		t.Fatalf("journal permissions = %04o, want 0600", journalInfo.Mode().Perm())
	}

	restored, err := Restore(context.Background(), journalPath)
	if err != nil {
		t.Fatal(err)
	}
	if !restored.After.AccessTime.Equal(originalAccess) || !restored.After.ModifiedTime.Equal(originalModified) {
		t.Fatalf("restored metadata = %+v", restored.After)
	}
	assertContents(t, target, "payload")

	_, err = Restore(context.Background(), journalPath)
	assertKind(t, err, KindAlreadyRestored)
}

func TestBuildPatchValidation(t *testing.T) {
	valid := time.Now()
	zero := time.Time{}
	outOfJSONRange := time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)
	badMode := fs.FileMode(0o1000)

	tests := []struct {
		name    string
		path    string
		changes Changes
	}{
		{name: "empty path", changes: Changes{ModifiedTime: &valid}},
		{name: "empty changes", path: "file"},
		{name: "zero modified", path: "file", changes: Changes{ModifiedTime: &zero}},
		{name: "zero access", path: "file", changes: Changes{AccessTime: &zero}},
		{name: "unencodable modified", path: "file", changes: Changes{ModifiedTime: &outOfJSONRange}},
		{name: "non-permission mode bits", path: "file", changes: Changes{Permissions: &badMode}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := BuildPatch(tt.path, tt.changes)
			assertKind(t, err, KindInvalidInput)
		})
	}
}

func TestCancellationStopsBeforeMutation(t *testing.T) {
	target := writeTestFile(t, "cancel.txt", "unchanged")
	before, err := Inspect(context.Background(), target)
	if err != nil {
		t.Fatal(err)
	}
	modified := before.ModifiedTime.Add(-time.Hour)
	patch, err := BuildPatch(target, Changes{ModifiedTime: &modified})
	if err != nil {
		t.Fatal(err)
	}
	journalPath := filepath.Join(t.TempDir(), "should-not-exist.json")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = Apply(ctx, patch, journalPath)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Apply error = %v, want context.Canceled", err)
	}
	assertKind(t, err, KindCanceled)
	_, statErr := os.Stat(journalPath)
	if !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("journal unexpectedly created: %v", statErr)
	}
	after, err := Inspect(context.Background(), target)
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModifiedTime.Equal(before.ModifiedTime) {
		t.Fatalf("canceled Apply changed modified time: got %v want %v", after.ModifiedTime, before.ModifiedTime)
	}
}

func TestApplyRequiresAndPreservesJournal(t *testing.T) {
	target := writeTestFile(t, "target.txt", "data")
	modified := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	patch, err := BuildPatch(target, Changes{ModifiedTime: &modified})
	if err != nil {
		t.Fatal(err)
	}

	_, err = Apply(context.Background(), patch, "")
	assertKind(t, err, KindInvalidInput)

	journalPath := filepath.Join(t.TempDir(), "existing.json")
	const existing = "do not overwrite this backup"
	writeErr := os.WriteFile(journalPath, []byte(existing), 0o600)
	if writeErr != nil {
		t.Fatal(writeErr)
	}
	before, err := Inspect(context.Background(), target)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Apply(context.Background(), patch, journalPath)
	assertKind(t, err, KindJournalExists)
	got, err := os.ReadFile(journalPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != existing {
		t.Fatalf("existing journal was overwritten: %q", got)
	}
	after, err := Inspect(context.Background(), target)
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModifiedTime.Equal(before.ModifiedTime) {
		t.Fatal("target mutated when journal already existed")
	}
}

func TestSymlinkRefused(t *testing.T) {
	target := writeTestFile(t, "real.txt", "data")
	link := filepath.Join(t.TempDir(), "link.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	modified := time.Now().Add(-time.Hour)
	patch, err := BuildPatch(link, Changes{ModifiedTime: &modified})
	if err != nil {
		t.Fatal(err)
	}
	_, err = Preview(context.Background(), patch)
	assertKind(t, err, KindUnsafeFile)
	journalPath := filepath.Join(t.TempDir(), "journal.json")
	_, err = Apply(context.Background(), patch, journalPath)
	assertKind(t, err, KindUnsafeFile)
	if _, err := os.Stat(journalPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("journal created for symlink: %v", err)
	}
}

func TestRestoreRejectsCorruptAndUnknownJournal(t *testing.T) {
	dir := t.TempDir()
	corrupt := filepath.Join(dir, "corrupt.json")
	if err := os.WriteFile(corrupt, []byte("not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Restore(context.Background(), corrupt)
	assertKind(t, err, KindCorruptJournal)

	unknown := filepath.Join(dir, "unknown.json")
	writeErr := os.WriteFile(unknown, []byte(`{"version":99}`), 0o600)
	if writeErr != nil {
		t.Fatal(writeErr)
	}
	_, err = Restore(context.Background(), unknown)
	assertKind(t, err, KindJournalVersion)
}

func TestRollbackEarlierFieldWhenLaterFieldFails(t *testing.T) {
	before := Metadata{
		Path:         "target",
		AccessTime:   time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
		ModifiedTime: time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC),
		Permissions:  0o644,
	}
	after := before
	after.Permissions = 0o600
	after.ModifiedTime = before.ModifiedTime.Add(time.Hour)
	var modes []fs.FileMode
	ops := fileOps{
		chmod: func(_ string, mode fs.FileMode) error {
			modes = append(modes, mode)
			return nil
		},
		chtimes: func(string, time.Time, time.Time) error {
			return errors.New("injected timestamp failure")
		},
	}
	err := applyChanges(
		context.Background(),
		"target",
		before,
		after,
		changeSet{permissions: true, times: true},
		ops,
	)
	if err == nil || !strings.Contains(err.Error(), "injected timestamp failure") {
		t.Fatalf("applyChanges error = %v", err)
	}
	if len(modes) != 2 || modes[0] != 0o600 || modes[1] != 0o644 {
		t.Fatalf("chmod calls = %v, want [0600 0644]", modes)
	}
}

func TestApplyChangesRollsBackOnCancellation(t *testing.T) {
	before := Metadata{
		Path:         "target",
		AccessTime:   time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
		ModifiedTime: time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC),
		Permissions:  0o644,
	}
	after := before
	after.Permissions = 0o600
	after.ModifiedTime = after.ModifiedTime.Add(time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	var modes []fs.FileMode
	ops := fileOps{
		chmod: func(_ string, mode fs.FileMode) error {
			modes = append(modes, mode)
			if len(modes) == 1 {
				cancel()
			}
			return nil
		},
		chtimes: func(string, time.Time, time.Time) error {
			t.Fatal("timestamp mutation must not run after cancellation")
			return nil
		},
	}
	err := applyChanges(ctx, "target", before, after, changeSet{permissions: true, times: true}, ops)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("applyChanges error = %v, want context.Canceled", err)
	}
	if len(modes) != 2 || modes[0] != 0o600 || modes[1] != 0o644 {
		t.Fatalf("chmod calls = %v, want [0600 0644]", modes)
	}
}

func TestApplyChangesRollsBackTimestampsOnCancellation(t *testing.T) {
	before := Metadata{
		AccessTime:   time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
		ModifiedTime: time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC),
	}
	after := before
	after.ModifiedTime = after.ModifiedTime.Add(time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	type call struct{ access, modified time.Time }
	var calls []call
	ops := fileOps{
		chmod: os.Chmod,
		chtimes: func(_ string, access, modified time.Time) error {
			calls = append(calls, call{access: access, modified: modified})
			if len(calls) == 1 {
				cancel()
			}
			return nil
		},
	}
	err := applyChanges(ctx, "target", before, after, changeSet{times: true}, ops)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("applyChanges error = %v, want context.Canceled", err)
	}
	if len(calls) != 2 ||
		!calls[0].modified.Equal(after.ModifiedTime) ||
		!calls[1].modified.Equal(before.ModifiedTime) {
		t.Fatalf("timestamp calls = %+v, want apply then rollback", calls)
	}
}

func TestRollbackReportsItsOwnFailure(t *testing.T) {
	want := Metadata{
		AccessTime:   time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
		ModifiedTime: time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC),
		Permissions:  0o640,
	}
	ops := fileOps{
		chmod: func(string, fs.FileMode) error { return errors.New("chmod rollback failed") },
		chtimes: func(string, time.Time, time.Time) error {
			return errors.New("timestamp rollback failed")
		},
	}
	err := rollbackChanges("target", want, changeSet{permissions: true, times: true}, ops)
	if err == nil ||
		!strings.Contains(err.Error(), "chmod rollback failed") ||
		!strings.Contains(err.Error(), "timestamp rollback failed") {
		t.Fatalf("rollbackChanges error = %v", err)
	}
}

func TestInvalidPatchAndJournalPaths(t *testing.T) {
	_, err := Preview(context.Background(), Patch{})
	assertKind(t, err, KindInvalidInput)

	target := writeTestFile(t, "same-path.txt", "preserved")
	modified := time.Now().Add(-time.Hour)
	patch, err := BuildPatch(target, Changes{ModifiedTime: &modified})
	if err != nil {
		t.Fatal(err)
	}
	_, err = Apply(context.Background(), patch, target)
	assertKind(t, err, KindInvalidInput)
	assertContents(t, target, "preserved")

	_, err = Restore(context.Background(), filepath.Join(t.TempDir(), "missing.json"))
	assertKind(t, err, KindJournalIO)
}

func TestTypedErrorFormattingAndNilContext(t *testing.T) {
	target := writeTestFile(t, "nil-context.txt", "data")
	// Passing nil is intentional here because the public API promises to reject it.
	_, err := Inspect(nil, target) //nolint:staticcheck // Exercise the documented nil-context error path.
	assertKind(t, err, KindInvalidInput)
	var metadataErr *Error
	if !errors.As(err, &metadataErr) || metadataErr.Op != "inspect" || metadataErr.Path != target {
		t.Fatalf("typed error omitted operation or path: %v", err)
	}
	var nilError *Error
	if got := nilError.Error(); got != "<nil>" {
		t.Fatalf("nil Error string = %q, want <nil>", got)
	}
}

func TestJournalStructuralValidation(t *testing.T) {
	target := writeTestFile(t, "journal-target.txt", "data")
	metadata, err := Inspect(context.Background(), target)
	if err != nil {
		t.Fatal(err)
	}
	valid := journalFromPreview(PreviewResult{Before: metadata, After: metadata})

	tests := []struct {
		name string
		data func() []byte
	}{
		{
			name: "unknown field",
			data: func() []byte { return []byte(`{"version":1,"unexpected":true}`) },
		},
		{
			name: "multiple values",
			data: func() []byte {
				data, marshalErr := json.Marshal(valid)
				if marshalErr != nil {
					t.Fatal(marshalErr)
				}
				return append(data, []byte("\n{}")...)
			},
		},
		{
			name: "invalid state",
			data: func() []byte {
				value := valid
				value.State = "mystery"
				return mustMarshalJournal(t, value)
			},
		},
		{
			name: "relative target",
			data: func() []byte {
				value := valid
				value.Target = "relative.txt"
				return mustMarshalJournal(t, value)
			},
		},
		{
			name: "zero timestamp",
			data: func() []byte {
				value := valid
				value.Before.AccessTime = time.Time{}
				return mustMarshalJournal(t, value)
			},
		},
		{
			name: "invalid permissions",
			data: func() []byte {
				value := valid
				value.After.Permissions = 0o1000
				return mustMarshalJournal(t, value)
			},
		},
		{
			name: "oversized",
			data: func() []byte { return make([]byte, maxJournalSize+1) },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "journal.json")
			if err := os.WriteFile(path, tt.data(), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := Restore(context.Background(), path)
			assertKind(t, err, KindCorruptJournal)
		})
	}
}

func mustMarshalJournal(t *testing.T, value journal) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func writeTestFile(t *testing.T, name, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertContents(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("contents = %q, want %q", got, want)
	}
}

func assertKind(t *testing.T, err error, want Kind) {
	t.Helper()
	var metadataErr *Error
	if !errors.As(err, &metadataErr) {
		t.Fatalf("error = %v (%T), want *Error kind %q", err, err, want)
	}
	if metadataErr.Kind != want {
		t.Fatalf("error kind = %q, want %q (error: %v)", metadataErr.Kind, want, err)
	}
}
