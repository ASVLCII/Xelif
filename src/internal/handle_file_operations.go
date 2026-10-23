package internal

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	variable "github.com/yorukot/superfile/src/config"
	"github.com/yorukot/superfile/src/internal/trash"
	"github.com/yorukot/superfile/src/internal/ui/filepanel"
	"github.com/yorukot/superfile/src/internal/ui/spferror"
	"github.com/yorukot/superfile/src/pkg/utils"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui/notify"
	"github.com/yorukot/superfile/src/internal/ui/processbar"

	tea "charm.land/bubbletea/v2"

	"github.com/atotto/clipboard"
)

// Processes any standard (f.e. deletion) operation with a list of files
func (m *model) runFileProcessor(processor processbar.FileListProcessor,
	finalizer processbar.ProcessFinalizer,
	items []string,
	reqID int,
) tea.Msg {
	slog.Debug("Lock mutex for modal error window")
	m.mutexErrorModal.Lock()
	result, toDo := processor(items)
	if result.State == processbar.Failed && len(toDo) > 0 {
		// we unlock mutexErrorModal on dispatch SpfErrorModalUpdateMsg
		errorModel := spferror.New(true,
			"Error",
			result.ErrorMsg, spferror.NewFileListError(toDo, processor, finalizer))
		return NewSpfErrorModalMsg(errorModel, reqID)
	}
	slog.Debug("Unlock mutex for modal error window")
	m.mutexErrorModal.Unlock()
	return finalizer(result.State, reqID)
}

func markProcessDone(process processbar.Process, processBarModel *processbar.Model) {
	process.DoneTime = time.Now()
	err := processBarModel.SendUpdateProcessMsg(process, true)
	if err != nil {
		slog.Error("Failed to send final delete operation update", "error", err)
	}
}

func formatFileError(filePath string, err error) string {
	var e *os.LinkError
	if errors.As(err, &e) {
		return fmt.Sprintf("Deleting %s: \n%s", filePath, e.Err.Error())
	}
	return err.Error()
}

// Create a file in the currently focus file panel
// TODO: Fix it. It doesn't creates a new file. It just opens a file model,
// that allows you to create a file. Actual creation happens here - createItem() in handle_modal.go
func (m *model) panelCreateNewFile() {
	panel := m.getFocusedFilePanel()

	m.typingModal.location = panel.Location
	m.typingModal.open = true
	m.typingModal.textInput = common.GenerateNewFileTextInput()
	m.firstTextInput = true
}

// Rename file where the cusror is located
// TODO: Fix this. It doesn't do any rename, just opens the rename text input
// Actual rename happens at confirmRename() in handle_modal.go
func (m *model) panelItemRename() {
	panel := m.getFocusedFilePanel()
	if panel.Empty() {
		return
	}

	cursorPos := -1
	nameRunes := []rune(panel.GetFocusedItem().Name)
	nameLen := len(nameRunes)
	for i := nameLen - 1; i >= 0; i-- {
		if nameRunes[i] == '.' {
			cursorPos = i
			break
		}
	}
	if cursorPos == -1 || cursorPos == 0 && nameLen > 0 || panel.GetFocusedItem().Directory {
		cursorPos = nameLen
	}

	m.fileModel.Renaming = true
	panel.Renaming = true
	m.firstTextInput = true
	// TODO: Don't re-create a new model on each rename. Don't create
	// unnecessary gargage for collection. Reuse the existing model.
	// Maintain its state, dimensions. Update its cursor and text when needed
	panel.Rename = common.GenerateRenameTextInput(
		m.fileModel.SinglePanelWidth-common.InnerPadding,
		cursorPos,
		panel.GetFocusedItem().Name)
}

