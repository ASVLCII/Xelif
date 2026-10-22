package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/plugincommands"
	"github.com/yorukot/superfile/src/internal/ui/commandpalette"
	"github.com/yorukot/superfile/src/internal/ui/filepanel"
)

func TestPluginCommandIntegrationHelper(t *testing.T) {
	separator := slices.Index(os.Args, "--")
	if separator < 0 || separator+1 >= len(os.Args) {
		return
	}
	args := os.Args[separator+1:]
	marker := args[0]
	if err := os.WriteFile(marker, []byte("executed"), 0o600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	fmt.Println("cwd=" + cwd)
	for _, path := range args[1:] {
		fmt.Println("path=" + path)
	}
	os.Exit(0)
}

func TestPluginCommandsRefreshPopulatesPaletteWithoutExecution(t *testing.T) {
	root := t.TempDir()
	selected := filepath.Join(root, "selected.txt")
	require.NoError(t, os.WriteFile(selected, []byte("selected"), 0o600))
	manifestDir := t.TempDir()
	marker := filepath.Join(t.TempDir(), "marker")
	writePluginManifest(t, manifestDir, integrationManifest(t, "inspect", "Inspect selection", marker))

	m := defaultTestModel(root)
	m.pluginCommandDir = manifestDir
	count, err := m.refreshPluginCommands()
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	assert.NoFileExists(t, marker, "discovery must never execute a plugin")

	items := m.paletteItems()
	pluginItem, ok := findPaletteItem(items, commandIDPluginRunPrefix+"inspect")
	require.True(t, ok)
	assert.Equal(t, "Plugin: Inspect selection", pluginItem.Label)
	assert.Equal(t, "Plugins", pluginItem.Category)
	assert.False(t, pluginItem.Disabled)
	_, ok = findPaletteItem(items, commandIDPluginRefresh)
	assert.True(t, ok, "the explicit refresh action must always be available")
	assert.NoFileExists(t, marker, "opening or rendering the palette must never execute a plugin")
}

func TestPluginCommandPaletteExecutionUsesSelectionAndCurrentDirectory(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "one file.txt")
	second := filepath.Join(root, "二.txt")
	require.NoError(t, os.WriteFile(first, []byte("one"), 0o600))
	require.NoError(t, os.WriteFile(second, []byte("two"), 0o600))
	manifestDir := t.TempDir()
	marker := filepath.Join(t.TempDir(), "marker")
	writePluginManifest(t, manifestDir, integrationManifest(t, "inspect", "Inspect selection", marker))

	m := defaultTestModel(root)
	m.pluginCommandDir = manifestDir
	_, err := m.refreshPluginCommands()
	require.NoError(t, err)
	panel := m.getFocusedFilePanel()
	panel.ChangeFilePanelMode()
	require.Equal(t, filepanel.SelectMode, panel.PanelMode)
	panel.SetSelectedAll([]string{second, first})

	cmd := m.applyCommandPaletteAction(commandpalette.ExecuteAction{ID: commandIDPluginRunPrefix + "inspect"})
	require.NotNil(t, cmd)
	assert.NoFileExists(t, marker, "returning the command must not execute it")
	assert.True(t, m.textViewer.IsOpen())
	assert.Contains(t, ansi.Strip(m.textViewer.Render()), "Running")

	msg, ok := cmd().(pluginCommandFinishedMsg)
	require.True(t, ok)
	require.NoError(t, msg.err, "output: %s", msg.result.Output)
	assert.FileExists(t, marker)
	output := string(msg.result.Output)
	// Child processes report their physical working directory on macOS, where
	// the temporary directory may be reached through the /var symlink.
	physicalRoot, err := filepath.EvalSymlinks(root)
	require.NoError(t, err)
	assert.Contains(t, output, "cwd="+physicalRoot)
	assert.Contains(t, output, "path="+first)
	assert.Contains(t, output, "path="+second)

	TeaUpdate(m, msg)
	assert.True(t, m.textViewer.IsOpen())
	content := renderPluginCommandResult(msg)
	assert.Contains(t, content, "Status: completed")
	assert.Contains(t, content, "Exit code: 0")
	assert.Contains(t, content, "cwd="+physicalRoot)
	assert.False(t, m.notifyModel.IsOpen())
}

func TestPluginCommandFailureUsesTypedErrorAndBothResultSurfaces(t *testing.T) {
	root := t.TempDir()
	selected := filepath.Join(root, "image.png")
	require.NoError(t, os.WriteFile(selected, []byte("image"), 0o600))
	manifestDir := t.TempDir()
	marker := filepath.Join(t.TempDir(), "marker")
	manifest := integrationManifest(t, "text-only", "Text only", marker)
	manifest.Extensions = []string{".txt"}
	writePluginManifest(t, manifestDir, manifest)

	m := defaultTestModel(root)
	m.pluginCommandDir = manifestDir
	_, err := m.refreshPluginCommands()
	require.NoError(t, err)
	cmd := m.applyCommandPaletteAction(commandpalette.ExecuteAction{ID: commandIDPluginRunPrefix + "text-only"})
	require.NotNil(t, cmd)
	msg, ok := cmd().(pluginCommandFinishedMsg)
	require.True(t, ok)
	var extensionErr *plugincommands.ExtensionError
	require.ErrorAs(t, msg.err, &extensionErr)
	assert.NoFileExists(t, marker)

	TeaUpdate(m, msg)
	assert.True(t, m.textViewer.IsOpen(), "the bounded command result must remain inspectable")
	assert.True(t, m.notifyModel.IsOpen(), "the typed failure must reach the existing error surface")
	assert.Equal(t, "Plugin command failed", m.notifyModel.GetTitle())
	assert.Contains(t, m.notifyModel.GetContent(), "does not accept")
	assert.Contains(t, renderPluginCommandResult(msg), "Status: failed")
}

