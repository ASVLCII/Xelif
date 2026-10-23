package common

import (
	"strconv"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestFilePanelCellMatchesWrappedRendering(t *testing.T) {
	original, selected := FilePanelStyle, FilePanelItemSelectedStyle
	t.Cleanup(func() {
		FilePanelStyle, FilePanelItemSelectedStyle = original, selected
	})
	styles := []lipgloss.Style{
		lipgloss.NewStyle(),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#abcdef")),
		lipgloss.NewStyle().Bold(true).Underline(true),
		lipgloss.NewStyle().Padding(1, 2),
		lipgloss.NewStyle().Margin(1),
		lipgloss.NewStyle().Border(lipgloss.NormalBorder()),
		lipgloss.NewStyle().Transform(func(s string) string { return s + "\nextra" }),
	}
	inputs := []string{
		"", "plain", "a b  ", "\tname", "first\nsecond", "first\r\nsecond",
		"文件 tài-liệu", "👩‍💻e\u0301", "\x1b[31mred\x1b[0m",
		"\x1b]8;;https://example.com\x1b\\link\x1b]8;;\x1b\\", "\xff\xfe",
		"\x1b[31munclosed", "\x1b]8;;https://example.com\x1b\\unclosed",
		"ab\ac", "ab\bc", "ab\x7fc",
		"\u00a0\u2028\u3000tail", strings.Repeat("long ", 20),
	}
	for index, style := range styles {
		t.Run(strconv.Itoa(index), func(t *testing.T) {
			FilePanelStyle, FilePanelItemSelectedStyle = style, style.Reverse(true)
			for _, input := range inputs {
				checkCellRenderInput(t, input)
			}
		})
	}
}

func checkCellRenderInput(t *testing.T, input string) {
	t.Helper()
	for _, width := range []int{-1, 0, 1, 2, 3, 5, 20, 80} {
		for _, alignment := range []lipgloss.Position{lipgloss.Left, lipgloss.Center, lipgloss.Right} {
			for _, selected := range []bool{false, true} {
				checkCellRenderCase(t, input, width, alignment, selected)
			}
		}
	}
}

func checkCellRenderCase(t *testing.T, input string, width int, alignment lipgloss.Position, selected bool) {
	t.Helper()
	bg := lipgloss.Color("#123456")
	style := FilePanelStyle
	var expected string
	if selected {
		style = FilePanelItemSelectedStyle
		expected = style.Background(filePanelItemSelectedBGColor).Width(width).Align(alignment).
			Render(ansi.Truncate(input, width, "..."))
	} else {
		expected = style.Background(bg).Width(width).Align(alignment).Render(ansi.Truncate(input, width, "..."))
	}
	if actual := FilePanelItemRender(input, width, selected, bg, alignment); actual != expected {
		t.Fatalf("cell output changed for %q width=%d alignment=%v selected=%t\ngot %q\nwant %q",
			input, width, alignment, selected, actual, expected)
	}
}