func (m *model) getDeleteCmd(permDelete bool) tea.Cmd {
	panel := m.getFocusedFilePanel()
	if panel.Empty() {
		return nil
	}

	var items []string
	if panel.PanelMode == filepanel.SelectMode {
		items = panel.GetSelectedLocationsSortedAsVisible()
	} else {
		items = []string{panel.GetFocusedItem().Location}
	}

	useTrash := m.hasTrash && trash.Available(panel.Location) && !permDelete

	reqID := m.nextIoReqCnt()
	slog.Debug("Submitting delete request", "id", reqID, "items cnt", len(items))
	return func() tea.Msg {
		return m.deleteOperation(&m.processBarModel, items, useTrash, reqID)
	}
}

func (m *model) deleteOperation(processBarModel *processbar.Model, items []string, useTrash bool, reqID int) tea.Msg {
	if len(items) == 0 {
		return NewDeleteOperationMsg(processbar.Cancelled, reqID)
	}
	p, err := processBarModel.SendAddProcessMsg(filepath.Base(items[0]), processbar.OpDelete, len(items), true)
	if err != nil {
		slog.Error("Cannot spawn a new process", "error", err)
		return NewDeleteOperationMsg(processbar.Failed, reqID)
	}
	finalizer := func(state processbar.ProcessState, reqID int) tea.Msg { return NewDeleteOperationMsg(state, reqID) }
	processor := makeDeleteProcessor(p, processBarModel, useTrash)
	msg := m.runFileProcessor(processor, finalizer, items, reqID)
	return msg
}

