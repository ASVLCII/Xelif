package internal

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/yorukot/superfile/src/internal/catalog"
	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui/globalsearch"
)

func loadCatalog(path string) *catalog.Catalog {
	model, err := catalog.Open(path)
	if err == nil {
		return model
	}
	// Open already initializes genuinely missing catalogs. Resetting any other
	// load failure would let the next Save overwrite unreadable or newer data.
	slog.Error("Catalog unavailable; existing storage preserved", "path", path, "error", err)
	return nil
}

func (m *model) executeCatalogAction(action common.CatalogAction) (string, tea.Cmd, error) {
	if m.catalogModel == nil {
		return "", nil, errors.New("tag and collection storage is unavailable; existing catalog preserved (see log)")
	}
	paths := m.gitOperationPaths()
	switch action.Operation {
	case common.TagAdd, common.TagRemove:
		message, err := m.updateCatalogTags(paths, action)
		return message, nil, err
	case common.TagList:
		tags := m.catalogModel.ListTags()
		if len(tags) == 0 {
			return "No tags", nil, nil
		}
		return "Tags: " + strings.Join(tags, ", "), nil, nil
	case common.TagFind:
		matches, err := m.catalogModel.PathsByAllTags(action.Name)
		if err != nil {
			return "", nil, err
		}
		m.openCatalogResults("Tag: "+action.Name, matches)
		return fmt.Sprintf("Found %d tagged path(s)", len(matches)), nil, nil
	case common.CollectionSave:
		if len(paths) == 0 {
			return "", nil, errors.New("select at least one item")
		}
		if err := m.catalogModel.CreateCollection(action.Name, paths); err != nil {
			return "", nil, err
		}
		return "Saved collection: " + action.Name, nil, m.catalogModel.Save()
	case common.CollectionOpen:
		matches, err := m.catalogModel.GetCollection(action.Name)
		if err != nil {
			return "", nil, err
		}
		m.openCatalogResults("Collection: "+action.Name, matches)
		return fmt.Sprintf("Opened collection with %d path(s)", len(matches)), nil, nil
	case common.CollectionList:
		names := m.catalogModel.ListCollections()
		if len(names) == 0 {
			return "No collections", nil, nil
		}
		return "Collections: " + strings.Join(names, ", "), nil, nil
	case common.CollectionDelete:
		if err := m.catalogModel.DeleteCollection(action.Name); err != nil {
			return "", nil, err
		}
		return "Deleted collection: " + action.Name, nil, m.catalogModel.Save()
	default:
		return "", nil, errors.New("unknown catalog operation: " + string(action.Operation))
	}
}

func (m *model) updateCatalogTags(paths []string, action common.CatalogAction) (string, error) {
	update := m.catalogModel.AddTag
	message := "Tagged %d item(s): %s"
	if action.Operation == common.TagRemove {
		update = m.catalogModel.RemoveTag
		message = "Removed tag from %d item(s): %s"
	}
	for _, path := range paths {
		if err := update(path, action.Name); err != nil {
			return "", err
		}
	}
	return fmt.Sprintf(message, len(paths), action.Name), m.catalogModel.Save()
}

func (m *model) openCatalogResults(title string, paths []string) {
	results := make([]globalsearch.Result, len(paths))
	for index, path := range paths {
		results[index] = globalsearch.Result{Path: path}
	}
	m.globalSearchModal.OpenResults(title, results)
	m.setGlobalSearchModelSize()
}

// Paths may disappear after a collection is saved. Check only the selected
// item, rather than statting every saved path on open.
func (m *model) revealCatalogPath(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	directory := path
	if !info.IsDir() {
		directory = filepath.Dir(path)
	}
	if err := m.updateCurrentFilePanelDir(directory); err != nil {
		return err
	}
	panel := m.getFocusedFilePanel()
	panel.UpdateElementsIfNeeded(true, m.fileModel.DisplayDotFiles)
	if !info.IsDir() {
		index := panel.FindElementIndexByLocation(path)
		if index < 0 {
			return errors.New("saved path is hidden by the current file filter or hidden-file setting")
		}
		panel.FocusIndex(index)
	}
	return nil
}

func (m *model) updateCatalogMoves(moves []pathMove) {
	if m.catalogModel == nil || len(moves) == 0 {
		return
	}
	for _, move := range moves {
		if err := m.catalogModel.UpdatePath(move.from, move.to); err != nil {
			slog.Warn("Could not update catalog path", "from", move.from, "to", move.to, "error", err)
			return
		}
	}
	if err := m.catalogModel.Save(); err != nil {
		slog.Warn("Could not save catalog path updates", "error", err)
	}
}
