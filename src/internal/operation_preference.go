package internal

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/pkg/utils"
)

func (m *model) handleOperationsShortcut(msg tea.Msg) bool {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok || !slices.Contains(common.Hotkeys.ToggleOperations, key.String()) {
		return false
	}
	if m.spfError.IsOpen() || m.notifyModel.IsOpen() || m.typingModal.open || m.firstUse ||
		m.fileModel.Renaming || m.sidebarModel.IsRenaming() {
		return false
	}
	if m.operations.open {
		m.closeOperations()
	} else {
		m.openOperations()
	}
	return true
}

func (m *model) operationsBlockPickerInput(msg tea.Msg) bool {
	if !m.operations.open {
		return false
	}
	switch msg.(type) {
	case tea.KeyPressMsg, tea.MouseMsg, tea.PasteMsg:
		return true
	default:
		return false
	}
}

func (m *model) restoreOperationsVisibility(path string) {
	m.operationsStatePath = path
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		m.showUserError("Could not restore operation center", err)
		return
	}
	switch strings.TrimSpace(string(data)) {
	case "true":
		m.openOperationsView()
	case "false":
		m.operations.open = false
	default:
		m.showUserError(
			"Could not restore operation center",
			errors.New("invalid visibility preference; existing bytes were preserved"),
		)
	}
}

func (m *model) rememberOperationsVisibility(visible bool) {
	if m.operationsStatePath == "" {
		return
	}
	if err := saveOperationsVisibility(m.operationsStatePath, visible); err != nil {
		if m.operations.open {
			m.operations.notice = "Visibility was not saved: " + common.MakePrintableWithEscCheck(err.Error(), false)
		} else {
			m.showUserError("Could not save operation center visibility", err)
		}
	}
}

// The only persistent state is one boolean, not task history or pending work.
// Publish atomically so an interrupted write leaves the previous choice intact.
func saveOperationsVisibility(path string, visible bool) error {
	if err := os.MkdirAll(filepath.Dir(path), utils.ConfigDirPerm); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".operation-visibility-*")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer func() { _ = file.Close(); _ = os.Remove(temporary) }()
	if _, err := file.WriteString(strconv.FormatBool(visible)); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

func (m *model) closeOperations() {
	m.operations.open = false
	m.rememberOperationsVisibility(false)
}
