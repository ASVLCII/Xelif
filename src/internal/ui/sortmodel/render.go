package sortmodel

import (
	"image/color"
	"os"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/yorukot/superfile/src/internal/common"
)

func (m *Model) Render() string {
	innerWidth := m.Width - common.BorderPadding
	lines := make([]string, 0, sortMenuRowCount)
	for index, label := range SortOptionsStr {
		if index == sortFieldCount {
			lines = append(lines, common.ModalStyle.Render(strings.Repeat("─", innerWidth)))
		}
		checked := index < sortFieldCount && menuKinds[index] == m.SelectedKind ||
			index == sortFieldCount && !m.Reversed ||
			index == sortFieldCount+1 && m.Reversed
		prefix := "  "
		if checked {
			prefix = "● "
		}
		style := common.ModalStyle.Width(innerWidth)
		if index == m.Cursor {
			style = ControlStyle().Width(innerWidth)
		}
		if _, noColor := os.LookupEnv("NO_COLOR"); noColor {
			marker := " "
			if index == m.Cursor {
				marker = ">"
			}
			prefix = marker + prefix
		}
		lines = append(lines, style.Render(prefix+label))
	}
	return common.SortOptionsModalBorderStyle(m.Height, m.Width, "─").
		Border(lipgloss.NormalBorder()).Render(strings.Join(lines, "\n"))
}

// ControlStyle gives the toolbar and focused row a quiet, theme-relative surface.
func ControlStyle() lipgloss.Style {
	return controlStyle(common.ModalBGColor)
}

func controlStyle(background color.Color) lipgloss.Style {
	style := common.ModalStyle
	if _, noColor := os.LookupEnv("NO_COLOR"); noColor {
		return style
	}
	bg := color.NRGBA{
		R: defaultSurfaceChannel,
		G: defaultSurfaceChannel,
		B: defaultSurfaceChannel,
		A: maxSurfaceChannel,
	}
	if background != nil {
		if converted, ok := color.NRGBAModel.Convert(background).(color.NRGBA); ok {
			bg = converted
		}
	}
	delta := defaultSurfaceChannel
	if int(bg.R)+int(bg.G)+int(bg.B) > lightSurfaceThreshold {
		delta = lightSurfaceDelta
	}
	bg.R, bg.G, bg.B = shiftSurfaceChannel(
		bg.R,
		delta,
	), shiftSurfaceChannel(
		bg.G,
		delta,
	), shiftSurfaceChannel(
		bg.B,
		delta,
	)
	bg.A = maxSurfaceChannel
	return style.Background(bg)
}

func shiftSurfaceChannel(channel uint8, delta int) uint8 {
	value := int(channel) + delta
	if value < 0 {
		return 0
	}
	if value > maxSurfaceChannel {
		return maxSurfaceChannel
	}
	return uint8(value)
}
