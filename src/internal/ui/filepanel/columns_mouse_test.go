package filepanel

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui/sortmodel"
)

func TestApplySortColumnAtContentXSortsAndPreservesFocusedPath(t *testing.T) {
	originalConfig := common.Config
	common.Config.FilePanelExtraColumns = 2
	common.Config.FilePanelNamePercent = 50
	t.Cleanup(func() { common.SetConfig(originalConfig) })

	panel := testModel(0, 0, 12, BrowserMode, []Element{
		{Name: "b.txt", Location: "/tmp/b.txt"},
		{Name: "a.txt", Location: "/tmp/a.txt"},
	})
	panel.SortKind = sortmodel.SortBySize
	panel.SortReversed = true
	panel.SetWidth(100)

	require.True(t, panel.ApplySortColumnAtContentX(0))
	assert.Equal(t, sortmodel.SortByName, panel.SortKind)
	assert.False(t, panel.SortReversed)
	assert.Equal(t, []string{"a.txt", "b.txt"}, []string{panel.element[0].Name, panel.element[1].Name})
	assert.Equal(t, 1, panel.GetCursor())
}

func TestApplySortColumnAtContentXTogglesCurrentColumnDirection(t *testing.T) {
	originalConfig := common.Config
	common.Config.FilePanelExtraColumns = 1
	common.Config.FilePanelNamePercent = 50
	t.Cleanup(func() { common.SetConfig(originalConfig) })

	panel := testModel(0, 0, 12, BrowserMode, []Element{
		{Name: "a.txt", Location: "/tmp/a.txt"},
		{Name: "b.txt", Location: "/tmp/b.txt"},
	})
	panel.SortKind = sortmodel.SortByName
	panel.SetWidth(80)

	require.True(t, panel.ApplySortColumnAtContentX(0))
	assert.True(t, panel.SortReversed)
	assert.Equal(t, []string{"b.txt", "a.txt"}, []string{panel.element[0].Name, panel.element[1].Name})
	assert.Equal(t, 1, panel.GetCursor())
}

func TestApplySortColumnAtContentXUsesSizeColumn(t *testing.T) {
	originalConfig := common.Config
	common.Config.FilePanelExtraColumns = 1
	common.Config.FilePanelNamePercent = 50
	t.Cleanup(func() { common.SetConfig(originalConfig) })

	dir := t.TempDir()
	largePath := filepath.Join(dir, "large.txt")
	smallPath := filepath.Join(dir, "small.txt")
	require.NoError(t, os.WriteFile(largePath, []byte("large"), 0o600))
	require.NoError(t, os.WriteFile(smallPath, []byte("s"), 0o600))
	largeInfo, err := os.Stat(largePath)
	require.NoError(t, err)
	smallInfo, err := os.Stat(smallPath)
	require.NoError(t, err)

	panel := testModel(0, 0, 12, BrowserMode, []Element{
		{Name: "large.txt", Location: largePath, Info: largeInfo},
		{Name: "small.txt", Location: smallPath, Info: smallInfo},
	})
	panel.SortKind = sortmodel.SortByName
	panel.SetWidth(100)
	sizeColumnX := panel.columns[0].Size + panel.columns[1].Size

	require.True(t, panel.ApplySortColumnAtContentX(sizeColumnX))
	assert.Equal(t, sortmodel.SortBySize, panel.SortKind)
	assert.False(t, panel.SortReversed)
	assert.Equal(t, []string{"small.txt", "large.txt"}, []string{panel.element[0].Name, panel.element[1].Name})
	assert.Equal(t, 1, panel.GetCursor())
}

func TestApplySortColumnAtContentXUsesModifyTimeColumn(t *testing.T) {
	originalConfig := common.Config
	common.Config.FilePanelExtraColumns = 2
	common.Config.FilePanelNamePercent = 40
	t.Cleanup(func() { common.SetConfig(originalConfig) })

	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old.txt")
	newPath := filepath.Join(dir, "new.txt")
	require.NoError(t, os.WriteFile(oldPath, nil, 0o600))
	require.NoError(t, os.WriteFile(newPath, nil, 0o600))
	oldTime := time.Now().Add(-2 * time.Hour)
	require.NoError(t, os.Chtimes(oldPath, oldTime, oldTime))
	oldInfo, err := os.Stat(oldPath)
	require.NoError(t, err)
	newInfo, err := os.Stat(newPath)
	require.NoError(t, err)

	panel := testModel(0, 0, 12, BrowserMode, []Element{
		{Name: "old.txt", Location: oldPath, Info: oldInfo},
		{Name: "new.txt", Location: newPath, Info: newInfo},
	})
	panel.SortKind = sortmodel.SortByName
	panel.SetWidth(120)
	modifyColumnX := 0
	for i := range 4 {
		modifyColumnX += panel.columns[i].Size
	}

	require.True(t, panel.ApplySortColumnAtContentX(modifyColumnX))
	assert.Equal(t, sortmodel.SortByDate, panel.SortKind)
	assert.False(t, panel.SortReversed)
	assert.Equal(t, []string{"new.txt", "old.txt"}, []string{panel.element[0].Name, panel.element[1].Name})
	assert.Equal(t, 1, panel.GetCursor())
}

func TestSortKindAtContentXDistinguishesSortableColumnsFromDelimiters(t *testing.T) {
	originalConfig := common.Config
	common.Config.FilePanelExtraColumns = 1
	common.Config.FilePanelNamePercent = 50
	t.Cleanup(func() { common.SetConfig(originalConfig) })

	panel := Model{width: 100}
	panel.SetWidth(100)
	delimiterX := panel.columns[0].Size
	sizeColumnX := delimiterX + panel.columns[1].Size

	sortKind, ok := panel.SortKindAtContentX(sizeColumnX)
	require.True(t, ok)
	assert.Equal(t, sortmodel.SortBySize, sortKind)
	_, ok = panel.SortKindAtContentX(delimiterX)
	assert.False(t, ok)
}