func makeDeleteProcessor(process processbar.Process,
	processBarModel *processbar.Model,
	useTrash bool) processbar.FileListProcessor {
	processorFunction := func(items []string) (processbar.Process, []string) {
		notProcessed := make([]string, 0)
		if len(items) == 0 {
			markProcessDone(process, processBarModel)
			return process, notProcessed
		}
		deleteFunc := os.RemoveAll
		if useTrash {
			deleteFunc = func(item string) error {
				_, err := trash.Move(item)
				return err
			}
		}
		for i, item := range items {
			err := deleteFunc(item)
			if err != nil {
				process.State = processbar.Failed
				slog.Error("Error in delete operation", "item", item, "useTrash", useTrash, "error", err)
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

func (m *model) getDeleteTriggerCmd(deletePermanent bool) tea.Cmd {
	panel := m.getFocusedFilePanel()
	if (panel.PanelMode == filepanel.SelectMode && panel.SelectedCount() == 0) ||
		(panel.PanelMode == filepanel.BrowserMode && panel.Empty()) {
		return nil
	}

	reqID := m.nextIoReqCnt()

	return func() tea.Msg {
		title := common.TrashWarnTitle
		content := common.TrashWarnContent
		action := notify.DeleteAction

		if !m.hasTrash || !trash.Available(panel.Location) || deletePermanent {
			title = common.PermanentDeleteWarnTitle
			content = common.PermanentDeleteWarnContent
			action = notify.PermanentDeleteAction
		}
		return NewNotifyModalMsg(notify.New(true, title, content, action), reqID)
	}
}

// Copy directory or file's path to superfile's clipboard
// set cut to true/false accordingly
func (m *model) copySingleItem(cut bool) {
	panel := m.getFocusedFilePanel()
	m.clipboard.Reset(cut)
	if panel.Empty() {
		return
	}
	slog.Debug("handle_file_operations.copySingleItem", "cut", cut,
		"panel location", panel.GetFocusedItem().Location)
	m.clipboard.Add(panel.GetFocusedItem().Location)
}

// Copy all selected file or directory's paths to the clipboard
func (m *model) copyMultipleItem(cut bool) {
	panel := m.getFocusedFilePanel()
	m.clipboard.Reset(cut)
	if panel.SelectedCount() == 0 {
		return
	}
	items := panel.GetSelectedLocationsSortedAsVisible()
	slog.Debug("handle_file_operations.copyMultipleItem", "cut", cut,
		"panel selected files", items)
	m.clipboard.SetItems(items)
}

func (m *model) getPasteItemCmd() tea.Cmd {
	copyItems := m.clipboard.PruneInaccessibleItemsAndGet()
	cut := m.clipboard.IsCut()
	if len(copyItems) == 0 {
		return nil
	}

	reqID := m.nextIoReqCnt()
	clipboardGeneration := m.clipboard.Generation()
	panelLocation := m.getFocusedFilePanel().Location

	slog.Debug("Submitting pasteItems request", "id", reqID, "items cnt", len(copyItems), "dest", panelLocation)
	return func() tea.Msg {
		err := validatePasteOperation(panelLocation, copyItems, cut)
		if err != nil {
			return NewNotifyModalMsg(notify.New(true, "Invalid paste location", err.Error(), notify.NoAction),
				reqID)
		}
		return m.executePasteOperation(
			&m.processBarModel, panelLocation, copyItems, cut, clipboardGeneration, reqID,
		)
	}
}

func validatePasteOperation(panelLocation string, copyItems []string, cut bool) error {
	// Check if trying to paste into source or subdirectory for both cut and copy operations
	for _, srcPath := range copyItems {
		// Check if trying to cut and paste into the same directory - this would be a no-op
		// and could potentially cause issues, so we prevent it
		if filepath.Dir(srcPath) == panelLocation && cut {
			return fmt.Errorf("cannot paste into parent directory of source, srcPath : %v, panelLocation : %v",
				srcPath, panelLocation)
		}
		if cut && srcPath == panelLocation {
			return errors.New("cannot paste a directory into itself")
		}

		if isAncestor(srcPath, panelLocation) {
			return fmt.Errorf("cannot %s and paste a directory into itself or its subdirectory",
				getCopyOrCutOperationName(cut))
		}
	}

	return nil
}

func makePasteProcessor(process processbar.Process,
	processBarModel *processbar.Model,
	panelLocation string, cut bool, moves *[]pathMove,
) processbar.FileListProcessor {
	processorFunction := func(items []string) (processbar.Process, []string) {
		notProcessed := make([]string, 0)
		if len(items) == 0 {
			markProcessDone(process, processBarModel)
			return process, notProcessed
		}
		var err error
		for i, filePath := range items {
			errMessage := "cut item error"
			if cut {
				err = movePasteItem(filePath, panelLocation, moves)
			} else {
				// TODO : These error cases are hard to test. We have to somehow make the paste operations fail,
				// which is time consuming and manual. We should test these with automated testcases
				// UPD: use "chattr +i" for target catalog to fail past opeations
				err = pasteDir(filePath, filepath.Join(panelLocation, filepath.Base(filePath)),
					&process, false, processBarModel)
				if err != nil {
					errMessage = "paste item error"
				}
			}

			process.CurrentFile = filepath.Base(filePath)
			if err != nil {
				process.State = processbar.Failed
				slog.Error(errMessage, "error", err)
				slog.Debug("model.pasteItem - paste failure", "error", err,
					"current item", filePath, "errMessage", errMessage)
				process.ErrorMsg = formatFileError(filePath, err)
				notProcessed = items[i:]
				break
			}
			processBarModel.TrySendingUpdateProcessMsg(process)
		}
		if process.State != processbar.Failed {
			process.State = processbar.Successful
			process.Done = process.Total
		}
		if len(notProcessed) == 0 {
			markProcessDone(process, processBarModel)
		}
		return process, notProcessed
	}
	return processorFunction
}

func movePasteItem(filePath, panelLocation string, moves *[]pathMove) error {
	destination := filepath.Join(panelLocation, filepath.Base(filePath))
	destination, err := renameIfDuplicate(destination)
	if err != nil {
		return err
	}
	if err := moveElementNoReplace(filePath, destination); err != nil {
		return err
	}
	if moves != nil {
		*moves = append(*moves, pathMove{from: filePath, to: destination})
	}
	return nil
}

func (m *model) executePasteOperation(processBarModel *processbar.Model,
	panelLocation string, items []string, cut bool, clipboardGeneration uint64, reqID int,
) tea.Msg {
	if len(items) == 0 {
		return NewPasteOperationMsg(processbar.Cancelled, reqID)
	}
	var operation processbar.OperationType
	if cut {
		operation = processbar.OpCut
	} else {
		operation = processbar.OpCopy
	}

	p, err := processBarModel.SendAddProcessMsg(
		filepath.Base(items[0]),
		operation,
		getPasteProgressTotal(items, cut), true)
	if err != nil {
		slog.Error("Cannot spawn a new process", "error", err)
		return NewPasteOperationMsg(processbar.Failed, reqID)
	}
	moves := make([]pathMove, 0, len(items))
	finalizer := func(state processbar.ProcessState, reqId int) tea.Msg {
		return newPasteOperationMsgWithContext(state, reqId, cut, clipboardGeneration, moves)
	}
	processor := makePasteProcessor(p, processBarModel, panelLocation, cut, &moves)
	msg := m.runFileProcessor(processor, finalizer, items, reqID)
	return msg
}

func getPasteProgressTotal(items []string, cut bool) int {
	if cut {
		return len(items)
	}
	return getTotalFilesCnt(items)
}

func getTotalFilesCnt(copyItems []string) int {
	totalFiles := 0
	for _, folderPath := range copyItems {
		// TODO : Fix this. This is inefficient
		// In case of a cut operations for a directory with a lot of files
		// we are unnecessarily walking the whole directory recursively
		// while os will just perform a rename
		// So instead of few operations this will cause the cut paste
		// to read the whole directory recursively
		// we should avoid doing this.
		// Although this allows us a more detailed progress tracking
		// this make the copy/cut more inefficient
		// instead, we could just track progress based on total items in
		// copyItems
		// efficiency should be prioritized over more detailed feedback.
		count, err := countFiles(folderPath)
		if err != nil {
			slog.Error("Error in countFiles", "error", err)
			continue
		}
		totalFiles += count
	}
	return totalFiles
}

// Extract compressed file
// TODO : err should be returned and properly handled by the caller
func (m *model) getExtractFileCmd() tea.Cmd {
	panel := m.getFocusedFilePanel()
	if panel.Empty() {
		return nil
	}

	item := panel.GetFocusedItem().Location

	ext := strings.ToLower(filepath.Ext(item))
	if !common.IsExtensionExtractable(ext) {
		slog.Error("Error unexpected file", "extension type", ext, "item", item, "error", errors.ErrUnsupported)
		return nil
	}
	reqID := m.nextIoReqCnt()

	slog.Debug("Submitting Extract file request", "reqID", reqID, "item", item)

	return func() tea.Msg {
		outputDir := common.FileNameWithoutExtension(item)
		outputDir, err := renameIfDuplicate(outputDir)
		if err != nil {
			slog.Error("Error while renaming for duplicates", "error", err)
			return NewExtractOperationMsg(processbar.Failed, reqID)
		}

		// Reserve the new extraction directory exclusively. A path that appeared
		// after renameIfDuplicate must not become an existing extraction target.
		err = os.Mkdir(
			outputDir,
			utils.ExtractedDirMode,
		)
		if err != nil {
			slog.Error("Error while making directory for extracting files", "error", err)
			return NewExtractOperationMsg(processbar.Failed, reqID)
		}
		err = extractCompressFile(item, outputDir, &m.processBarModel)
		if err != nil {
			slog.Error("Error extract file", "error", err)
			return NewExtractOperationMsg(processbar.Failed, reqID)
		}
		return NewExtractOperationMsg(processbar.Successful, reqID)
	}
}

func checkFileReadable(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	return nil
}

func (m *model) chooserPaths() []string {
	panel := m.getFocusedFilePanel()
	if panel.SelectedCount() > 0 {
		return panel.GetSelectedLocationsSortedAsVisible()
	}
	if panel.Empty() {
		return nil
	}
	return []string{panel.GetFocusedItem().Location}
}

func (m *model) chooserFileWriteAndQuit(paths ...string) error {
	// Attempt to write to the file
	err := os.WriteFile(variable.ChooserFile, []byte(strings.Join(paths, "\n")), utils.ConfigFilePerm)
	if err != nil {
		return err
	}
	m.modelQuitState = quitInitiated
	return nil
}

// Open file with default editor
func (m *model) openFileWithEditor() tea.Cmd {
	panel := m.getFocusedFilePanel()
	// Check if panel is empty
	if panel.Empty() {
		return nil
	}

	if variable.ChooserFile != "" {
		err := m.chooserFileWriteAndQuit(m.chooserPaths()...)
		if err == nil {
			return nil
		}
		// Continue with preview if file is not writable
		slog.Error("Error while writing to chooser file, continuing with open via file editor", "error", err)
	}

	editor := common.Config.Editor
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}

	// Make sure there is an editor
	if editor == "" {
		if runtime.GOOS == utils.OsWindows {
			editor = "notepad"
		} else {
			editor = "nano"
		}
	}

	// Split the editor command into command and arguments
	parts := strings.Fields(editor)
	cmd := parts[0]

	//nolint:gocritic // appendAssign: intentionally creating a new slice
	args := append(parts[1:], panel.GetFocusedItem().Location)

	c := exec.Command(cmd, args...) //nolint:gosec // Editor command is intentionally user-configurable.

	return tea.ExecProcess(c, func(err error) tea.Msg {
		return editorFinishedMsg{err}
	})
}

// Open directory with default editor
func (m *model) openDirectoryWithEditor() tea.Cmd {
	if variable.ChooserFile != "" {
		err := m.chooserFileWriteAndQuit(m.getFocusedFilePanel().Location)
		if err == nil {
			return nil
		}
		// Continue with preview if file is not writable
		slog.Error("Error while writing to chooser file, continuing with open via directory editor", "error", err)
	}

	editor := common.Config.DirEditor

	if editor == "" {
		switch runtime.GOOS {
		case utils.OsWindows:
			editor = "explorer"
		case utils.OsDarwin:
			editor = "open"
		default:
			editor = "vi"
		}
	}

	// Split the editor command into command and arguments
	parts := strings.Fields(editor)
	cmd := parts[0]
	//nolint:gocritic // appendAssign: intentionally creating a new slice
	args := append(parts[1:], m.getFocusedFilePanel().Location)

	c := exec.Command(cmd, args...)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return editorFinishedMsg{err}
	})
}

// Copy file path
// TODO: This is also an IO operations, do it via tea.Cmd
func (m *model) copyPath() {
	pathText := m.copyPathText()
	if pathText == "" {
		return
	}

	if err := m.writeClipboard(pathText); err != nil {
		slog.Error("Error while copy path", "error", err)
	}
}

func (m *model) copyPathText() string {
	panel := m.getFocusedFilePanel()

	if panel.Empty() {
		return ""
	}

	if panel.PanelMode == filepanel.SelectMode && panel.SelectedCount() > 0 {
		return strings.Join(panel.GetSelectedLocationsSortedAsVisible(), "\n")
	}

	return panel.GetFocusedItem().Location
}

// TODO: This is also an IO operations, do it via tea.Cmd
func (m *model) copyPWD() {
	panel := m.getFocusedFilePanel()
	if err := m.writeClipboard(panel.Location); err != nil {
		slog.Error("Error while copy present working directory", "error", err)
	}
}

func (m *model) writeClipboard(text string) error {
	if m.clipboardWriter != nil {
		return m.clipboardWriter(text)
	}

	return clipboard.WriteAll(text)
}
