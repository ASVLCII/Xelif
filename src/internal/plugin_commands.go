package internal

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/plugincommands"
	"github.com/yorukot/superfile/src/internal/ui/commandpalette"
	"github.com/yorukot/superfile/src/internal/ui/filepanel"
)

const (
	commandIDPluginRefresh   = "plugin_refresh"
	commandIDPluginRunPrefix = "plugin_run:"
)

type pluginCommandFinishedMsg struct {
	reqID    int
	manifest plugincommands.Manifest
	result   plugincommands.Result
	err      error
}

func (m *model) initializePluginCommands() {
	if _, err := m.refreshPluginCommands(); err != nil {
		m.showUserError("Could not load command plugins", sanitizePluginCommandError(err))
	}
}

func (m *model) refreshPluginCommands() (int, error) {
	registry, err := plugincommands.Discover(m.pluginCommandDir)
	if err != nil {
		return 0, err
	}
	m.pluginCommands = registry
	return len(registry.List()), nil
}

func (m *model) refreshPluginCommandsFromPalette() {
	count, err := m.refreshPluginCommands()
	if err != nil {
		m.showUserError("Could not refresh command plugins", sanitizePluginCommandError(err))
		return
	}
	m.textViewer.Open("Command plugins", fmt.Sprintf("Loaded %d command plugin(s).\n\nDirectory: %s", count,
		m.pluginCommandDir))
	m.setTextViewerSize()
}

func (m *model) pluginCommandPaletteItems(hasOperationItems bool) []commandpalette.Item {
	items := []commandpalette.Item{
		paletteItem(commandIDPluginRefresh, "Refresh command plugins", "Plugins", nil, "reload", "manifests"),
	}
	if m.pluginCommands == nil {
		return items
	}
	for _, manifest := range m.pluginCommands.List() {
		keywords := append([]string{manifest.ID}, manifest.Extensions...)
		item := paletteItem(commandIDPluginRunPrefix+manifest.ID, "Plugin: "+manifest.Name, "Plugins", nil,
			keywords...)
		disablePaletteItem(&item, !hasOperationItems, "no selected item")
		items = append(items, item)
	}
	return items
}

func (m *model) pluginCommandPaths() []string {
	panel := m.getFocusedFilePanel()
	if panel.PanelMode == filepanel.SelectMode && panel.SelectedCount() > 0 {
		return panel.GetSelectedLocationsSortedAsVisible()
	}
	if panel.Empty() {
		return nil
	}
	return []string{panel.GetFocusedItem().Location}
}

func (m *model) runPluginCommand(pluginID string) tea.Cmd {
	if m.pluginCommandCancel != nil {
		m.pluginCommandCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.pluginCommandCancel = cancel
	m.pluginCommandReqID++
	reqID := m.pluginCommandReqID
	paths := append([]string(nil), m.pluginCommandPaths()...)
	cwd := m.getFocusedFilePanel().Location
	registry := m.pluginCommands
	manifest, _ := findPluginManifest(registry, pluginID)
	title := pluginCommandTitle(manifest, pluginID)
	m.textViewer.Open(title, "Running…")
	m.setTextViewerSize()

	return func() tea.Msg {
		if registry == nil {
			return pluginCommandFinishedMsg{
				reqID:    reqID,
				manifest: manifest,
				result:   plugincommands.Result{PluginID: pluginID, ExitCode: -1},
				err:      &plugincommands.NotFoundError{PluginID: pluginID},
			}
		}
		result, err := registry.Execute(ctx, pluginID, paths, cwd)
		return pluginCommandFinishedMsg{reqID: reqID, manifest: manifest, result: result, err: err}
	}
}

func (m *model) applyPluginCommandResult(msg pluginCommandFinishedMsg) {
	if msg.reqID != m.pluginCommandReqID {
		return
	}
	if m.pluginCommandCancel != nil {
		m.pluginCommandCancel()
		m.pluginCommandCancel = nil
	}
	m.textViewer.Open(pluginCommandTitle(msg.manifest, msg.result.PluginID), renderPluginCommandResult(msg))
	m.setTextViewerSize()
	if msg.err != nil {
		m.showUserError("Plugin command failed", sanitizePluginCommandError(msg.err))
	}
}

func (m *model) cancelPluginCommand() {
	if m.pluginCommandCancel == nil {
		return
	}
	m.pluginCommandCancel()
	m.pluginCommandCancel = nil
	m.pluginCommandReqID++
}

type pluginCommandDisplayError struct{ err error }

func (e *pluginCommandDisplayError) Error() string {
	return common.MakePrintableWithEscCheck(e.err.Error(), false)
}

func (e *pluginCommandDisplayError) Unwrap() error { return e.err }

func sanitizePluginCommandError(err error) error {
	if err == nil {
		return nil
	}
	return &pluginCommandDisplayError{err: err}
}

func findPluginManifest(registry *plugincommands.Registry, pluginID string) (plugincommands.Manifest, bool) {
	if registry == nil {
		return plugincommands.Manifest{}, false
	}
	for _, manifest := range registry.List() {
		if manifest.ID == pluginID {
			return manifest, true
		}
	}
	return plugincommands.Manifest{}, false
}

func pluginCommandTitle(manifest plugincommands.Manifest, fallbackID string) string {
	if manifest.Name != "" {
		return "Plugin: " + manifest.Name
	}
	return "Plugin: " + fallbackID
}

func renderPluginCommandResult(msg pluginCommandFinishedMsg) string {
	var builder strings.Builder
	status := "completed"
	if msg.err != nil {
		status = "failed"
	}
	fmt.Fprintf(&builder, "Plugin ID: %s\nStatus: %s\nExit code: %d\nDuration: %s",
		msg.result.PluginID, status, msg.result.ExitCode, msg.result.Duration)
	if msg.err != nil {
		builder.WriteString("\nError: ")
		builder.WriteString(common.MakePrintableWithEscCheck(msg.err.Error(), false))
	}
	output := strings.TrimSpace(common.MakePrintableWithEscCheck(string(msg.result.Output), false))
	builder.WriteString("\n\nOutput:\n")
	if output == "" {
		builder.WriteString("(no output)")
	} else {
		builder.WriteString(output)
	}
	return builder.String()
}
