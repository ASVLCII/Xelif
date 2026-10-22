package internal

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/catalog"
	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/pkg/utils"
)

func TestCatalogLoadFailurePreservesStoredBytes(t *testing.T) {
	for _, content := range []string{"not JSON", `{"version":99,"paths":[],"collections":[]}`} {
		t.Run(content, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "catalog.json")
			require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
			m := defaultTestModel(root)
			m.catalogModel = loadCatalog(path)
			require.Nil(t, m.catalogModel)
			_, _, err := m.executeCatalogAction(common.CatalogAction{Operation: common.TagAdd, Name: "test"})
			require.ErrorContains(t, err, "preserved")
			m.updateCatalogMoves([]pathMove{{from: path, to: path + ".moved"}})
			data, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, content, string(data))
		})
	}
}

func TestMissingCatalogCanInitializeAndPersist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog.json")
	c := loadCatalog(path)
	require.NotNil(t, c)
	require.NoError(t, c.AddTag(filepath.Join(t.TempDir(), "file.txt"), "work"))
	require.NoError(t, c.Save())
	loaded, err := catalog.Open(path)
	require.NoError(t, err)
	assert.Equal(t, []string{"work"}, loaded.ListTags())
}

func TestCatalogResultsRevealAnyMemberWithKeyboardAndMouse(t *testing.T) {
	for _, mouse := range []bool{false, true} {
		t.Run(map[bool]string{false: "keyboard", true: "mouse"}[mouse], func(t *testing.T) {
			root := t.TempDir()
			first := filepath.Join(root, "one", "a.txt")
			second := filepath.Join(root, "two", "b.txt")
			utils.SetupDirectories(t, filepath.Dir(first), filepath.Dir(second))
			utils.SetupFiles(t, first, second)
			m := defaultTestModel(root)
			m.catalogModel = loadCatalog(filepath.Join(t.TempDir(), "catalog.json"))
			require.NoError(t, m.catalogModel.CreateCollection("work", []string{first, second}))
			_, _, err := m.executeCatalogAction(common.CatalogAction{Operation: common.CollectionOpen, Name: "work"})
			require.NoError(t, err)
			require.True(t, m.globalSearchModal.IsOpen())
			assert.Equal(t, root, m.getFocusedFilePanel().Location, "opening results must not jump to the first path")
			if mouse {
				x := m.fullWidth/common.CenterDivisor - m.globalSearchModal.GetWidth()/common.CenterDivisor
				y := m.fullHeight/common.CenterDivisor - m.globalSearchModal.GetMaxHeight()/common.CenterDivisor
				TeaUpdate(m, clickAt(x+1, y+4, tea.MouseLeft))
			} else {
				TeaUpdate(m, tea.KeyPressMsg{Code: tea.KeyDown})
				TeaUpdate(m, tea.KeyPressMsg{Code: tea.KeyEnter})
			}
			assert.False(t, m.globalSearchModal.IsOpen())
			assert.Equal(t, filepath.Dir(second), m.getFocusedFilePanel().Location)
			assert.Equal(t, second, m.getFocusedFilePanel().GetFocusedItem().Location)
			m.mainKey(common.Hotkeys.HistoryBack[0])
			assert.Equal(t, root, m.getFocusedFilePanel().Location)
		})
	}
}

func TestCatalogMissingResultRetainsPickerAndOtherResults(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "a-missing.txt")
	existing := filepath.Join(root, "b-present.txt")
	utils.SetupFiles(t, existing)
	m := defaultTestModel(root)
	m.openCatalogResults("Collection: work", []string{missing, existing})
	TeaUpdate(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.Contains(t, m.globalSearchModal.Render(), "Could not reveal")
	assert.False(t, m.notifyModel.IsOpen())
	assert.True(t, m.globalSearchModal.IsOpen())
	assert.Equal(t, root, m.getFocusedFilePanel().Location)
	TeaUpdate(m, tea.KeyPressMsg{Code: tea.KeyDown})
	TeaUpdate(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.False(t, m.globalSearchModal.IsOpen())
	assert.Equal(t, existing, m.getFocusedFilePanel().GetFocusedItem().Location)
}

func TestCatalogHiddenResultDoesNotPretendAnotherFileIsSelected(t *testing.T) {
	root := t.TempDir()
	hidden := filepath.Join(root, ".hidden.txt")
	visible := filepath.Join(root, "visible.txt")
	utils.SetupFiles(t, hidden, visible)
	m := defaultTestModel(root)
	m.fileModel.DisplayDotFiles = false
	m.fileModel.UpdateFilePanelsIfNeeded(true)
	m.openCatalogResults("Tag: hidden", []string{hidden})
	TeaUpdate(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.True(t, m.globalSearchModal.IsOpen())
	assert.Contains(t, m.globalSearchModal.Render(), "hidden")
	assert.Equal(t, visible, m.getFocusedFilePanel().GetFocusedItem().Location)
}

func TestCatalogFilteredResultRetainsSameDirectoryAndFilter(t *testing.T) {
	root := t.TempDir()
	excluded := filepath.Join(root, "a-excluded.txt")
	visible := filepath.Join(root, "b-visible.txt")
	utils.SetupFiles(t, excluded, visible)
	m := defaultTestModel(root)
	panel := m.getFocusedFilePanel()
	panel.SearchBar.SetValue("b-visible")
	panel.SearchBar.Blur()
	panel.UpdateElementsIfNeeded(true, m.fileModel.DisplayDotFiles)
	require.Equal(t, 1, panel.ElemCount())
	require.Equal(t, visible, panel.GetFocusedItem().Location)
	displayDotFiles := m.fileModel.DisplayDotFiles

	m.openCatalogResults("Collection: filtered", []string{excluded, visible})
	TeaUpdate(m, utils.TeaRuneKeyMsg("txt"))
	require.Len(t, m.globalSearchModal.GetResults(), 2)
	TeaUpdate(m, tea.KeyPressMsg{Code: tea.KeyEnter})

	assert.True(t, m.globalSearchModal.IsOpen(), "an excluded file has not been revealed")
	assert.Contains(t, m.globalSearchModal.Render(), "Could not reveal saved path")
	assert.Equal(t, "txt", m.globalSearchModal.GetQuery())
	assert.False(t, m.notifyModel.IsOpen(), "the error must remain visible in the picker")
	assert.Equal(t, root, panel.Location)
	assert.Equal(t, "b-visible", panel.SearchBar.Value())
	assert.Equal(t, displayDotFiles, m.fileModel.DisplayDotFiles)
	assert.Equal(t, visible, panel.GetFocusedItem().Location)

	TeaUpdate(m, tea.KeyPressMsg{Code: tea.KeyDown})
	TeaUpdate(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.False(t, m.globalSearchModal.IsOpen(), "another visible saved result remains usable")
	assert.Equal(t, visible, panel.GetFocusedItem().Location)
	assert.Equal(t, "b-visible", panel.SearchBar.Value())
}
