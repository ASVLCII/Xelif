package internal

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/profile"
)

func appearanceWorkflowPaths(root string) profile.Paths {
	return profile.Paths{
		ConfigDir:   filepath.Join(root, "config"),
		DataDir:     filepath.Join(root, "data"),
		StateDir:    filepath.Join(root, "state"),
		CacheDir:    filepath.Join(root, "cache"),
		ConfigFile:  filepath.Join(root, "config", "config.toml"),
		HotkeysFile: filepath.Join(root, "config", "hotkeys.toml"),
	}
}

func appearanceWorkflowModel(t *testing.T, legacy, active profile.Paths) *model {
	t.Helper()
	m := defaultTestModel(t.TempDir())
	m.firstUse = false
	m.appearanceOpen = true
	m.appearanceChoice = 0
	m.appearanceMessage = ""
	m.setupImport = &setupImportState{Mode: setupImportMain, Legacy: legacy, Active: active}
	TeaUpdate(m, tea.WindowSizeMsg{Width: 60, Height: 24})
	return m
}

func appearancePreferences(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "superfile_config", "config.toml"))
	require.NoError(t, err)
	return data
}

func writeAppearanceFile(t *testing.T, path string, data []byte) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, data, 0o600))
}

func appearanceClickLabel(t *testing.T, m *model, label string) {
	t.Helper()
	content := m.appearanceRender()
	lines := strings.Split(ansi.Strip(content), "\n")
	row, column := -1, -1
	for index, line := range lines {
		if offset := strings.Index(line, label); offset >= 0 {
			row, column = index, ansi.StringWidth(line[:offset])
			break
		}
	}
	require.GreaterOrEqual(t, row, 0, "label %q not rendered in:\n%s", label, ansi.Strip(content))
	x := m.fullWidth/2 - lipgloss.Width(content)/2 + column
	y := m.fullHeight/2 - lipgloss.Height(content)/2 + row
	TeaUpdate(m, tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
}

func appearanceSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	snapshot := map[string]string{}
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return snapshot
	}
	require.NoError(t, filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			snapshot[relative] = "directory"
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		snapshot[relative] = hex.EncodeToString(sum[:])
		return nil
	}))
	return snapshot
}

func writePendingRecovery(t *testing.T, active profile.Paths, data []byte, corrupt bool) string {
	t.Helper()
	dir := filepath.Join(active.StateDir, "legacy-import-pending")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "staged"), 0o700))
	if corrupt {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "journal.json"), []byte("{"), 0o600))
		return dir
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "staged", "0000"), data, 0o600))
	sum := sha256.Sum256(data)
	journal := map[string]any{
		"version":     1,
		"id":          strings.Repeat("a", 32),
		"destination": active,
		"entries": []any{map[string]any{
			"kind":   "preferences",
			"sha256": hex.EncodeToString(sum[:]),
		}},
	}
	encoded, err := json.MarshalIndent(journal, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "journal.json"), append(encoded, '\n'), 0o600))
	return dir
}

