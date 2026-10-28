package internal

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui/notify"
	"github.com/yorukot/superfile/src/internal/ui/sortmodel"
)

func renderedLabelPosition(t *testing.T, rendered, label string) (int, int) {
	t.Helper()
	for y, line := range strings.Split(ansi.Strip(rendered), "\n") {
		index := strings.Index(line, ansi.Strip(label))
		if index >= 0 {
			return ansi.StringWidth(line[:index]), y
		}
	}
	t.Fatalf("label %q missing from rendered modal", label)
	return 0, 0
}

func TestMouseConfirmationUsesRenderedButtonsWithoutClickThrough(t *testing.T) {
	m := defaultTestModel(t.TempDir())
	m.notifyModel = notify.New(true, "Confirm", "Example", notify.GitDiscardAction)
	m.pendingGitDiscard = []string{"example"}
	// Outside clicks cannot confirm or leak into the underlying file panel.
	TeaUpdate(m, clickAt(0, 0, tea.MouseLeft))
	require.True(t, m.notifyModel.IsOpen())
	label := common.ModalCancelInputText
	require.NotEmpty(t, label)
	x, y := renderedLabelPosition(t, m.notifyModel.Render(), label)
	x += m.fullWidth/2 - common.ModalWidth/2
	y += m.fullHeight/2 - common.ModalHeight/2
	TeaUpdate(m, clickAt(x, y, tea.MouseLeft))
	assert.False(t, m.notifyModel.IsOpen())
	assert.Empty(t, m.pendingGitDiscard)
}

func TestMouseCanChooseSortAndDismissSearch(t *testing.T) {
	m := defaultTestModel(t.TempDir())
	m.sortModal.Open(sortmodel.SortByName)
	x, y := renderedLabelPosition(t, m.sortModal.Render(), "Size")
	left, top := m.sortMenuPosition()
	x += left
	y += top
	TeaUpdate(m, clickAt(x, y, tea.MouseLeft))
	assert.False(t, m.sortModal.IsOpen())
	assert.Equal(t, sortmodel.SortBySize, m.getFocusedFilePanel().SortKind)
	_ = m.globalSearchModal.Open(m.getFocusedFilePanel().Location)
	TeaUpdate(m, clickAt(0, 0, tea.MouseLeft))
	assert.False(t, m.globalSearchModal.IsOpen())
}

func TestRenderedLabelHitUsesTerminalCellsForUnicode(t *testing.T) {
	assert.True(t, renderedLabelHit("│ 目錄 [Confirm] │", "[Confirm]", 7, 0))
	assert.False(t, renderedLabelHit("│ 目錄 [Confirm] │", "[Confirm]", 3, 0))
	assert.False(t, renderedLabelHit("[Confirm]", "[Confirm]", 0, 1))
	assert.False(t, renderedLabelHit("File named [Confirm]\n[Confirm]", "[Confirm]", 11, 0))
	assert.True(t, renderedLabelHit("File named [Confirm]\n[Confirm]", "[Confirm]", 0, 1))
}

func TestMouseDoesNotActivateSortBehindSearch(t *testing.T) {
	m := defaultTestModel(t.TempDir())
	m.sortModal.Open(sortmodel.SortByName)
	_ = m.globalSearchModal.Open(m.getFocusedFilePanel().Location)
	_, handled := m.handleModalMouseClick(tea.Mouse{X: 0, Y: 0, Button: tea.MouseLeft})
	assert.False(t, handled)
	assert.True(t, m.sortModal.IsOpen())
	assert.True(t, m.globalSearchModal.IsOpen())
}

func TestMouseCanCancelSortOutsideWithoutChangingOrder(t *testing.T) {
	m := defaultTestModel(t.TempDir())
	before := m.getFocusedFilePanel().SortKind
	m.sortModal.Open(before)
	TeaUpdate(m, clickAt(0, 0, tea.MouseLeft))
	assert.False(t, m.sortModal.IsOpen())
	assert.Equal(t, before, m.getFocusedFilePanel().SortKind)
}
