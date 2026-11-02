package common

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestPortableNamesDistinguishFoldersAndLinksWithoutGlyphs(t *testing.T) {
	original := Config
	t.Cleanup(func() { SetConfig(original) })
	Config.PlainMode, Config.Nerdfont = true, false
	for _, item := range []struct {
		name            string
		directory, link bool
		want            string
	}{
		{"Documents", true, false, "Documents/"},
		{"notes.txt", false, false, "notes.txt"},
		{"shortcut", false, true, "shortcut@"},
	} {
		output := ansi.Strip(
			FilePanelItemRenderWithIcon(item.name, 40, item.directory, item.link, false, lipgloss.Color("#000000")),
		)
		require.Contains(t, output, item.want)
		for _, r := range output {
			require.Less(t, r, rune(128))
		}
	}
	output := ansi.Strip(
		FilePanelItemRenderWithIcon("very-long-folder-name", 8, true, false, false, lipgloss.Color("#000000")),
	)
	require.True(t, strings.HasSuffix(strings.TrimSpace(output), "/"))
	require.LessOrEqual(t, ansi.StringWidth(output), 8)
}
