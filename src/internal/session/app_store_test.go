package session

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestAppLastRoundTripPreservesOrderIdentityLayoutsAndOfflinePaths(t *testing.T) {
	t.Parallel()
	store, path := newTestAppStore(t)
	want := testAppState()
	want.Workspaces[0].Layout.Panels[0].Tabs = []TabState{
		{
			Location:       `Q:\offline\folder`,
			BackHistory:    []string{`\\nas\offline\share`},
			ForwardHistory: []string{`Q:\later`},
		},
		{Location: `/unmounted/volume`, BackHistory: []string{`/unmounted/earlier`}},
	}
	want.Workspaces[0].Layout.Panels[0].ActiveTab = 1
	if err := store.SaveAppLast(want); err != nil {
		t.Fatal(err)
	}
	got, err := store.LoadAppLast()
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadAppLast() = (%#v, %v), want %#v", got, err, want)
	}
	got.Workspaces[0].Layout.Panels[0].Tabs[0].BackHistory[0] = "mutated"
	want.Workspaces[2].Layout.Panels[0].Tabs[0].ForwardHistory[0] = "caller mutation"
	again, err := store.LoadAppLast()
	if err != nil || again.Workspaces[0].Layout.Panels[0].Tabs[0].BackHistory[0] != `\\nas\offline\share` ||
		again.Workspaces[2].Layout.Panels[0].Tabs[0].ForwardHistory[0] == "caller mutation" {
		t.Fatalf("later load aliases a caller: (%#v, %v)", again, err)
	}
	data := readTestFile(t, path)
	if bytes.Contains(data, []byte(`selected`)) || bytes.Contains(data, []byte(`dialog`)) {
		t.Fatalf("saved transient state: %s", data)
	}
}

func TestLegacyMigrationIsReadOnlyUntilSaveAndBacksUpExactSource(t *testing.T) {
	t.Parallel()
	store, path := newTestAppStore(t)
	legacy := testState(`Q:\offline`)
	legacy.FocusedPanel = 0
	legacy.Panels[0].Tabs = append(legacy.Panels[0].Tabs, TabState{
		Location: `\\nas\disconnected\日本語`, BackHistory: []string{`Q:\offline\before`},
	})
	legacy.Panels[0].ActiveTab = 1
	source, err := json.MarshalIndent(legacy, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	source = append(source, '\n')
	writeTestFile(t, path, source)
	got, err := store.LoadAppLast()
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Workspaces) != 1 || got.ActiveWorkspaceID != got.Workspaces[0].ID ||
		got.Version != CurrentAppVersion || got.Workspaces[0].Title != "日本語" ||
		!statesEqual(got.Workspaces[0].Layout, legacy) {
		t.Fatalf("migrated state = %#v", got)
	}
	again, err := store.LoadAppLast()
	if err != nil || !reflect.DeepEqual(got, again) {
		t.Fatalf("legacy migration is unstable: (%#v, %v)", again, err)
	}
	assertTestFile(t, path, source)
	if _, statErr := os.Stat(path + ".v1.bak"); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("read-only load created migration backup: %v", statErr)
	}
	if saveErr := store.SaveAppLast(got); saveErr != nil {
		t.Fatal(saveErr)
	}
	assertTestFile(t, path+".v1.bak", source)
	assertTestFile(t, path+".bak", source)
	reloaded, err := store.LoadAppLast()
	if err != nil || !reflect.DeepEqual(reloaded, got) {
		t.Fatalf("migrated reload = (%#v, %v), want %#v", reloaded, err, got)
	}
	// A subsequent save must not change the original legacy backup.
	reloaded.Workspaces[0].Title = "Renamed"
	if err := store.SaveAppLast(reloaded); err != nil {
		t.Fatal(err)
	}
	assertTestFile(t, path+".v1.bak", source)
}

