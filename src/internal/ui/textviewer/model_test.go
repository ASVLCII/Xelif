package textviewer

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/common"
)

func setupViewerTheme() {
	common.Config.BorderTop = "─"
	common.Config.BorderBottom = "─"
	common.Config.BorderLeft = "│"
	common.Config.BorderRight = "│"
	common.Config.BorderTopLeft = "╭"
	common.Config.BorderTopRight = "╮"
	common.Config.BorderBottomLeft = "╰"
	common.Config.BorderBottomRight = "╯"
	common.Config.BorderMiddleLeft = "├"
	common.Config.BorderMiddleRight = "┤"
}

func TestViewerScrollsAndClamps(t *testing.T) {
	setupViewerTheme()
	m := New()
	m.SetDimensions(30, 6)
	m.Open("Git diff", "one\ntwo\nthree\nfour\nfive\nsix")

	first := ansi.Strip(m.Render())
	assert.Contains(t, first, "one")
	assert.NotContains(t, first, "six")
	m.End()
	last := ansi.Strip(m.Render())
	assert.Contains(t, last, "six")
	assert.NotContains(t, last, "one")
	m.Scroll(-100)
	assert.Contains(t, ansi.Strip(m.Render()), "one")
}

func TestViewerDimensionsAndClickClose(t *testing.T) {
	setupViewerTheme()
	m := New()
	m.SetDimensions(2, 2)
	m.Open("", "details")
	render := ansi.Strip(m.Render())
	require.Equal(t, minimumHeight, m.GetHeight())
	assert.GreaterOrEqual(t, len(strings.Split(render, "\n")), 3)
	assert.False(t, m.HandleClick(13, minimumHeight-1))
	assert.True(t, m.HandleClick(2, minimumHeight-1))
	assert.False(t, m.IsOpen())
}
