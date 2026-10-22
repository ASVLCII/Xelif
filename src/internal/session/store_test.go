package session

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

func TestStoreLastRoundTripOverwriteAndCopies(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := NewStore(filepath.Join(dir, "state", "last.json"), filepath.Join(dir, "workspaces"))
	first := testState("/first")
	if err := store.SaveLast(first); err != nil {
		t.Fatalf("SaveLast() error = %v", err)
	}

	// Saving must not retain aliases into caller-owned slices.
	first.Panels[0].Tabs[0].BackHistory[0] = "/mutated"
	got, err := store.LoadLast()
	if err != nil {
		t.Fatalf("LoadLast() error = %v", err)
	}
	want := testState("/first")
	if !statesEqual(got, want) {
		t.Fatalf("LoadLast() = %#v, want %#v", got, want)
	}

	// Loaded values must not alias later loads.
	got.Panels[0].Tabs[0].ForwardHistory[0] = "/mutated"
	again, err := store.LoadLast()
	if err != nil {
		t.Fatalf("second LoadLast() error = %v", err)
	}
	if !statesEqual(again, want) {
		t.Fatalf("second LoadLast() = %#v, want %#v", again, want)
	}

	second := testState("/second")
	if saveErr := store.SaveLast(second); saveErr != nil {
		t.Fatalf("overwrite SaveLast() error = %v", saveErr)
	}
	got, err = store.LoadLast()
	if err != nil {
		t.Fatalf("LoadLast() after overwrite error = %v", err)
	}
	if !statesEqual(got, second) {
		t.Fatalf("LoadLast() after overwrite = %#v, want %#v", got, second)
	}
}

func TestStoreWorkspaceLifecycleAndOrdering(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := NewStore(filepath.Join(dir, "last.json"), filepath.Join(dir, "workspaces"))
	for _, name := range []string{"zeta", "Alpha", "middle"} {
		if err := store.SaveWorkspace(name, testState("/"+name)); err != nil {
			t.Fatalf("SaveWorkspace(%q) error = %v", name, err)
		}
	}

	names, err := store.ListWorkspaces()
	if err != nil {
		t.Fatalf("ListWorkspaces() error = %v", err)
	}
	wantNames := []string{"Alpha", "middle", "zeta"}
	if !slices.Equal(names, wantNames) {
		t.Fatalf("ListWorkspaces() = %q, want %q", names, wantNames)
	}

	got, err := store.LoadWorkspace("middle")
	if err != nil {
		t.Fatalf("LoadWorkspace() error = %v", err)
	}
	if want := testState("/middle"); !statesEqual(got, want) {
		t.Fatalf("LoadWorkspace() = %#v, want %#v", got, want)
	}

	if saveErr := store.SaveWorkspace("middle", testState("/overwritten")); saveErr != nil {
		t.Fatalf("overwrite SaveWorkspace() error = %v", saveErr)
	}
	got, err = store.LoadWorkspace("middle")
	if err != nil || got.Panels[0].Tabs[0].Location != "/overwritten" {
		t.Fatalf("LoadWorkspace() after overwrite = (%#v, %v)", got, err)
	}

	if err := store.DeleteWorkspace("middle"); err != nil {
		t.Fatalf("DeleteWorkspace() error = %v", err)
	}
	if _, err := store.LoadWorkspace("middle"); !errors.Is(err, ErrNotFound) || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("LoadWorkspace() after delete error = %v, want ErrNotFound and os.ErrNotExist", err)
	}
}

func TestListWorkspacesMissingDirectoryIsEmpty(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := NewStore(filepath.Join(dir, "last.json"), filepath.Join(dir, "missing"))
	got, err := store.ListWorkspaces()
	if err != nil {
		t.Fatalf("ListWorkspaces() error = %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("ListWorkspaces() = %#v, want non-nil empty slice", got)
	}
}

func TestLoadErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		wantErr error
	}{
		{name: "corrupt json", content: `{nope`, wantErr: ErrCorrupt},
		{name: "trailing json", content: `{"version":1} {}`, wantErr: ErrCorrupt},
		{name: "missing version", content: `{}`, wantErr: ErrCorrupt},
		{name: "null document", content: `null`, wantErr: ErrCorrupt},
		{name: "unknown version", content: `{"version":999}`, wantErr: ErrUnsupportedVersion},
		{
			name:    "invalid indexes",
			content: `{"version":1,"focused_panel":2,"panels":[{"active_tab":0}]}`,
			wantErr: ErrCorrupt,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			path := filepath.Join(dir, "last.json")
			if err := os.WriteFile(path, []byte(tt.content), 0o600); err != nil {
				t.Fatal(err)
			}
			store := NewStore(path, filepath.Join(dir, "workspaces"))
			if _, err := store.LoadLast(); !errors.Is(err, tt.wantErr) {
				t.Fatalf("LoadLast() error = %v, want %v", err, tt.wantErr)
			}
		})
	}

	t.Run("missing", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		store := NewStore(filepath.Join(dir, "missing.json"), filepath.Join(dir, "workspaces"))
		if _, err := store.LoadLast(); !errors.Is(err, ErrNotFound) || !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("LoadLast() error = %v, want ErrNotFound and os.ErrNotExist", err)
		}
	})
}

