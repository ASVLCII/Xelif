package internal

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/yorukot/superfile/src/config/icon"
	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/profile"
)

const (
	appearanceDefaultContentLines = 64
	appearanceChromeLines         = 4
	appearanceMaxWidth            = 56
	appearanceMinWidth            = 20
	appearanceWidthMargin         = 6
	appearanceMessageWidth        = 48
	appearanceItemWidth           = 50
	appearanceCenterDivisor       = 2
	appearanceContentTopRow       = 2
	appearancePortableFirstRow    = 2
	appearancePortableLastRow     = 3
	appearanceRichFirstRow        = 5
	appearanceRichLastRow         = 7
	appearanceMaxPageItems        = 6
	appearanceImportHeaderLines   = 4
	appearanceImportPageOverhead  = 12
	appearanceLeftKey             = "left"
	appearanceRightKey            = "right"
)

type setupImportMode int

const (
	setupImportPreview setupImportMode = iota
	setupImportRecover
	setupImportMain
)

type setupImportState struct {
	Preview      profile.ImportPreview
	Page         int
	Mode         setupImportMode
	Status       profile.Status
	InspectError string
	Legacy       profile.Paths
	Active       profile.Paths
	Checked      bool
}

func (m *model) appearanceState() *setupImportState {
	if m.setupImport == nil {
		m.setupImport = &setupImportState{Mode: setupImportMain}
	}
	s := m.setupImport
	if s.Checked {
		return s
	}
	if s.Active.ConfigDir == "" {
		s.Legacy, s.Active = profile.LegacyPaths(), profile.ActivePaths()
	}
	status, err := profile.Inspect(s.Active)
	if err != nil {
		s.Status, s.InspectError = profile.Status{}, err.Error()
	} else {
		s.Status, s.InspectError = status, ""
	}
	preview, err := profile.PreviewLegacyImport(s.Legacy, s.Active)
	if err != nil {
		if s.InspectError == "" {
			s.InspectError = err.Error()
		}
	} else {
		s.Preview = preview
	}
	s.Checked = true
	return s
}

func (m *model) openAppearance() {
	m.appearanceOpen = true
	m.appearanceChoice = 0
	m.appearanceMessage = ""
	m.setupImport = nil
	m.appearanceState()
}

func (m *model) appearanceContentLimit() int {
	if m.fullHeight <= 0 {
		return appearanceDefaultContentLines
	}
	return max(1, m.fullHeight-appearanceChromeLines)
}

func (m *model) appearanceBox(lines []string) string {
	if limit := m.appearanceContentLimit(); len(lines) > limit {
		lines = lines[:limit]
	}
	w := min(appearanceMaxWidth, max(appearanceMinWidth, m.fullWidth-appearanceWidthMargin))
	style := lipgloss.NewStyle().Width(w).Border(common.GenerateBorder()).Padding(1, 1)
	contentWidth := max(1, w-style.GetHorizontalFrameSize())
	bounded := make([]string, len(lines))
	for i, line := range lines {
		bounded[i] = ansi.Truncate(line, contentWidth, "...")
	}
	return style.Render(strings.Join(bounded, "\n"))
}

func (m *model) appearanceRender() string {
	s := m.appearanceState()
	switch s.Mode {
	case setupImportPreview:
		return m.importPreviewRender()
	case setupImportRecover:
		return m.importRecoveryRender()
	case setupImportMain:
		// The main setup screen is rendered below.
	}
	portable, rich := "  Portable mode", "  Rich icons"
	if m.appearanceChoice == 0 {
		portable = "> Portable mode"
	} else {
		rich = "> Rich icons"
	}
	base := []string{
		"Appearance and setup",
		"",
		portable,
		"  dir/ notes.txt photo.png archive.zip app.go",
		"",
		rich,
		"  " + icon.Folders["folder"].Icon + " Folder  " + icon.Icons["doc"].Icon + " Document  " + icon.Icons["image"].Icon + " Image",
		"  " + icon.Icons["package"].Icon + " Archive  " + icon.Icons["go"].Icon + " Code",
		"  Nerd Font: installed AND selected in terminal.",
		"  Font support cannot be detected automatically.",
		"",
		"Save choice   Cancel   Import legacy profile",
	}
	optional := []string{}
	if s.Preview.Found {
		optional = append(optional, fmt.Sprintf(
			"Legacy profile found: %d ready, %d rejected",
			s.Preview.ReadyCount,
			s.Preview.RejectedCount,
		))
	}
	if s.Status.PendingImport {
		optional = append(optional, "Interrupted import found", "Recover interrupted import (R)")
	}
	if s.InspectError != "" {
		optional = append(optional, "Profile check unavailable; nothing changed")
	}
	help := []string{
		"Arrows: choose  Enter: save  Esc: back",
		"I: import  R: recover interrupted import",
	}
	message := []string{}
	if m.appearanceMessage != "" {
		message = []string{"", common.TruncateText(m.appearanceMessage, appearanceMessageWidth, "...")}
	}
	budget := m.appearanceContentLimit() - len(base) - len(help) - len(message)
	if budget < 0 {
		message, budget = nil, m.appearanceContentLimit()-len(base)-len(help)
	}
	if budget < 0 {
		budget = 0
	}
	if len(optional) > budget {
		optional = optional[:budget]
	}
	lines := make([]string, 0, len(base)+len(optional)+len(help)+len(message))
	lines = append(lines, base...)
	lines = append(lines, optional...)
	lines = append(lines, help...)
	lines = append(lines, message...)
	return m.appearanceBox(lines)
}

