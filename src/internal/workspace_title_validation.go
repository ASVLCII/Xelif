package internal

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/session"
)

// Both generated titles and duplicate suffixes must fit the durable format.
// Truncation removes whole terminal graphemes, never a partial UTF-8 sequence.
func boundedWorkspaceTitle(base, suffix string) string {
	base = strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, ansi.Strip(base)))
	budget := session.MaxWorkspaceTitleBytes - len(suffix)
	base = ansi.Truncate(base, budget, "")
	for len(base) > budget {
		width := ansi.StringWidth(base)
		if width == 0 {
			base = ""
			break
		}
		base = ansi.Truncate(base, width-1, "")
	}
	if strings.TrimSpace(base) == "" {
		base = "Workspace"
	}
	return strings.TrimSpace(base) + suffix
}

func workspaceRenameValidation(title string) string {
	if title == "" || !utf8.ValidString(title) {
		return "Enter a valid, non-empty name"
	}
	if len(title) > session.MaxWorkspaceTitleBytes {
		return fmt.Sprintf("Shorten the name (maximum %d bytes)", session.MaxWorkspaceTitleBytes)
	}
	if strings.ContainsFunc(title, unicode.IsControl) {
		return "Use a single line without control characters"
	}
	return ""
}

func (m *model) workspaceRenameRender() string {
	content := "Rename workspace\n\n" + m.workspaceNameInput.View() + "\n\n"
	if m.workspaceRenameError != "" {
		content += common.PromptFailureStyle.Render(
			common.TruncateText(m.workspaceRenameError, workspaceRenameWidth, "..."),
		) + "\n\n"
	}
	return lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Padding(1, workspaceRenameHorizontalPadding).
		Render(content + "Enter: save   Esc: cancel")
}

func (m *model) canAddWorkspace() bool {
	if len(m.workspaces) < session.MaxWorkspaces {
		return true
	}
	m.workspaceNotice = fmt.Sprintf("Limit of %d open workspaces reached; close one first", session.MaxWorkspaces)
	return false
}
