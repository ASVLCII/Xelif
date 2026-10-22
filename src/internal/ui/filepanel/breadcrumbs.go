package filepanel

import (
	"path/filepath"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/yorukot/superfile/src/internal/common"
)

const breadcrumbSeparator = " > "
const truncatedBreadcrumbPrefix = "… > "

// BreadcrumbSpan identifies the terminal-cell range and destination of a
// rendered breadcrumb.
type BreadcrumbSpan struct {
	Start int
	End   int
	Path  string
}

type breadcrumbPart struct {
	label string
	path  string
}

func renderBreadcrumbs(location string, maxWidth int) (string, []BreadcrumbSpan) {
	if maxWidth <= 0 {
		return "", nil
	}
	cleanLocation := filepath.Clean(location)
	parts := make([]breadcrumbPart, 0)

	for current := cleanLocation; ; current = filepath.Dir(current) {
		parent := filepath.Dir(current)
		label := filepath.Base(current)
		if parent == current {
			label = current
		}
		parts = append([]breadcrumbPart{{label: label, path: current}}, parts...)
		if parent == current {
			break
		}
	}

	startPart := 0
	truncated := breadcrumbPartsWidth(parts) > maxWidth
	if truncated {
		startPart = len(parts) - 1
		for startPart > 0 {
			candidateWidth := ansi.StringWidth(truncatedBreadcrumbPrefix) +
				breadcrumbPartsWidth(parts[startPart-1:])
			if candidateWidth > maxWidth {
				break
			}
			startPart--
		}
	}

	if truncated && maxWidth <= ansi.StringWidth(truncatedBreadcrumbPrefix) {
		label := ansi.Truncate(filepath.Base(cleanLocation), maxWidth, "")
		return label, []BreadcrumbSpan{{Start: 0, End: ansi.StringWidth(label), Path: cleanLocation}}
	}
	var line strings.Builder
	if truncated {
		line.WriteString(truncatedBreadcrumbPrefix)
	}
	visibleParts := parts[startPart:]
	if truncated && len(visibleParts) == 1 {
		availableWidth := maxWidth - ansi.StringWidth(truncatedBreadcrumbPrefix)
		visibleParts[0].label = common.TruncateTextBeginning(visibleParts[0].label, availableWidth, "...")
	}
	spans := make([]BreadcrumbSpan, 0, len(visibleParts))
	for i, part := range visibleParts {
		if i > 0 {
			line.WriteString(breadcrumbSeparator)
		}
		start := ansi.StringWidth(line.String())
		line.WriteString(part.label)
		spans = append(spans, BreadcrumbSpan{
			Start: start,
			End:   start + ansi.StringWidth(part.label),
			Path:  part.path,
		})
	}
	return line.String(), spans
}

func breadcrumbPartsWidth(parts []breadcrumbPart) int {
	width := 0
	for i, part := range parts {
		if i > 0 {
			width += ansi.StringWidth(breadcrumbSeparator)
		}
		width += ansi.StringWidth(part.label)
	}
	return width
}

func breadcrumbPathAt(spans []BreadcrumbSpan, x int) (string, bool) {
	for _, span := range spans {
		if x >= span.Start && x < span.End {
			return span.Path, true
		}
	}
	return "", false
}

// BreadcrumbPathAtContentX returns the breadcrumb destination at a file-panel
// content X coordinate. The coordinate includes the tab and directory icon
// prefixes.
func (m *Model) BreadcrumbPathAtContentX(x int) (string, bool) {
	prefixWidth := m.tabBarWidth() + ansi.StringWidth(common.FilePanelTopDirectoryIcon)
	if x < prefixWidth {
		return "", false
	}
	availableWidth := m.GetContentWidth() - prefixWidth
	if m.TabCount() == 1 {
		availableWidth -= len(" [+]")
	}
	if availableWidth <= 0 {
		return "", false
	}
	_, spans := m.headerBreadcrumbs(availableWidth)
	return breadcrumbPathAt(spans, x-prefixWidth)
}

func (m *Model) headerBreadcrumbs(width int) (string, []BreadcrumbSpan) {
	if m.TabCount() != 1 {
		return renderBreadcrumbs(m.Location, width)
	}
	if width <= 0 {
		return "", nil
	}
	path := filepath.Clean(m.Location)
	fullWidth := ansi.StringWidth(path)
	offset := max(0, fullWidth-width)
	line := path
	if offset > 0 {
		line = ansi.Cut(path, offset+1, fullWidth)
		line = "…" + line
	}
	var spans []BreadcrumbSpan
	for current := path; ; current = filepath.Dir(current) {
		end := ansi.StringWidth(current) - offset
		start := end - ansi.StringWidth(filepath.Base(current))
		if filepath.Dir(current) == current {
			start = -offset
		}
		if end > 0 && start >= 0 && (offset == 0 || start > 0) {
			spans = append(spans, BreadcrumbSpan{Start: start, End: end, Path: current})
		}
		if filepath.Dir(current) == current {
			break
		}
	}
	return line, spans
}