func (m *model) handleAppearanceKey(key string) tea.Cmd {
	s := m.appearanceState()
	switch s.Mode {
	case setupImportPreview:
		return m.handleImportKey(key)
	case setupImportRecover:
		return m.handleRecoveryKey(key)
	case setupImportMain:
		// The main setup keys are handled below.
	}
	switch key {
	case "i", "I":
		m.beginImport()
	case "r", "R":
		m.beginRecovery()
	case "up", browserDownKey, "tab", "shift+tab", appearanceLeftKey, appearanceRightKey:
		m.appearanceChoice = 1 - m.appearanceChoice
	case browserEscapeKey:
		if !m.firstUse {
			m.appearanceOpen = false
			m.setupImport = nil
		} else {
			m.appearanceMessage = "Choose an appearance to finish setup; Ctrl+Q exits."
		}
	case "ctrl+q":
		m.modelQuitState = quitInitiated
		return m.finishKeyInput(nil, false)
	case browserEnterKey:
		mode := "portable"
		if m.appearanceChoice == 1 {
			mode = "rich"
		}
		if err := profile.SaveAppearance(s.Active, mode); err != nil {
			m.appearanceMessage = fmt.Sprintf("Not saved: %v", err)
			return nil
		}
		common.LoadConfigFile()
		applyPlainModeConfig()
		icon.InitIcon(common.Config.Nerdfont, common.Theme.DirectoryIconColor)
		common.LoadThemeConfig()
		common.LoadPrerenderedVariables()
		m.appearanceOpen, m.firstUse = false, false
		m.setupImport = nil
		return m.updateComponentDimensions()
	}
	return nil
}

func (m *model) beginImport() {
	s := m.appearanceState()
	preview, err := profile.PreviewLegacyImport(s.Legacy, s.Active)
	if err != nil {
		s.InspectError = err.Error()
		m.appearanceMessage = fmt.Sprintf("Import unavailable: %v", err)
		return
	}
	s.Preview, s.Page = preview, 0
	if !preview.Found {
		m.appearanceMessage = "No legacy Superfile profile found; nothing changed."
		return
	}
	s.Mode = setupImportPreview
	m.appearanceMessage = ""
}

func (m *model) beginRecovery() {
	s := m.appearanceState()
	status, err := profile.Inspect(s.Active)
	if err != nil {
		m.appearanceMessage = fmt.Sprintf("Recovery unavailable: %v", err)
		return
	}
	if !status.PendingImport {
		m.appearanceMessage = "No interrupted import found; no files changed."
		return
	}
	s.Status, s.Mode = status, setupImportRecover
	m.appearanceMessage = ""
}

func (m *model) handleAppearanceClick(mouse tea.Mouse) tea.Cmd {
	s := m.appearanceState()
	content := m.appearanceRender()
	x := m.fullWidth/appearanceCenterDivisor - lipgloss.Width(content)/appearanceCenterDivisor
	y := m.fullHeight/appearanceCenterDivisor - lipgloss.Height(content)/appearanceCenterDivisor
	localX, localY := mouse.X-x, mouse.Y-y
	if localX < 0 || localY < 0 || localX >= lipgloss.Width(content) || localY >= lipgloss.Height(content) {
		return nil
	}
	switch s.Mode {
	case setupImportPreview:
		switch {
		case renderedLabelHit(content, "Import valid files", localX, localY):
			return m.handleImportKey(browserEnterKey)
		case renderedLabelHit(content, "Cancel", localX, localY):
			return m.handleImportKey(browserEscapeKey)
		case renderedLabelHit(content, "PgUp", localX, localY):
			return m.handleImportKey("pgup")
		case renderedLabelHit(content, "PgDn", localX, localY):
			return m.handleImportKey("pgdown")
		}
	case setupImportRecover:
		switch {
		case renderedLabelHit(content, "Recover now", localX, localY):
			return m.handleRecoveryKey(browserEnterKey)
		case renderedLabelHit(content, "Cancel", localX, localY):
			return m.handleRecoveryKey(browserEscapeKey)
		}
	case setupImportMain:
		row := localY - appearanceContentTopRow
		switch {
		case row >= appearancePortableFirstRow && row <= appearancePortableLastRow:
			m.appearanceChoice = 0
		case row >= appearanceRichFirstRow && row <= appearanceRichLastRow:
			m.appearanceChoice = 1
		}
		switch {
		case renderedLabelHit(content, "Save choice", localX, localY):
			return m.handleAppearanceKey(browserEnterKey)
		case renderedLabelHit(content, "Cancel", localX, localY):
			return m.handleAppearanceKey(browserEscapeKey)
		case renderedLabelHit(content, "Import legacy profile", localX, localY):
			return m.handleAppearanceKey("i")
		case renderedLabelHit(content, "Recover interrupted import", localX, localY):
			return m.handleAppearanceKey("r")
		}
	}
	return nil
}

