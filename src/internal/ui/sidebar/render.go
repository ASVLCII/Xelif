package sidebar

import (
	"log/slog"

	"charm.land/lipgloss/v2"

	"github.com/yorukot/superfile/src/internal/ui"

	"github.com/yorukot/superfile/src/config/icon"
	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui/rendering"
)

// Render returns the rendered sidebar string.
func (s *Model) Render(sidebarFocused bool, currentFilePanelLocation string) string {
	if s.Disabled() {
		return ""
	}

	r := ui.SidebarRenderer(s.height, s.width, sidebarFocused)

	r.AddLines(common.SideBarSuperfileTitle, "")

	r.AddLines(s.searchBar.View())

	if s.NoActualDir() {
		r.AddLines(common.SideBarNoneText)
	} else {
		s.directoriesRender(currentFilePanelLocation, sidebarFocused, r)
	}
	return r.Render()
}

type sidebarViewport struct {
	remaining, skip int
}

func (v *sidebarViewport) addRows(r *rendering.Renderer, lines ...string) {
	for _, line := range lines {
		if v.skip > 0 {
			v.skip--
		} else if v.remaining > 0 {
			r.AddLineWithCustomTruncate(line, rendering.TailsTruncateRight)
			v.remaining--
		}
	}
}

// directoriesRender clips entries and heading rows to the independent viewport.
func (s *Model) directoriesRender(curFilePanelFileLocation string,
	sideBarFocused bool, r *rendering.Renderer) {
	// Cursor should always point to a valid directory at this point
	if s.isCursorInvalid() {
		slog.Error("Unexpected situation in sideBar Model. "+
			"Cursor is at invalid position, while there are valid directories", "cursor", s.cursor,
			"directory count", len(s.directories))
	}

	viewport := sidebarViewport{remaining: s.viewportRows(), skip: s.renderOffset}
	for i := s.renderIndex; i < len(s.directories) && viewport.remaining > 0; i++ {
		switch s.directories[i] {
		case homeDividerDir:
			viewport.addRows(r, "", common.SideBarHomeDivider, "")
		case recentDividerDir:
			viewport.addRows(r, "", common.SideBarRecentDivider, "")
		case pinnedDividerDir:
			viewport.addRows(r, "", common.SideBarPinnedDivider, "")
		case diskDividerDir:
			viewport.addRows(r, "", common.SideBarDisksDivider, "")
		default:
			cursor := " "
			if s.cursor == i && sideBarFocused && !s.searchBar.Focused() {
				cursor = icon.Cursor
			}
			if s.renaming && s.cursor == i {
				viewport.addRows(r, s.rename.View())
			} else {
				renderStyle := common.SidebarStyle
				if s.directories[i].Location == curFilePanelFileLocation {
					renderStyle = common.SidebarSelectedStyle
				}
				line := common.FilePanelCursorStyle.Render(cursor+" ") +
					renderStyle.Foreground(lipgloss.Color(common.SidebarIconColor(
						s.directories[i].Icon, s.directories[i].Location == curFilePanelFileLocation,
					))).Render(s.directories[i].Icon+" ") +
					renderStyle.Render(s.directories[i].Name)
				viewport.addRows(r, line)
			}
		}
	}
}
