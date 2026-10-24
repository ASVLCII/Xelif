package filemodel

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui/filepanel"
	"github.com/yorukot/superfile/src/internal/ui/preview"
	"github.com/yorukot/superfile/src/internal/ui/processbar"
)

func (m *Model) CreateNewFilePanel(location string) (tea.Cmd, error) {
	if m.PanelCount() >= m.MaxFilePanel {
		return nil, ErrMaximumPanelCount
	}

	if _, err := os.Stat(location); err != nil {
		return nil, fmt.Errorf("cannot access location : %s", location)
	}

	m.FilePanels = append(m.FilePanels, filepanel.New(
		location, false, "", m.GetFocusedFilePanel().SortKind,
		m.GetFocusedFilePanel().SortReversed))

	newPanelIndex := m.PanelCount() - 1

	m.FilePanels[m.FocusedPanelIndex].IsFocused = false
	m.FilePanels[newPanelIndex].IsFocused = true
	m.FilePanels[newPanelIndex].SetHeight(m.Height)
	m.FocusedPanelIndex = newPanelIndex

	m.updateChildComponentWidth()
	return m.ensurePreviewDimensionsSync(), nil
}

func (m *Model) CloseFilePanel() (tea.Cmd, error) {
	if m.PanelCount() <= 1 {
		return nil, ErrMinimumPanelCount
	}

	m.FilePanels = append(m.FilePanels[:m.FocusedPanelIndex],
		m.FilePanels[m.FocusedPanelIndex+1:]...)

	if m.FocusedPanelIndex != 0 {
		m.FocusedPanelIndex--
	}
	m.FilePanels[m.FocusedPanelIndex].IsFocused = true
	m.updateChildComponentWidth()

	return m.ensurePreviewDimensionsSync(), nil
}

func (m *Model) ToggleFilePreviewPanel() tea.Cmd {
	if m.previewCancel != nil {
		m.previewCancel()
	}
	m.FilePreview.ToggleOpen()
	if !m.FilePreview.IsOpen() {
		m.ioReqCnt++
		m.activePreviewReqID = m.ioReqCnt
	}
	m.updateChildComponentWidth()
	return m.ensurePreviewDimensionsSync()
}

func (m *Model) UpdatePreviewPanel(msg preview.UpdateMsg) tea.Cmd {
	if !m.FilePreview.IsOpen() {
		slog.Debug("FilePreviewUpdateMsg received while preview is closed. Ignoring")
		return nil
	}
	if msg.GetReqID() != m.activePreviewReqID {
		slog.Debug("FilePreviewUpdateMsg for older request. Ignoring",
			"activeReqID", m.activePreviewReqID, "msgReqID", msg.GetReqID())
		return nil
	}

	selectedItem := m.GetFocusedFilePanel().GetFocusedItemPtr()
	if selectedItem == nil {
		slog.Debug("Panel empty or cursor invalid. Ignoring FilePreviewUpdateMsg")
		return nil
	}
	if selectedItem.Location != msg.GetLocation() {
		slog.Debug("FilePreviewUpdateMsg for older files. Ignoring",
			"curLocation", selectedItem.Location, "msgLocation", msg.GetLocation())
		return nil
	}

	if m.ExpectedPreviewWidth != msg.GetContentWidth() ||
		m.Height != msg.GetContentHeight() {
		slog.Debug("FilePreviewUpdateMsg for older dimensions. Ignoring",
			"curW", m.ExpectedPreviewWidth, "curH", m.Height,
			"msgW", msg.GetContentWidth(), "msgH", msg.GetContentHeight())
		return nil
	}
	m.FilePreview.Apply(msg)

	// For Kitty images, transmit image data directly to the terminal
	if raw := msg.GetRawTransmit(); raw != "" {
		return tea.Raw(raw)
	}
	return nil
}

func (m *Model) GetFilePreviewCmd(forcePreviewRender bool) tea.Cmd {
	if !m.FilePreview.IsOpen() {
		return nil
	}
	panel := m.GetFocusedFilePanel()
	if panel.EmptyOrInvalid() {
		// Sync call because this will be fast
		m.FilePreview.SetEmptyWithDimensions(m.ExpectedPreviewWidth, m.Height)
		return nil
	}
	selectedItem := panel.GetFocusedItem()
	if m.FilePreview.GetLocation() == selectedItem.Location && !forcePreviewRender {
		return nil
	}

	m.FilePreview.SetLocation(selectedItem.Location)
	m.FilePreview.SetLoading()

	// HACK!!!. fileModel must not be aware of other dimensions. but...
	// Unfortunately, previewPanel isn't completely 'under' fileModel
	// Note: Must save the dimensions for the closure of the Cmd to avoid
	// problems
	fullModalWidth := m.Width + common.Config.SidebarWidth
	if common.Config.SidebarWidth != 0 {
		fullModalWidth += common.BorderPadding
	}
	width := m.ExpectedPreviewWidth
	height := m.Height

	reqCnt := m.ioReqCnt
	m.ioReqCnt++
	m.activePreviewReqID = reqCnt
	slog.Debug("Submitting file preview render request", "id", reqCnt,
		"path", selectedItem.Location, "w", width, "h", height)

	if m.previewCancel != nil {
		m.previewCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.previewCancel = cancel
	renderer := m.FilePreview
	cmd := func() tea.Msg {
		content, rawTransmit := "Preview cancelled", ""
		if ctx.Err() == nil {
			content, rawTransmit = renderer.RenderWithPath(selectedItem.Location, width, height, fullModalWidth)
		}
		err := ctx.Err()
		if err != nil {
			content, rawTransmit = "Preview cancelled", ""
		}
		result := preview.NewUpdateMsg(selectedItem.Location, content, rawTransmit, width, height, reqCnt)
		result.Error = err
		return result
	}
	if m.Tasks == nil {
		return cmd
	}
	return m.Tasks.Track(
		processbar.TaskSpec{
			Kind:       "Preview",
			Source:     selectedItem.Location,
			Background: true,
			GroupKey:   fmt.Sprint("preview:", m.FocusedPanelIndex),
		},
		ctx,
		cancel,
		cmd,
		func(msg tea.Msg) error {
			result, _ := msg.(preview.UpdateMsg)
			return result.Error
		},
	)
}

func (m *Model) ToggleDotFile() {
	m.DisplayDotFiles = !m.DisplayDotFiles
	m.UpdateFilePanelsIfNeeded(true)
}

func (m *Model) UpdateFilePanelsIfNeeded(force bool) {
	for i := range m.FilePanels {
		m.FilePanels[i].UpdateElementsIfNeeded(force, m.DisplayDotFiles)
	}
}