func TestPluginCommandRefreshFailurePreservesLastGoodRegistry(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "selected.txt"), []byte("selected"), 0o600))
	manifestDir := t.TempDir()
	marker := filepath.Join(t.TempDir(), "marker")
	writePluginManifest(t, manifestDir, integrationManifest(t, "known", "Known", marker))
	m := defaultTestModel(root)
	m.pluginCommandDir = manifestDir
	_, err := m.refreshPluginCommands()
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(manifestDir, "broken.json"), []byte("{"), 0o600))
	_, err = m.refreshPluginCommands()
	var manifestErr *plugincommands.ManifestError
	require.ErrorAs(t, err, &manifestErr)
	require.Len(t, m.pluginCommands.List(), 1)

	m.applyCommandPaletteAction(commandpalette.ExecuteAction{ID: commandIDPluginRefresh})
	assert.True(t, m.notifyModel.IsOpen())
	assert.Equal(t, "Could not refresh command plugins", m.notifyModel.GetTitle())
	_, ok := findPaletteItem(m.paletteItems(), commandIDPluginRunPrefix+"known")
	assert.True(t, ok, "a failed refresh must retain the last known-good commands")
}

func TestPluginCommandsStartupFailureUsesErrorSurface(t *testing.T) {
	manifestDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(manifestDir, "broken.json"), []byte("{"), 0o600))
	m := defaultTestModel(t.TempDir())
	m.pluginCommandDir = manifestDir

	m.initializePluginCommands()

	assert.True(t, m.notifyModel.IsOpen())
	assert.Equal(t, "Could not load command plugins", m.notifyModel.GetTitle())
	assert.Contains(t, m.notifyModel.GetContent(), "broken.json")
}

func integrationManifest(t *testing.T, id, name, marker string) plugincommands.Manifest {
	t.Helper()
	executable, err := os.Executable()
	require.NoError(t, err)
	return plugincommands.Manifest{
		Version:        plugincommands.ManifestVersion,
		ID:             id,
		Name:           name,
		Executable:     executable,
		Args:           []string{"-test.run=^TestPluginCommandIntegrationHelper$", "--", marker, "{paths}"},
		Extensions:     []string{".txt"},
		TimeoutMS:      2_000,
		MaxOutputBytes: 64 * 1024,
		EnvAllowlist:   []string{"GOCOVERDIR", "TEMP", "TMP"},
	}
}

func writePluginManifest(t *testing.T, dir string, manifest plugincommands.Manifest) {
	t.Helper()
	data, err := json.Marshal(manifest)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, manifest.ID+".json"), data, 0o600))
}

func findPaletteItem(items []commandpalette.Item, id string) (commandpalette.Item, bool) {
	for _, item := range items {
		if item.ID == id {
			return item, true
		}
	}
	return commandpalette.Item{}, false
}

func TestRenderPluginCommandResultNoOutput(t *testing.T) {
	msg := pluginCommandFinishedMsg{
		manifest: plugincommands.Manifest{ID: "quiet", Name: "Quiet"},
		result:   plugincommands.Result{PluginID: "quiet", ExitCode: 0},
	}
	content := renderPluginCommandResult(msg)
	assert.Contains(t, content, "Output:\n(no output)")

	msg.result.Output = []byte("safe\x1b[2J\x00")
	content = renderPluginCommandResult(msg)
	assert.Contains(t, content, "safe[2J")
	assert.NotContains(t, content, "\x1b")
	assert.NotContains(t, content, "\x00")
}

func TestPluginCommandResultsIgnoreStaleGenerationAndCancelReplacement(t *testing.T) {
	m := defaultTestModel(t.TempDir())
	canceled := false
	m.pluginCommandCancel = func() { canceled = true }
	m.pluginCommandReqID = 1

	_ = m.runPluginCommand("replacement")
	assert.True(t, canceled)
	require.Equal(t, 2, m.pluginCommandReqID)

	current := pluginCommandFinishedMsg{
		reqID:    2,
		manifest: plugincommands.Manifest{ID: "replacement", Name: "Replacement"},
		result: plugincommands.Result{
			PluginID: "replacement", ExitCode: 0, Output: []byte("current result"),
		},
	}
	m.applyPluginCommandResult(current)
	currentRender := ansi.Strip(m.textViewer.Render())

	stale := pluginCommandFinishedMsg{
		reqID:    1,
		manifest: plugincommands.Manifest{ID: "old", Name: "Old"},
		result:   plugincommands.Result{PluginID: "old", ExitCode: 0, Output: []byte("stale result")},
	}
	m.applyPluginCommandResult(stale)
	assert.Equal(t, currentRender, ansi.Strip(m.textViewer.Render()))
}

func TestPluginCommandNotificationSanitizesErrorsAndPreservesType(t *testing.T) {
	m := defaultTestModel(t.TempDir())
	m.pluginCommandReqID = 1
	cause := &plugincommands.StartError{PluginID: "unsafe", Err: errors.New("bad\x1b]2;title\a path")}
	msg := pluginCommandFinishedMsg{
		reqID:    1,
		manifest: plugincommands.Manifest{ID: "unsafe", Name: "Unsafe"},
		result:   plugincommands.Result{PluginID: "unsafe", ExitCode: -1},
		err:      cause,
	}

	m.applyPluginCommandResult(msg)

	content := m.notifyModel.GetContent()
	assert.NotContains(t, content, "\x1b")
	assert.NotContains(t, content, "\a")
	var typed *plugincommands.StartError
	require.ErrorAs(t, sanitizePluginCommandError(cause), &typed)
}