func TestAppearanceLegacyOfferPreviewPagingAndCancelNoWrites(t *testing.T) {
	root := t.TempDir()
	legacy := appearanceWorkflowPaths(filepath.Join(root, "legacy"))
	active := appearanceWorkflowPaths(filepath.Join(root, "active"))
	preferences := appearancePreferences(t)
	writeAppearanceFile(t, legacy.ConfigFile, preferences)
	require.NoError(t, os.MkdirAll(active.ConfigDir, 0o700))

	legacyBefore := appearanceSnapshot(t, legacy.ConfigDir)
	activeBefore := appearanceSnapshot(t, filepath.Dir(active.ConfigDir))
	m := appearanceWorkflowModel(t, legacy, active)

	view := m.appearanceRender()
	require.Contains(t, view, "Legacy profile found: 1 ready")
	require.Contains(t, view, "Import legacy profile")
	require.LessOrEqual(t, lipgloss.Width(view), 60)
	require.LessOrEqual(t, lipgloss.Height(view), 24)

	appearanceClickLabel(t, m, "Rich icons")
	require.Equal(t, 1, m.appearanceChoice)
	appearanceClickLabel(t, m, "Portable mode")
	require.Zero(t, m.appearanceChoice)
	TeaUpdate(m, tea.MouseClickMsg{X: 0, Y: 0, Button: tea.MouseLeft})
	require.Equal(t, setupImportMain, m.setupImport.Mode)

	appearanceClickLabel(t, m, "Import legacy profile")
	require.Equal(t, setupImportPreview, m.setupImport.Mode)
	preview := m.appearanceRender()
	require.Contains(t, preview, "Import valid files")
	require.Contains(t, preview, "PgUp: previous")
	require.Contains(t, preview, "PgDn: next")
	require.Contains(t, preview, "Page 1/1")

	appearanceClickLabel(t, m, "Cancel")
	require.Equal(t, setupImportMain, m.setupImport.Mode)
	require.Contains(t, m.appearanceMessage, "Import cancelled")
	require.Equal(t, legacyBefore, appearanceSnapshot(t, legacy.ConfigDir))
	require.Equal(t, activeBefore, appearanceSnapshot(t, filepath.Dir(active.ConfigDir)))

	m.setupImport = &setupImportState{
		Mode:    setupImportPreview,
		Checked: true,
		Legacy:  legacy,
		Active:  active,
		Preview: profile.ImportPreview{Found: true, ReadyCount: 8, Items: make([]profile.ImportItem, 8)},
	}
	require.Contains(t, m.appearanceRender(), "Page 1/2")
	TeaUpdate(m, tea.KeyPressMsg{Code: tea.KeyRight})
	require.Equal(t, 1, m.setupImport.Page)
	require.Contains(t, m.appearanceRender(), "Page 2/2")
	appearanceClickLabel(t, m, "PgUp")
	require.Zero(t, m.setupImport.Page)
	require.Contains(t, m.appearanceRender(), "Page 1/2")
	appearanceClickLabel(t, m, "Cancel")
	require.Equal(t, legacyBefore, appearanceSnapshot(t, legacy.ConfigDir))
	require.Equal(t, activeBefore, appearanceSnapshot(t, filepath.Dir(active.ConfigDir)))
}

func TestAppearanceRecoveryRequiresConfirmationAndSucceeds(t *testing.T) {
	root := t.TempDir()
	legacy := appearanceWorkflowPaths(filepath.Join(root, "legacy"))
	active := appearanceWorkflowPaths(filepath.Join(root, "active"))
	preferences := appearancePreferences(t)
	pending := writePendingRecovery(t, active, preferences, false)
	before := appearanceSnapshot(t, filepath.Dir(active.ConfigDir))

	m := appearanceWorkflowModel(t, legacy, active)
	view := m.appearanceRender()
	require.Contains(t, view, "Interrupted import found")
	require.Contains(t, view, "Recover interrupted import")
	require.LessOrEqual(t, lipgloss.Width(view), 60)
	require.LessOrEqual(t, lipgloss.Height(view), 24)

	appearanceClickLabel(t, m, "Recover interrupted import")
	require.Equal(t, setupImportRecover, m.setupImport.Mode)
	confirmation := m.appearanceRender()
	require.Contains(t, confirmation, "may be partial")
	require.Contains(t, confirmation, "Recover now (Enter)")
	require.Contains(t, confirmation, "Cancel (Esc)")
	require.Equal(t, before, appearanceSnapshot(t, filepath.Dir(active.ConfigDir)))
	_, err := os.Stat(active.ConfigFile)
	require.ErrorIs(t, err, os.ErrNotExist)

	appearanceClickLabel(t, m, "Cancel")
	require.Equal(t, setupImportMain, m.setupImport.Mode)
	require.Contains(t, m.appearanceMessage, "Recovery cancelled")
	require.Equal(t, before, appearanceSnapshot(t, filepath.Dir(active.ConfigDir)))

	appearanceClickLabel(t, m, "Recover interrupted import")
	appearanceClickLabel(t, m, "Recover now")
	require.Contains(t, m.appearanceMessage, "Recovered 1 files")
	require.Equal(t, setupImportMain, m.setupImport.Mode)
	imported, err := os.ReadFile(active.ConfigFile)
	require.NoError(t, err)
	require.Equal(t, preferences, imported)
	_, err = os.Stat(pending)
	require.ErrorIs(t, err, os.ErrNotExist)
	backups, err := filepath.Glob(filepath.Join(active.StateDir, "legacy-import-backups", "*"))
	require.NoError(t, err)
	require.NotEmpty(t, backups)

	view = m.appearanceRender()
	require.NotContains(t, view, "Interrupted import found")
	require.LessOrEqual(t, lipgloss.Width(view), 60)
	require.LessOrEqual(t, lipgloss.Height(view), 24)
}

