package internal

import (
	"fmt"

	variable "github.com/yorukot/superfile/src/config"
	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui/globalsearch"
)

func (m *model) applyGlobalSearchAction(action common.ModelAction) {
	resultAction, ok := action.(globalsearch.OpenResultAction)
	if !ok {
		return
	}

	result := resultAction.Result
	if result.Directory {
		if err := m.updateCurrentFilePanelDir(result.Path); err != nil {
			m.showUserError("Could not open search result", err)
		}
		return
	}

	if variable.ChooserFile != "" {
		if err := m.chooserFileWriteAndQuit(result.Path); err != nil {
			m.showUserError("Could not choose search result", err)
		}
		return
	}
	if err := m.executeOpenPath(result.Path); err != nil {
		m.showUserError("Could not open search result", fmt.Errorf("%s: %w", result.Path, err))
	}
}