func TestLegacyTitleFallbackAndFocusedFolder(t *testing.T) {
	t.Parallel()
	for _, location := range []string{"", "/", `C:\`, ".", "..", strings.Repeat("x", MaxWorkspaceTitleBytes+1)} {
		state := testState(location)
		state.FocusedPanel = 0
		if got := legacyWorkspaceTitle(state); got != "Workspace 1" {
			t.Errorf("legacyWorkspaceTitle(%q) = %q", location, got)
		}
	}
	state := testState("/not-focused")
	state.Panels[1].Tabs[0].Location = "/focused/folder"
	if got := legacyWorkspaceTitle(state); got != "folder" {
		t.Errorf("focused legacy title = %q", got)
	}
}

func TestAppLastRejectsAndPreservesInvalidCurrentFiles(t *testing.T) {
	t.Parallel()
	duplicate := testAppState()
	duplicate.Workspaces[1].ID = duplicate.Workspaces[0].ID
	duplicateData, err := json.Marshal(duplicate)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		data []byte
		want error
	}{
		{"truncated", []byte(`{"version":2,"workspaces":[`), ErrCorrupt},
		{"unknown version", []byte(`{"version":999,"future_data":[1,2,3]}`), ErrUnsupportedVersion},
		{"duplicate identities", duplicateData, ErrCorrupt},
		{"empty workspaces", []byte(`{"version":2,"active_workspace_id":"a","workspaces":[]}`), ErrCorrupt},
		{"missing version", []byte(`{}`), ErrCorrupt},
		{"null version", []byte(`{"version":null}`), ErrCorrupt},
		{"null", []byte(`null`), ErrCorrupt},
		{"wrong version type", []byte(`{"version":"2"}`), ErrCorrupt},
		{"trailing JSON", []byte(`{"version":1} {}`), ErrCorrupt},
		{"unknown fields", []byte(`{"version":1,"extra_durable_state":true}`), ErrCorrupt},
		{"duplicate schema version", []byte(`{"version":999,"version":1}`), ErrCorrupt},
		{
			"duplicate nested fields",
			[]byte(`{"version":1,"panels":[{"tabs":[{"location":"/first","location":"/second"}]}]}`),
			ErrCorrupt,
		},
		{"invalid UTF8", []byte{'{', '"', 'x', '"', ':', '"', 0xff, '"', '}'}, ErrCorrupt},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			store, path := newTestAppStore(t)
			writeTestFile(t, path, tt.data)
			if _, err := store.LoadAppLast(); !errors.Is(err, tt.want) {
				t.Fatalf("LoadAppLast() error = %v, want %v", err, tt.want)
			}
			if err := store.SaveAppLast(testAppState()); err == nil {
				t.Fatal("SaveAppLast() overwrote invalid current file")
			}
			if err := store.SaveLast(testState("/new")); err == nil {
				t.Fatal("legacy SaveLast() overwrote invalid current file")
			}
			assertTestFile(t, path, tt.data)
			if _, err := os.Stat(path + ".bak"); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("invalid primary was copied to recovery backup: %v", err)
			}
		})
	}
}

func TestAppSaveValidationPreservesExistingBytes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		edit func(*AppState)
	}{
		{"wrong app version", func(s *AppState) { s.Version++ }},
		{"no workspace", func(s *AppState) { s.Workspaces = nil }},
		{"missing active identity", func(s *AppState) { s.ActiveWorkspaceID = "not-present" }},
		{"duplicate identity", func(s *AppState) { s.Workspaces[1].ID = s.Workspaces[0].ID }},
		{"empty identity", func(s *AppState) { s.Workspaces[0].ID = "" }},
		{"invalid identity", func(s *AppState) { s.Workspaces[0].ID = "with space" }},
		{"overlong identity", func(s *AppState) { s.Workspaces[0].ID = strings.Repeat("x", MaxWorkspaceIDBytes+1) }},
		{"empty title", func(s *AppState) { s.Workspaces[0].Title = "  " }},
		{"control title", func(s *AppState) { s.Workspaces[0].Title = "line\nbreak" }},
		{"overlong title", func(s *AppState) { s.Workspaces[0].Title = strings.Repeat("x", MaxWorkspaceTitleBytes+1) }},
		{"unknown layout version", func(s *AppState) { s.Workspaces[0].Layout.Version++ }},
		{"invalid focused panel", func(s *AppState) { s.Workspaces[0].Layout.FocusedPanel = -1 }},
		{"invalid active folder tab", func(s *AppState) { s.Workspaces[0].Layout.Panels[0].ActiveTab = 100 }},
		{"too many workspaces", func(s *AppState) { s.Workspaces = make([]WorkspaceState, MaxWorkspaces+1) }},
		{"too many panels", func(s *AppState) { s.Workspaces[0].Layout.Panels = make([]PanelState, MaxPanels+1) }},
		{
			"too many folder tabs",
			func(s *AppState) { s.Workspaces[0].Layout.Panels[0].Tabs = make([]TabState, MaxTabsPerPanel+1) },
		},
		{"too much history", func(s *AppState) {
			s.Workspaces[0].Layout.Panels[0].Tabs[0].BackHistory = make([]string, MaxHistoryEntries+1)
		}},
		{"overlong location", func(s *AppState) {
			s.Workspaces[0].Layout.Panels[0].Tabs[0].Location = strings.Repeat("x", MaxLocationBytes+1)
		}},
		{"NUL history location", func(s *AppState) {
			s.Workspaces[0].Layout.Panels[0].Tabs[0].ForwardHistory = []string{"nul\x00location"}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			store, path := newTestAppStore(t)
			if err := store.SaveAppLast(testAppState()); err != nil {
				t.Fatal(err)
			}
			before := readTestFile(t, path)
			invalid := testAppState()
			tt.edit(&invalid)
			if err := store.SaveAppLast(invalid); err == nil {
				t.Fatal("SaveAppLast(invalid) succeeded")
			}
			assertTestFile(t, path, before)
		})
	}
}

func TestAppLastBackupRecoveryIsExplicitAndPreservesCorruptPrimary(t *testing.T) {
	t.Parallel()
	store, path := newTestAppStore(t)
	first := testAppState()
	if err := store.SaveAppLast(first); err != nil {
		t.Fatal(err)
	}
	second := CloneAppState(first)
	second.Workspaces[0].Title = "Second"
	if err := store.SaveAppLast(second); err != nil {
		t.Fatal(err)
	}
	corrupt := []byte(`{"version":2, interrupted`)
	writeTestFile(t, path, corrupt)
	if _, err := store.LoadAppLast(); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("LoadAppLast() = %v, want visible corruption", err)
	}
	recovered, err := store.LoadAppBackup()
	if err != nil || !reflect.DeepEqual(recovered, first) {
		t.Fatalf("LoadAppBackup() = (%#v, %v), want %#v", recovered, err, first)
	}
	if err := store.SaveAppLast(recovered); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("saving recovered state silently replaced corrupt primary: %v", err)
	}
	assertTestFile(t, path, corrupt)
	if got, err := store.LoadAppBackup(); err != nil || !reflect.DeepEqual(got, first) {
		t.Fatalf("backup changed after rejected save: (%#v, %v)", got, err)
	}
}

func TestAppLastWriteFailureRetainsPrimaryAndRecovery(t *testing.T) {
	t.Parallel()
	for _, failureTarget := range []string{"backup", "primary"} {
		t.Run(failureTarget, func(t *testing.T) {
			t.Parallel()
			store, path := newTestAppStore(t)
			if err := store.SaveAppLast(testAppState()); err != nil {
				t.Fatal(err)
			}
			before := readTestFile(t, path)
			injected := errors.New("injected write failure")
			store.writeFile = failingAppWriter(path, failureTarget, injected)
			next := testAppState()
			next.Workspaces[0].Title = "Not persisted"
			if err := store.SaveAppLast(next); !errors.Is(err, injected) {
				t.Fatalf("SaveAppLast() = %v, want injected error", err)
			}
			assertTestFile(t, path, before)
			if failureTarget == "primary" {
				assertTestFile(t, path+".bak", before)
			}
			temporary, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".*.tmp-*"))
			if err != nil || len(temporary) != 0 {
				t.Fatalf("temporary files after failure = (%v, %v)", temporary, err)
			}
		})
	}
}

func failingAppWriter(path, failureTarget string, injected error) func(string, []byte) error {
	return func(target string, data []byte) error {
		fail := failureTarget == "primary" && target == path ||
			failureTarget == "backup" && target == path+".bak"
		if fail {
			return atomicWriteFileWithReplace(target, data, func(string, string) error { return injected })
		}
		return atomicWriteFile(target, data)
	}
}

func TestLegacyMigrationPreservesAnOccupiedBackup(t *testing.T) {
	t.Parallel()
	store, path := newTestAppStore(t)
	source, err := json.Marshal(testState("/source"))
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, path, source)
	previousBackup := []byte("earlier migration source - preserve exactly")
	writeTestFile(t, path+".v1.bak", previousBackup)
	app, err := store.LoadAppLast()
	if err != nil {
		t.Fatal(err)
	}
	if saveErr := store.SaveAppLast(app); saveErr != nil {
		t.Fatal(saveErr)
	}
	assertTestFile(t, path+".v1.bak", previousBackup)
	additional, err := filepath.Glob(path + ".v1.*.bak")
	if err != nil || len(additional) != 1 {
		t.Fatalf("additional immutable backups = (%v, %v)", additional, err)
	}
	assertTestFile(t, additional[0], source)
}

func TestLegacyMigrationBackupFailureDoesNotReplaceSource(t *testing.T) {
	t.Parallel()
	store, path := newTestAppStore(t)
	source, err := json.Marshal(testState("/source"))
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, path, source)
	if mkdirErr := os.Mkdir(path+".v1.bak", 0o700); mkdirErr != nil {
		t.Fatal(mkdirErr)
	}
	app, err := store.LoadAppLast()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveAppLast(app); err == nil {
		t.Fatal("migration ignored backup failure")
	}
	assertTestFile(t, path, source)
}

func TestAppAndLegacyMissingAndVersionBoundaries(t *testing.T) {
	t.Parallel()
	store, path := newTestAppStore(t)
	if _, err := store.LoadAppLast(); !errors.Is(err, ErrNotFound) || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing LoadAppLast() = %v", err)
	}
	if _, err := store.LoadAppBackup(); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing LoadAppBackup() = %v", err)
	}
	if err := store.SaveAppLast(testAppState()); err != nil {
		t.Fatal(err)
	}
	before := readTestFile(t, path)
	if err := store.SaveLast(testState("/legacy")); !errors.Is(err, ErrUnsupportedVersion) {
		t.Fatalf("legacy SaveLast(app v2) = %v", err)
	}
	assertTestFile(t, path, before)
	if err := store.SaveWorkspace("active-only", testAppState().Workspaces[1].Layout); err != nil {
		t.Fatal(err)
	}
	preset, err := store.LoadWorkspace("active-only")
	if err != nil || !statesEqual(preset, testAppState().Workspaces[1].Layout) {
		t.Fatalf("legacy preset round trip = (%#v, %v)", preset, err)
	}
	assertTestFile(t, path, before)
}

func TestAppStateFileSizeLimitPreservesOversizedSource(t *testing.T) {
	t.Parallel()
	store, path := newTestAppStore(t)
	oversized := bytes.Repeat([]byte(" "), MaxStateFileBytes+1)
	writeTestFile(t, path, oversized)
	if _, err := store.LoadAppLast(); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("oversized LoadAppLast() = %v", err)
	}
	if err := store.SaveAppLast(testAppState()); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("oversized primary overwritten: %v", err)
	}
	assertTestFile(t, path, oversized)
}

func TestCloneAppStateDoesNotAliasLayoutsTabsOrHistories(t *testing.T) {
	t.Parallel()
	original := testAppState()
	before := testAppState()
	clone := CloneAppState(original)
	clone.Workspaces[0].Title = "Changed"
	clone.Workspaces[0].Layout.Panels[0].Tabs[0].Location = "changed"
	clone.Workspaces[0].Layout.Panels[0].Tabs[0].BackHistory[0] = "changed"
	clone.Workspaces[1].Layout.Panels[0].Tabs[0].ForwardHistory[0] = "changed"
	clone.Workspaces[2].Layout.Panels[0].ActiveTab = 9
	if !reflect.DeepEqual(original, before) {
		t.Fatalf("CloneAppState mutated original: %#v", original)
	}
	if CloneState(State{}).Panels != nil || CloneAppState(AppState{}).Workspaces != nil {
		t.Fatal("cloning zero values changed nil slices")
	}
	empty := State{
		Version: CurrentVersion,
		Panels:  []PanelState{{Tabs: []TabState{{BackHistory: []string{}, ForwardHistory: []string{}}}}},
	}
	if !reflect.DeepEqual(CloneState(empty), empty) {
		t.Fatal("clone changed empty slice representation")
	}
}

func TestNewWorkspaceIDIsValidAndIndependent(t *testing.T) {
	t.Parallel()
	seen := make(map[string]bool)
	for range 100 {
		id, err := NewWorkspaceID()
		if err != nil || !validWorkspaceID(id) || seen[id] {
			t.Fatalf("NewWorkspaceID() = (%q, %v), duplicate=%v", id, err, seen[id])
		}
		seen[id] = true
	}
}

func newTestAppStore(t *testing.T) (*Store, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "last.json")
	return NewStore(path, filepath.Join(dir, "workspaces")), path
}

func testAppState() AppState {
	return AppState{
		Version:           CurrentAppVersion,
		ActiveWorkspaceID: "second",
		Workspaces: []WorkspaceState{
			{ID: "first", Title: "日本語", Layout: testState("/first")},
			{ID: "second", Title: "Second workspace", Layout: testState("/second")},
			{ID: "third", Title: "Third workspace", Layout: testState("/third")},
		},
	}
}

func writeTestFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, privateFileMode); err != nil {
		t.Fatal(err)
	}
}

func readTestFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func assertTestFile(t *testing.T, path string, want []byte) {
	t.Helper()
	if got := readTestFile(t, path); !bytes.Equal(got, want) {
		t.Fatalf("%s bytes changed: got length %d, want length %d", path, len(got), len(want))
	}
}