func (m *model) importPageSize() int {
	if m.fullHeight <= 0 {
		return appearanceMaxPageItems
	}
	return max(1, min(appearanceMaxPageItems, m.fullHeight-appearanceImportPageOverhead))
}

func (m *model) importPreviewRender() string {
	s := m.setupImport
	p := s.Preview
	size := m.importPageSize()
	pages := max(1, (len(p.Items)+size-1)/size)
	page := min(s.Page, pages-1)
	lines := []string{
		"Import legacy profile",
		"",
		fmt.Sprintf("%d valid files; %d rejected (kept at source)", p.ReadyCount, p.RejectedCount),
		"",
	}
	start := page * size
	for i := start; i < min(start+size, len(p.Items)); i++ {
		item := p.Items[i]
		label := item.Kind + " " + filepath.Base(item.Source) + " - " + item.Validation
		if item.ReplaceExisting {
			label += " (back up + replace)"
		}
		lines = append(lines, common.TruncateText(label, appearanceItemWidth, "..."))
	}
	for len(lines) < appearanceImportHeaderLines+size {
		lines = append(lines, "")
	}
	lines = append(
		lines,
		fmt.Sprintf("PgUp: previous   PgDn: next   Page %d/%d", page+1, pages),
		"Source untouched; no credentials, caches or plugins.",
		"Import valid files (Enter)",
		"Cancel (Esc)",
	)
	return m.appearanceBox(lines)
}

func (m *model) importRecoveryRender() string {
	lines := []string{
		"Recover interrupted import",
		"",
		"An earlier import may be partial.",
		"Recovery checks staged files and backups.",
		"Activated files are checked before any change.",
		"Partial changes are not rolled back automatically.",
		"",
		"Enter runs the explicit recovery; Esc cancels.",
		"Esc performs no further writes or deletions.",
		"",
		"Recover now (Enter)",
		"Cancel (Esc)",
	}
	return m.appearanceBox(lines)
}

func (m *model) handleImportKey(key string) tea.Cmd {
	s := m.setupImport
	if s == nil || s.Mode != setupImportPreview {
		return nil
	}
	size := m.importPageSize()
	last := max(0, (len(s.Preview.Items)-1)/size)
	s.Page = min(s.Page, last)
	switch key {
	case browserEscapeKey:
		s.Mode = setupImportMain
		m.appearanceMessage = "Import cancelled; no files changed."
	case "pgdown", browserDownKey, appearanceRightKey:
		s.Page = min(s.Page+1, last)
	case "pgup", "up", appearanceLeftKey:
		s.Page = max(0, s.Page-1)
	case "home":
		s.Page = 0
	case "end":
		s.Page = last
	case browserEnterKey:
		if s.Preview.ReadyCount == 0 {
			s.Mode = setupImportMain
			m.appearanceMessage = "No supported legacy files to import."
			return nil
		}
		result, err := profile.ImportLegacy(s.Legacy, s.Active)
		s.Mode, s.Checked = setupImportMain, false
		if err != nil {
			m.appearanceMessage = fmt.Sprintf("Import stopped safely: %v", err)
		} else {
			m.appearanceMessage = fmt.Sprintf(
				"Imported %d files. Restart to load imported state.",
				len(result.Imported),
			)
		}
	}
	return nil
}

func (m *model) handleRecoveryKey(key string) tea.Cmd {
	s := m.setupImport
	if s == nil || s.Mode != setupImportRecover {
		return nil
	}
	switch key {
	case browserEscapeKey:
		s.Mode = setupImportMain
		m.appearanceMessage = "Recovery cancelled; no files changed."
	case browserEnterKey:
		status, err := profile.Inspect(s.Active)
		if err != nil {
			s.Mode, s.Checked = setupImportMain, false
			m.appearanceMessage = fmt.Sprintf("Recovery failed; existing files preserved: %v", err)
			return nil
		}
		if !status.PendingImport {
			s.Mode, s.Checked = setupImportMain, false
			m.appearanceMessage = "No interrupted import found; no files changed."
			return nil
		}
		result, err := profile.RecoverImport(s.Active)
		s.Mode, s.Checked = setupImportMain, false
		if err != nil {
			if result.BackupDir != "" && len(result.Imported) == 0 {
				m.appearanceMessage = fmt.Sprintf(
					"Recovery failed; journal archived, existing files preserved: %v",
					err,
				)
			} else {
				m.appearanceMessage = fmt.Sprintf(
					"Recovery stopped after %d files; keep the recovery journal and backups: %v",
					len(result.Imported),
					err,
				)
			}
			return nil
		}
		m.appearanceMessage = fmt.Sprintf(
			"Recovered %d files. Restart to load imported state.",
			len(result.Imported),
		)
	}
	return nil
}
