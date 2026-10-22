package internal

import (
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/pkg/utils"
)

func TestTabHotkeysCreateAndClosePaneLocalTab(t *testing.T) {
	m := defaultTestModel(t.TempDir())
	panel := m.getFocusedFilePanel()
	require.NotEmpty(t, common.Hotkeys.NewFileTab)
	require.NotEmpty(t, common.Hotkeys.CloseFileTab)

	m.mainKey(common.Hotkeys.NewFileTab[0])
	assert.Equal(t, 2, panel.TabCount())

	m.mainKey(common.Hotkeys.CloseFileTab[0])
	assert.Equal(t, 1, panel.TabCount())
}

func TestHistoryHotkeysNavigateBackAndForward(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	grandchild := filepath.Join(child, "grandchild")
	utils.SetupDirectories(t, grandchild)
	m := defaultTestModel(root)
	require.NoError(t, m.updateCurrentFilePanelDir(child))
	require.NoError(t, m.updateCurrentFilePanelDir(grandchild))
	require.NotEmpty(t, common.Hotkeys.HistoryBack)
	require.NotEmpty(t, common.Hotkeys.HistoryForward)

	m.mainKey(common.Hotkeys.HistoryBack[0])
	assert.Equal(t, child, m.getFocusedFilePanel().Location)

	m.mainKey(common.Hotkeys.HistoryForward[0])
	assert.Equal(t, grandchild, m.getFocusedFilePanel().Location)
}

func TestClickingInlineTabActivatesIt(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	utils.SetupDirectories(t, child)
	m := defaultTestModel(root)
	panel := m.getFocusedFilePanel()
	require.NoError(t, panel.NewTab(child))
	require.Equal(t, 1, panel.ActiveTabIndex())
	contentX := -1
	for x := range panel.GetContentWidth() {
		if index, ok := panel.TabAtContentX(x); ok && index == 0 {
			contentX = x
			break
		}
	}
	require.NotEqual(t, -1, contentX)
	panelX := m.sidebarModel.GetWidth()

	TeaUpdate(m, tea.MouseClickMsg{X: panelX + 1 + contentX, Y: 1, Button: tea.MouseLeft})

	assert.Equal(t, 0, panel.ActiveTabIndex())
	assert.Equal(t, root, panel.Location)
}
