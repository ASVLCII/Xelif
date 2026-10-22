package commandpalette

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
)

func TestDimensionsClampToMinimums(t *testing.T) {
	t.Parallel()

	m := New(nil)
	m.SetWidth(1)
	m.SetMaxHeight(1)

	assert.Equal(t, MinWidth, m.GetWidth())
	assert.Equal(t, MinHeight, m.GetMaxHeight())
}

func TestRenderShowsItemsCategoriesEffectiveHotkeysAndDisabledReason(t *testing.T) {
	t.Parallel()

	m := New([]Item{
		{ID: "copy", Label: "Copy path", Category: "Files", Hotkeys: []string{"ctrl+c", "", "ctrl+c", "y"}},
		{ID: "delete", Label: "Delete", Category: "Files", Disabled: true, DisabledReason: "read-only mode"},
	})
	m.SetWidth(80)
	m.SetMaxHeight(12)
	_ = m.Open()

	output := ansi.Strip(m.Render())

	assert.Contains(t, output, "Command Palette")
	assert.Contains(t, output, "Copy path")
	assert.Contains(t, output, "Files")
	assert.Contains(t, output, "ctrl+c, y")
	assert.NotContains(t, output, "ctrl+c, ctrl+c")
	assert.Contains(t, output, "Delete")
	assert.Contains(t, output, "disabled: read-only mode")
}

func TestRenderShowsQueryAndEmptyResultState(t *testing.T) {
	t.Parallel()

	m := New(testItems())
	_ = m.Open()
	m.textInput.SetValue("missing")
	m.refilter()

	output := ansi.Strip(m.Render())

	assert.Contains(t, output, "missing")
	assert.Contains(t, output, "No matching commands")
}

func TestRenderUsesDefaultReasonForDisabledItem(t *testing.T) {
	t.Parallel()

	m := New([]Item{{ID: "disabled", Label: "Disabled", Disabled: true}})
	_ = m.Open()

	assert.Contains(t, ansi.Strip(m.Render()), "disabled: unavailable")
}

func TestRenderIsHeightBoundedAndShowsOnlyScrolledWindow(t *testing.T) {
	t.Parallel()

	items := []Item{
		{ID: "zero", Label: "Zero"},
		{ID: "one", Label: "One"},
		{ID: "two", Label: "Two"},
		{ID: "three", Label: "Three"},
	}
	m := New(items)
	m.SetWidth(50)
	m.SetMaxHeight(MinHeight)
	_ = m.Open()
	m.cursor = 2
	m.renderIndex = 1

	output := ansi.Strip(m.Render())

	assert.LessOrEqual(t, len(strings.Split(output, "\n")), MinHeight)
	assert.NotContains(t, output, "Zero")
	assert.Contains(t, output, "One")
	assert.Contains(t, output, "Two")
	assert.NotContains(t, output, "Three")
}
