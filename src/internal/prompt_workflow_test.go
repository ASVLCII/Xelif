package internal

import (
	"fmt"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui/commandpalette"
	"github.com/yorukot/superfile/src/internal/ui/prompt"
	"github.com/yorukot/superfile/src/pkg/utils"
)

func TestPaletteCommandOpensContextualPrompt(t *testing.T) {
	for id, seed := range map[string]string{
		commandIDWorkspacePrompt: "workspace", commandIDSavedSearchPrompt: "search",
		commandIDBatchRenamePrompt: "batch-rename", commandIDCatalogPrompt: "tag",
		commandIDCollectionPrompt: "collection",
		commandIDMetadataPrompt:   "metadata",
	} {
		t.Run(id, func(t *testing.T) {
			m := defaultTestModel(t.TempDir())
			m.applyCommandPaletteAction(commandpalette.ExecuteAction{ID: id})
			require.True(t, m.promptModal.IsOpen())
			assert.False(t, m.promptModal.IsShellMode())
			assert.Contains(t, ansi.Strip(m.promptModal.Render()), seed+" ")
		})
	}
}

func TestPromptPreviewHandsFocusToViewer(t *testing.T) {
	for _, closeOnSuccess := range []bool{false, true} {
		for _, mouse := range []bool{false, true} {
			t.Run(fmt.Sprintf("close=%t/mouse=%t", closeOnSuccess, mouse), func(t *testing.T) {
				root := t.TempDir()
				utils.SetupFiles(t, filepath.Join(root, "file.txt"))
				m := defaultTestModel(root)
				m.promptModal = prompt.GenerateModel(
					common.Hotkeys.OpenSPFPrompt[0], common.Hotkeys.OpenCommandLine[0], closeOnSuccess, 24, 100)
				m.commandPalette = commandpalette.New(
					[]commandpalette.Item{{ID: commandIDBatchRenamePrompt, Label: "Batch rename"}},
				)
				m.setCommandPaletteSize()
				m.commandPalette.Open()
				if mouse {
					x := m.fullWidth/common.CenterDivisor - m.commandPalette.GetWidth()/common.CenterDivisor
					y := m.fullHeight/common.CenterDivisor - m.commandPalette.GetMaxHeight()/common.CenterDivisor
					TeaUpdate(m, clickAt(x+1, y+3, tea.MouseLeft))
				} else {
					TeaUpdate(m, tea.KeyPressMsg{Code: tea.KeyEnter})
				}
				require.True(t, m.promptModal.IsOpen())
				TeaUpdate(m, utils.TeaRuneKeyMsg("prefix done-"))
				TeaUpdate(m, tea.KeyPressMsg{Code: tea.KeyEnter})
				require.NotNil(t, m.pendingBatchRename)
				assert.True(t, m.textViewer.IsOpen())
				assert.False(t, m.promptModal.IsOpen(), "preview must be visible without an extra Escape")
				TeaUpdate(m, tea.KeyPressMsg{Code: tea.KeyEscape})
				assert.False(t, m.textViewer.IsOpen())
				m.applyCommandPaletteAction(commandpalette.ExecuteAction{ID: commandIDBatchRenameCancel})
				assert.Nil(t, m.pendingBatchRename)
			})
		}
	}
}

func TestPromptOrdinarySuccessAndErrorsRespectKeepOpen(t *testing.T) {
	m := defaultTestModel(t.TempDir())
	m.promptModal = prompt.GenerateModel(
		common.Hotkeys.OpenSPFPrompt[0], common.Hotkeys.OpenCommandLine[0], false, 24, 100)
	m.textViewer.Open("Existing preview", "Keep this underneath")
	m.promptModal.OpenCommand("workspace list")
	m.applyPromptModalAction(common.WorkspaceAction{Operation: common.WorkspaceList})
	assert.True(t, m.promptModal.IsOpen())
	m.applyPromptModalAction(common.BatchRenameAction{Operation: common.BatchRenameApply})
	assert.True(t, m.promptModal.IsOpen(), "error stays available for correction")
}

func TestCancelledDuplicateScanCannotReopenResults(t *testing.T) {
	m := defaultTestModel(t.TempDir())
	cmd := m.startDuplicateScan(t.TempDir())
	require.NotNil(t, cmd)
	queued := duplicateScanMsg{reqID: m.duplicateScanReqID}
	m.textViewer.Close()
	m.cancelDuplicateScan()
	m.applyDuplicateScan(queued)
	assert.False(t, m.textViewer.IsOpen())
	assert.False(t, m.spfError.IsOpen())
}