func TestAppearanceRecoveryFailurePreservesExistingFiles(t *testing.T) {
	root := t.TempDir()
	legacy := appearanceWorkflowPaths(filepath.Join(root, "legacy"))
	active := appearanceWorkflowPaths(filepath.Join(root, "active"))
	existing := []byte("# keep these bytes\nnerdfont = false\n")
	writeAppearanceFile(t, active.ConfigFile, existing)
	pending := writePendingRecovery(t, active, []byte("unused"), true)
	before := appearanceSnapshot(t, filepath.Dir(active.ConfigDir))

	m := appearanceWorkflowModel(t, legacy, active)
	require.Contains(t, m.appearanceRender(), "Interrupted import found")
	appearanceClickLabel(t, m, "Recover interrupted import")
	require.Contains(t, m.appearanceRender(), "Recover now (Enter)")
	require.Equal(t, before, appearanceSnapshot(t, filepath.Dir(active.ConfigDir)))

	appearanceClickLabel(t, m, "Recover now")
	require.Contains(t, m.appearanceMessage, "Recovery failed")
	require.Contains(t, m.appearanceMessage, "journal archived")
	require.Contains(t, m.appearanceMessage, "existing files preserved")
	current, err := os.ReadFile(active.ConfigFile)
	require.NoError(t, err)
	require.Equal(t, existing, current)
	_, err = os.Stat(pending)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestAppearanceWorkflowFitsSixtyByTwentyFour(t *testing.T) {
	root := t.TempDir()
	legacy := appearanceWorkflowPaths(filepath.Join(root, "legacy"))
	active := appearanceWorkflowPaths(filepath.Join(root, "active"))
	m := appearanceWorkflowModel(t, legacy, active)
	m.appearanceMessage = "A notice that must stay inside the overlay."
	m.setupImport = &setupImportState{
		Mode:         setupImportMain,
		Checked:      true,
		Legacy:       legacy,
		Active:       active,
		Status:       profile.Status{PendingImport: true},
		InspectError: "inspect failed",
		Preview: profile.ImportPreview{
			Found: true, ReadyCount: 7, RejectedCount: 1, Items: make([]profile.ImportItem, 7),
		},
	}

	for _, size := range [][2]int{{60, 24}, {80, 24}, {132, 40}} {
		TeaUpdate(m, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		for _, mode := range []setupImportMode{setupImportMain, setupImportPreview, setupImportRecover} {
			m.setupImport.Mode, m.setupImport.Page = mode, 0
			view := m.appearanceRender()
			require.LessOrEqual(t, lipgloss.Width(view), size[0], "mode %d at %dx%d", mode, size[0], size[1])
			require.LessOrEqual(
				t,
				lipgloss.Height(view),
				size[1],
				"mode %d at %dx%d:\n%s",
				mode,
				size[0],
				size[1],
				ansi.Strip(view),
			)
		}
	}
}