func TestSaveValidationPreservesExistingState(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "last.json")
	store := NewStore(path, filepath.Join(dir, "workspaces"))
	want := testState("/preserved")
	if err := store.SaveLast(want); err != nil {
		t.Fatal(err)
	}

	invalid := testState("/invalid")
	invalid.Version++
	if err := store.SaveLast(invalid); !errors.Is(err, ErrUnsupportedVersion) {
		t.Fatalf("SaveLast(invalid) error = %v, want ErrUnsupportedVersion", err)
	}
	got, err := store.LoadLast()
	if err != nil || !statesEqual(got, want) {
		t.Fatalf("state after rejected save = (%#v, %v), want %#v", got, err, want)
	}
}

func TestAtomicWriteFailurePreservesExistingState(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "last.json")
	store := NewStore(path, filepath.Join(dir, "workspaces"))
	want := testState("/preserved")
	if err := store.SaveLast(want); err != nil {
		t.Fatal(err)
	}

	writeErr := errors.New("injected replace failure")
	store.writeFile = func(path string, data []byte) error {
		return atomicWriteFileWithReplace(path, data, func(string, string) error { return writeErr })
	}
	if err := store.SaveLast(testState("/lost")); !errors.Is(err, writeErr) {
		t.Fatalf("SaveLast() error = %v, want injected error", err)
	}
	temporaryFiles, err := filepath.Glob(filepath.Join(dir, ".last.json.tmp-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(temporaryFiles) != 0 {
		t.Fatalf("temporary files after failed replacement = %q", temporaryFiles)
	}

	store.writeFile = atomicWriteFile
	got, err := store.LoadLast()
	if err != nil || !statesEqual(got, want) {
		t.Fatalf("state after failed write = (%#v, %v), want %#v", got, err, want)
	}
}

func TestWorkspaceNameValidation(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := NewStore(filepath.Join(dir, "last.json"), filepath.Join(dir, "workspaces"))
	invalid := []string{
		"", ".", "..", "../escape", `..\escape`, "/absolute", `C:\absolute`,
		" leading", "trailing ", "trailing.", "line\nbreak", "nul\x00byte", "CON", "con.txt",
		"CON .txt", "PRN", "AUX", "NUL", "COM1", "com9.log", "LPT1", "lpt9.txt", "a:b", "a*b", "a?b",
	}
	for _, name := range invalid {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := store.SaveWorkspace(name, testState("/safe")); !errors.Is(err, ErrInvalidWorkspaceName) {
				t.Fatalf("SaveWorkspace(%q) error = %v, want ErrInvalidWorkspaceName", name, err)
			}
			if _, err := store.LoadWorkspace(name); !errors.Is(err, ErrInvalidWorkspaceName) {
				t.Fatalf("LoadWorkspace(%q) error = %v, want ErrInvalidWorkspaceName", name, err)
			}
			if err := store.DeleteWorkspace(name); !errors.Is(err, ErrInvalidWorkspaceName) {
				t.Fatalf("DeleteWorkspace(%q) error = %v, want ErrInvalidWorkspaceName", name, err)
			}
		})
	}

	for _, name := range []string{"work", "Work 2026", "alpha-beta_1", "日本語"} {
		if err := store.SaveWorkspace(name, testState("/safe")); err != nil {
			t.Errorf("SaveWorkspace(%q) unexpected error = %v", name, err)
		}
	}
}

func TestPrivatePermissionsWhereSupported(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose Unix permission enforcement")
	}

	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "private", "workspaces")
	lastPath := filepath.Join(dir, "private-last", "last.json")
	store := NewStore(lastPath, workspaceDir)
	if err := store.SaveLast(testState("/last")); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveWorkspace("private", testState("/workspace")); err != nil {
		t.Fatal(err)
	}
	workspacePath := filepath.Join(workspaceDir, "private.json")
	if err := os.Chmod(workspacePath, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveWorkspace("private", testState("/overwritten")); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{lastPath, workspacePath} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Errorf("%s mode = %04o, want 0600", path, got)
		}
	}
	for _, path := range []string{filepath.Dir(lastPath), workspaceDir} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got&0o077 != 0 {
			t.Errorf("%s mode = %04o, want no group/other permissions", path, got)
		}
	}
}

func testState(location string) State {
	return State{
		Version:        CurrentVersion,
		FocusedPanel:   1,
		ShowDotFiles:   true,
		PreviewVisible: true,
		FooterVisible:  false,
		Panels: []PanelState{
			{
				ActiveTab:    0,
				SortKind:     4,
				SortReversed: true,
				Tabs: []TabState{{
					Location:       location,
					BackHistory:    []string{"/back"},
					ForwardHistory: []string{"/forward"},
				}},
			},
			{ActiveTab: 0, SortKind: 0, Tabs: []TabState{{Location: "/other"}}},
		},
	}
}

func statesEqual(a, b State) bool {
	if a.Version != b.Version || a.FocusedPanel != b.FocusedPanel ||
		a.ShowDotFiles != b.ShowDotFiles || a.PreviewVisible != b.PreviewVisible ||
		a.FooterVisible != b.FooterVisible || len(a.Panels) != len(b.Panels) {
		return false
	}
	for i := range a.Panels {
		ap, bp := a.Panels[i], b.Panels[i]
		if ap.ActiveTab != bp.ActiveTab || ap.SortKind != bp.SortKind ||
			ap.SortReversed != bp.SortReversed || len(ap.Tabs) != len(bp.Tabs) {
			return false
		}
		for j := range ap.Tabs {
			at, bt := ap.Tabs[j], bp.Tabs[j]
			if at.Location != bt.Location || !slices.Equal(at.BackHistory, bt.BackHistory) ||
				!slices.Equal(at.ForwardHistory, bt.ForwardHistory) {
				return false
			}
		}
	}
	return true
}
