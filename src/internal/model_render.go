package internal

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/yorukot/superfile/src/internal/common"

	"charm.land/lipgloss/v2"

	"github.com/yorukot/superfile/src/config/icon"
)

func (m *model) sidebarRender() string {
	return m.sidebarModel.Render(m.focusPanel == sidebarFocus,
		m.getFocusedFilePanel().Location)
}

func (m *model) processBarRender() string {
	return m.processBarModel.Render(m.focusPanel == processBarFocus)
}

func (m *model) terminalSizeWarnRender() string {
	fullWidthString := strconv.Itoa(m.fullWidth)
	fullHeightString := strconv.Itoa(m.fullHeight)
	minimumWidthString := strconv.Itoa(common.MinimumWidth)
	minimumHeightString := strconv.Itoa(common.MinimumHeight)
	if m.fullHeight < common.MinimumHeight {
		fullHeightString = common.TerminalTooSmall.Render(fullHeightString)
	}
	if m.fullWidth < common.MinimumWidth {
		fullWidthString = common.TerminalTooSmall.Render(fullWidthString)
	}
	fullHeightString = common.TerminalCorrectSize.Render(fullHeightString)
	fullWidthString = common.TerminalCorrectSize.Render(fullWidthString)

	heightString := common.MainStyle.Render(" Height = ")
	return common.FullScreenStyle(m.fullHeight, m.fullWidth).Render(`Terminal size too small:` + "\n" +
		"Width = " + fullWidthString +
		heightString + fullHeightString + "\n\n" +
		"Needed for current config:" + "\n" +
		"Width = " + common.TerminalCorrectSize.Render(minimumWidthString) +
		heightString + common.TerminalCorrectSize.Render(minimumHeightString))
}

func (m *model) terminalSizeWarnAfterFirstRender() string {
	minimumWidthInt := common.Config.SidebarWidth + common.FilePanelWidthUnit*len(
		m.fileModel.FilePanels,
	) + common.FilePanelWidthUnit - 1
	minimumWidthString := strconv.Itoa(minimumWidthInt)
	fullWidthString := strconv.Itoa(m.fullWidth)
	fullHeightString := strconv.Itoa(m.fullHeight)
	minimumHeightString := strconv.Itoa(common.MinimumHeight)

	if m.fullHeight < common.MinimumHeight {
		fullHeightString = common.TerminalTooSmall.Render(fullHeightString)
	}
	if m.fullWidth < minimumWidthInt {
		fullWidthString = common.TerminalTooSmall.Render(fullWidthString)
	}
	fullHeightString = common.TerminalCorrectSize.Render(fullHeightString)
	fullWidthString = common.TerminalCorrectSize.Render(fullWidthString)

	heightString := common.MainStyle.Render(" Height = ")
	return common.FullScreenStyle(m.fullHeight, m.fullWidth).Render(`Your terminal size is too small:` + "\n" +
		"Width = " + fullWidthString +
		heightString + fullHeightString + "\n\n" +
		"Needed for current config:" + "\n" +
		"Width = " + common.TerminalCorrectSize.Render(minimumWidthString) +
		heightString + common.TerminalCorrectSize.Render(minimumHeightString))
}

func (m *model) typineModalRender() string {
	previewPath := filepath.Join(m.typingModal.location, m.typingModal.textInput.Value())

	fileLocation := common.FilePanelTopDirectoryIconStyle.Render(" "+icon.Directory+icon.Space) +
		common.FilePanelTopPathStyle.Render(
			common.TruncateTextBeginning(previewPath, common.ModalWidth-common.InnerPadding, "..."),
		) + "\n"

	confirm := common.ModalConfirm.Render(" (" + common.Hotkeys.ConfirmTyping[0] + ") Create ")
	cancel := common.ModalCancel.Render(" (" + common.Hotkeys.CancelTyping[0] + ") Cancel ")

	tip := confirm +
		lipgloss.NewStyle().Background(common.ModalBGColor).Render("           ") +
		cancel

	// TODO : Move this all to rendering package to avoid specifying newlines manually
	return common.ModalBorderStyle(common.ModalHeight, common.ModalWidth).
		Render(fileLocation + "\n" + m.typingModal.textInput.View() + "\n\n" + tip)
}

func (m *model) introduceModalRender() string {
	title := common.SidebarTitleStyle.Render(" Xelif")
	content := common.ModalStyle.Render(
		" Click folders, breadcrumbs, tabs, Search, or Commands.\n" +
			" Keyboard: Enter opens; ? shows your configured keys.\n\n" +
			" Commands: ctrl+shift+p     Search: ctrl+g\n\n" +
			" Themes and Vim bindings are configurable.\n" +
			" Upstream configuration guide: superfile.dev\n\n" +
			" Inspired by github.com/yorukot/superfile.\n\n" +
			" Press any key to continue.",
	)
	body := title + "\n\n" + content
	style := common.FirstUseModal(0, m.helpMenu.GetWidth())
	if _, noColor := os.LookupEnv("NO_COLOR"); !noColor {
		style = style.BorderForeground(lipgloss.Color("#FFFFFF"))
	}
	return style.Render(body)
}

func (m *model) promptModalRender() string {
	return m.promptModal.Render()
}

func (m *model) zoxideModalRender() string {
	return m.zoxideModal.Render()
}
