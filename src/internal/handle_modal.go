package internal

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/yorukot/superfile/src/internal/ui/filepanel"
	"github.com/yorukot/superfile/src/internal/ui/processbar"
	"github.com/yorukot/superfile/src/pkg/utils"
)

// Cancel typing modal e.g. create file or directory
func (m *model) cancelTypingModal() {
	m.typingModal.textInput.Blur()
	m.typingModal.open = false
}

// Confirm to create file or directory
func createItem(location, item string) error {
	if err := checkFileNameValidity(item); err != nil {
		slog.Error("Errow while createItem during item creation", "error", err)
		return err
	}
	path := filepath.Join(location, item)
	if !strings.HasSuffix(item, string(filepath.Separator)) {
		path, _ = renameIfDuplicate(path)
		if err := os.MkdirAll(filepath.Dir(path), utils.UserDirPerm); err != nil {
			slog.Error("Error while createItem during directory creation", "error", err)
			return err
		}
		f, err := os.Create(path)
		if err != nil {
			slog.Error("Error while createItem during file creation", "error", err)
			return err
		}
		defer f.Close()
	} else {
		err := os.MkdirAll(path, utils.UserDirPerm)
		if err != nil {
			slog.Error("Error while createItem during directory creation", "error", err)
			return err
		}
	}
	return nil
}

func (m *model) getCreateCmd() tea.Cmd {
	if !m.typingModal.open {
		return nil
	}

	items := []string{m.typingModal.textInput.Value()}
	location := m.typingModal.location

	reqID := m.nextIoReqCnt()
	slog.Debug("Submitting create request", "id", reqID, "items cnt", len(items))
	m.cancelTypingModal()
	return m.processBarModel.Run(func() tea.Msg {
		return m.createOperation(&m.processBarModel, location, items, reqID)
	})
}

func (m *model) createOperation(
	processBarModel *processbar.Model,
	location string,
	items []string,
	reqID int,
) tea.Msg {
	if len(items) == 0 {
		return NewCreateOperationMsg(processbar.Cancelled, reqID)
	}
	p, err := processBarModel.SendAddProcessMsg(filepath.Base(items[0]), processbar.OpCreate, len(items), true)
	if err != nil {
		slog.Error("Cannot spawn a new process", "error", err)
		return NewCreateOperationMsg(processbar.Failed, reqID)
	}
	finalizer := func(state processbar.ProcessState, reqID int) tea.Msg {
		return NewCreateOperationMsg(state, reqID)
	}
	processor := makeCreateProcessor(location, p, processBarModel)
	msg := m.runFileProcessor(processor, finalizer, items, reqID)
	return msg
}

func makeCreateProcessor(location string,
	process processbar.Process,
	processBarModel *processbar.Model) processbar.FileListProcessor {
	processorFunction := func(items []string) (processbar.Process, []string) {
		notProcessed := make([]string, 0)
		if len(items) == 0 {
			markProcessDone(process, processBarModel)
			return process, notProcessed
		}

		for i, item := range items {
			err := createItem(location, item)
			if err != nil {
				process.State = processbar.Failed
				slog.Error("Error in create operation", "item", item, "error", err)
				process.ErrorMsg = formatFileError(item, err)
				notProcessed = items[i:]
				break
			}
			process.CurrentFile = filepath.Base(item)
			process.Done++
			processBarModel.TrySendingUpdateProcessMsg(process)
		}

		if process.State != processbar.Failed {
			process.State = processbar.Successful
		}
		if len(notProcessed) == 0 {
			markProcessDone(process, processBarModel)
		}
		return process, notProcessed
	}
	return processorFunction
}

// Cancel rename file or directory
func (m *model) cancelRename() {
	panel := m.getFocusedFilePanel()
	panel.Rename.Blur()
	panel.Renaming = false
	m.fileModel.Renaming = false
}

// Connfirm rename file or directory
func (m *model) confirmRename() {
	panel := m.getFocusedFilePanel()

	// Although we dont expect this to happen based on our current flow
	// Just adding it here to be safe
	if panel.Empty() {
		slog.Error("confirmRename called on empty panel")
		return
	}

	oldPath := panel.GetFocusedItem().Location
	newPath := filepath.Join(panel.Location, panel.Rename.Value())

	var err error
	switch {
	case oldPath == newPath:
		// No-op.
	case strings.EqualFold(filepath.Clean(oldPath), filepath.Clean(newPath)) &&
		sameFileIdentity(oldPath, newPath):
		// Allows case-only renames on case-insensitive filesystems.
		err = os.Rename(oldPath, newPath)
	default:
		err = moveElementNoReplace(oldPath, newPath)
	}
	if err != nil {
		m.showUserError("Could not rename item", err)
		// Dont return. We have to also reset the panel and model information
	} else if oldPath != newPath {
		moves := []pathMove{{from: oldPath, to: newPath}}
		m.recordUndo("rename", moves)
		m.updateCatalogMoves(moves)
	}
	m.fileModel.Renaming = false
	panel.Rename.Blur()
	panel.Renaming = false
	if err == nil && oldPath != newPath {
		m.refreshAfterDiskChange()
	}
}

func (m *model) confirmSortOptions() {
	panel := m.getFocusedFilePanel()
	panel.SortKind = m.sortModal.GetSelectedKind()
	m.sortModal.Close()
}

// Cancel search, this will clear all searchbar input
func (m *model) cancelSearch() {
	panel := m.getFocusedFilePanel()
	panel.SearchBar.Blur()
	panel.SearchBar.SetValue("")
}

// Confirm search. This will exit the search bar and filter the files
func (m *model) confirmSearch() {
	panel := m.getFocusedFilePanel()
	panel.SearchBar.Blur()
}

func (m *model) getFocusedFilePanel() *filepanel.Model {
	return m.fileModel.GetFocusedFilePanel()
}
