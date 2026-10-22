package internal

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	variable "github.com/yorukot/superfile/src/config"
	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui/globalsearch"
	"github.com/yorukot/superfile/src/pkg/utils"
)

func TestGlobalSearchHotkeyOpensAtCurrentDirectory(t *testing.T) {
	root := t.TempDir()
	m := defaultTestModel(root)
	require.NotEmpty(t, common.Hotkeys.OpenGlobalSearch)

	m.mainKey(common.Hotkeys.OpenGlobalSearch[0])

	assert.True(t, m.globalSearchModal.IsOpen())
	assert.Equal(t, root, m.globalSearchModal.GetRoot())
}

func TestGlobalSearchDirectoryResultNavigatesFocusedPanel(t *testing.T) {
	root := t.TempDir()
	destination := filepath.Join(root, "destination")
	utils.SetupDirectories(t, destination)
	m := defaultTestModel(root)

	m.applyGlobalSearchAction(globalsearch.OpenResultAction{
		Result: globalsearch.Result{Path: destination, Directory: true},
	})

	assert.Equal(t, destination, m.getFocusedFilePanel().Location)
}

func TestGlobalSearchFileResultHonorsChooserMode(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "chosen.txt")
	chooser := filepath.Join(t.TempDir(), "chooser.txt")
	utils.SetupFiles(t, file)
	originalChooser := variable.ChooserFile
	variable.SetChooserFile(chooser)
	t.Cleanup(func() { variable.SetChooserFile(originalChooser) })
	m := defaultTestModel(root)

	m.applyGlobalSearchAction(globalsearch.OpenResultAction{
		Result: globalsearch.Result{Path: file},
	})

	content, err := os.ReadFile(chooser)
	require.NoError(t, err)
	assert.Equal(t, file, string(content))
}

func TestGlobalSearchEscapeClosesOnlySearch(t *testing.T) {
	root := t.TempDir()
	m := defaultTestModel(root)
	m.globalSearchModal.Open(root)

	TeaUpdate(m, tea.KeyPressMsg{Code: tea.KeyEscape})

	assert.False(t, m.globalSearchModal.IsOpen())
	assert.Equal(t, notQuitting, m.modelQuitState)
}

func TestPrintableGlobalSearchHotkeyCanBeFirstQueryCharacter(t *testing.T) {
	originalHotkeys := append([]string(nil), common.Hotkeys.OpenGlobalSearch...)
	common.Hotkeys.OpenGlobalSearch = []string{"g"}
	t.Cleanup(func() { common.Hotkeys.OpenGlobalSearch = originalHotkeys })
	m := defaultTestModel(t.TempDir())

	TeaUpdate(m, utils.TeaRuneKeyMsg("g"))
	require.True(t, m.globalSearchModal.IsOpen())
	TeaUpdate(m, utils.TeaRuneKeyMsg("g"))

	assert.Equal(t, "g", m.globalSearchModal.GetQuery())
}

func TestGlobalSearchResultClickUsesCenteredOverlayCoordinates(t *testing.T) {
	root := t.TempDir()
	destination := filepath.Join(root, "destination")
	utils.SetupDirectories(t, destination)
	m := defaultTestModel(root)
	m.globalSearchModal = globalsearch.New(func(context.Context, string, string, int) globalsearch.Response {
		return globalsearch.Response{Results: []globalsearch.Result{{Path: destination, Directory: true}}}
	})
	m.setGlobalSearchModelSize()
	m.globalSearchModal.Open(root)
	p := NewTestTeaProgWithEventLoop(t, m)

	p.SendKey("d")
	require.Eventually(t, func() bool {
		return len(m.globalSearchModal.GetResults()) == 1
	}, time.Second, DefaultTestTick)
	overlayX := m.fullWidth/common.CenterDivisor - m.globalSearchModal.GetWidth()/common.CenterDivisor
	overlayY := m.fullHeight/common.CenterDivisor - m.globalSearchModal.GetMaxHeight()/common.CenterDivisor
	p.Send(clickAt(overlayX+1, overlayY+3, tea.MouseLeft))

	assert.Eventually(t, func() bool {
		return m.getFocusedFilePanel().Location == destination
	}, time.Second, DefaultTestTick)
	assert.False(t, m.globalSearchModal.IsOpen())
}

func TestGlobalSearchOutsideClickDismissesWithoutReachingBackground(t *testing.T) {
	root := t.TempDir()
	utils.SetupFiles(t, filepath.Join(root, "a.txt"), filepath.Join(root, "b.txt"))
	m := defaultTestModel(root)
	panel := m.getFocusedFilePanel()
	target := panel.FindElementIndexByName("b.txt")
	x, y := fileEntryScreenPosition(m, target)
	m.globalSearchModal.Open(root)

	TeaUpdate(m, clickAt(x, y, tea.MouseLeft))

	assert.Equal(t, 0, panel.GetCursor())
	assert.False(t, m.globalSearchModal.IsOpen())
}
