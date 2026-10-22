package rendering

import (
	"strings"

	"github.com/charmbracelet/x/ansi"

	"charm.land/lipgloss/v2"
)

type BorderConfig struct {
	// ANSI encoded strings are not allowed in border title and info items, for now.
	// The style is overridden with border's style.
	title string

	// Optional info items at the bottom of the border
	infoItems []string

	// Section dividers - A slice of values within [0,height-2]
	// Signifying usage of MiddleLeft and MiddleRight borders in Left and Right borders for
	// Section divider line.
	dividerIdx []int

	// Including corners. Both should be >= 2
	width  int
	height int

	titleLeftMargin int
}

type borderInfoItemRange struct {
	start int
	end   int
}

func NewBorderConfig(height int, width int) BorderConfig {
	return BorderConfig{
		height:          height,
		width:           width,
		titleLeftMargin: 1,
	}
}

func (b *BorderConfig) SetTitle(title string) {
	b.title = ansi.Strip(title)
}

func (b *BorderConfig) SetInfoItems(infoItems ...string) {
	for i := range infoItems {
		infoItems[i] = ansi.Strip(infoItems[i])
	}
	b.infoItems = infoItems
}

func (b *BorderConfig) AreInfoItemsTruncated() bool {
	cnt := len(b.infoItems)
	if cnt == 0 {
		return false
	}

	actualWidth := b.width - borderCornerWidth
	// border.MiddleLeft <content> border.MiddleRight border.Bottom
	availWidth := actualWidth/cnt - borderDividerWidth
	for i := range b.infoItems {
		if ansi.StringWidth(b.infoItems[i]) > availWidth {
			return true
		}
	}
	return false
}

// BorderInfoItemAtX returns the info-item index rendered at a bottom-border X
// coordinate. Coordinates include the outer border corners.
func BorderInfoItemAtX(width int, infoItems []string, x int) (int, bool) {
	_, _, ranges, ok := layoutBorderInfoItems(width, infoItems)
	if !ok {
		return 0, false
	}
	for i, itemRange := range ranges {
		if x >= itemRange.start && x < itemRange.end {
			return i, true
		}
	}
	return 0, false
}

func layoutBorderInfoItems(width int, infoItems []string) ([]string, int, []borderInfoItemRange, bool) {
	cnt := len(infoItems)
	actualWidth := width - borderCornerWidth
	if cnt == 0 || actualWidth < cnt*minInfoItemWidth {
		return nil, 0, nil, false
	}

	availWidth := actualWidth/cnt - borderDividerWidth
	truncatedItems := make([]string, len(infoItems))
	infoWidth := 0
	for i, item := range infoItems {
		truncatedItems[i] = ansi.Truncate(item, availWidth, "")
		infoWidth += ansi.StringWidth(truncatedItems[i]) + borderDividerWidth
	}

	remainingWidth := actualWidth - infoWidth
	ranges := make([]borderInfoItemRange, len(truncatedItems))
	start := 1 + remainingWidth
	for i, item := range truncatedItems {
		end := start + ansi.StringWidth(item) + borderDividerWidth
		ranges[i] = borderInfoItemRange{start: start, end: end}
		start = end
	}
	return truncatedItems, remainingWidth, ranges, true
}

func (b *BorderConfig) AddDivider(idx int) {
	b.dividerIdx = append(b.dividerIdx, idx)
}

// border.Top with something that takes up more than 1 runewidth will not work, so
// we only allow 1 runewidth for now, in the config. multiple things like
// border corner characters must be single rune, or else it would break rendering.
// This is all filled in one function to prevent passing around too many values
// in helper functions
func (b *BorderConfig) GetBorder(borderStrings lipgloss.Border) lipgloss.Border {
	res := borderStrings

	// excluding corners. Maybe we can move this to a utility function
	actualWidth := b.width - borderCornerWidth
	actualHeight := b.height - borderCornerWidth

	// Min 5 width is needed for title so that at least one character can be
	// rendered
	if b.title != "" && actualWidth >= minTitleWidth {
		// We need to plain truncate the title if needed.
		// topWidth - 1( for BorderMiddleLeft) - 1 (for BorderMiddleRight) - 2 (padding)
		titleAvailWidth := actualWidth - borderCornerWidth - borderPaddingWidth

		// Basic Right truncation
		truncatedTitle := ansi.Truncate(b.title, titleAvailWidth, "")
		remainingWidth := actualWidth - borderCornerWidth - borderPaddingWidth - ansi.StringWidth(truncatedTitle)

		margin := ""
		if remainingWidth > b.titleLeftMargin {
			margin = strings.Repeat(borderStrings.Top, b.titleLeftMargin)
			remainingWidth -= b.titleLeftMargin
		}

		// Title alignment is by default Left for now
		res.Top = margin + borderStrings.MiddleRight + " " + truncatedTitle + " " + borderStrings.MiddleLeft +
			strings.Repeat(borderStrings.Top, remainingWidth)
	}

	truncatedItems, remainingWidth, _, renderInfoItems := layoutBorderInfoItems(b.width, b.infoItems)
	if renderInfoItems {
		var infoText strings.Builder
		for _, item := range truncatedItems {
			infoText.WriteString(
				borderStrings.MiddleRight + item + borderStrings.MiddleLeft + borderStrings.Bottom,
			)
		}

		res.Bottom = strings.Repeat(borderStrings.Bottom, remainingWidth) + infoText.String()
	}

	if len(b.dividerIdx) > 0 {
		// Update res.Left and res.Right
		leftBorder := strings.Builder{}
		rightBorder := strings.Builder{}
		di := 0
		for i := range actualHeight {
			if di < len(b.dividerIdx) && b.dividerIdx[di] == i {
				di++
				leftBorder.WriteString(borderStrings.MiddleLeft)
				rightBorder.WriteString(borderStrings.MiddleRight)
			} else {
				leftBorder.WriteString(borderStrings.Left)
				rightBorder.WriteString(borderStrings.Right)
			}
		}

		res.Left = leftBorder.String()
		res.Right = rightBorder.String()
	}

	return res
}
