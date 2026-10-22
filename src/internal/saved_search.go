package internal

import (
	"errors"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/yorukot/superfile/src/internal/common"
)

func (m *model) executeSavedSearchAction(action common.SearchAction) (string, tea.Cmd, error) {
	if m.savedSearchStore == nil {
		return "", nil, errors.New("saved search storage is unavailable")
	}
	switch action.Operation {
	case common.SearchSave:
		return "Saved search: " + action.Name, nil, m.savedSearchStore.Save(action.Name, action.Query)
	case common.SearchRun:
		search, err := m.savedSearchStore.Get(action.Name)
		if err != nil {
			return "", nil, err
		}
		cmd := m.globalSearchModal.OpenQuery(m.getFocusedFilePanel().Location, search.Query)
		m.setGlobalSearchModelSize()
		return "Running saved search: " + search.Name, cmd, nil
	case common.SearchDelete:
		return "Deleted saved search: " + action.Name, nil, m.savedSearchStore.Delete(action.Name)
	case common.SearchList:
		searches, err := m.savedSearchStore.List()
		if err != nil {
			return "", nil, err
		}
		if len(searches) == 0 {
			return "No saved searches", nil, nil
		}
		names := make([]string, len(searches))
		for index, search := range searches {
			names[index] = search.Name
		}
		return "Saved searches: " + strings.Join(names, ", "), nil, nil
	default:
		return "", nil, errors.New("unknown saved search operation: " + string(action.Operation))
	}
}
