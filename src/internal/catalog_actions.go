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
)

func loadCatalog(path string) *catalog.Catalog {
	model, err := catalog.Open(path)
	if err == nil {
		return model
	}
	slog.Warn("Could not load catalog; starting with an empty in-memory catalog", "error", err)
	model, newErr := catalog.New(path)
	if newErr != nil {
		slog.Error("Could not initialize catalog", "error", newErr)
		return nil
	}
	return model
}

func (m *model) executeCatalogAction(action common.CatalogAction) (string, tea.Cmd, error) {
	if m.catalogModel == nil {
		return "", nil, errors.New("tag and collection storage is unavailable")
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
		m.openFirstCatalogPath(matches)
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
	content := "No matching paths."
	if len(paths) > 0 {
		content = strings.Join(paths, "\n")
	}
	m.textViewer.Open(title, content)
	m.setTextViewerSize()
}

func (m *model) openFirstCatalogPath(paths []string) {
	for _, catalogPath := range paths {
		info, err := os.Stat(catalogPath)
		if err != nil {
			continue
		}
		directory := catalogPath
		if !info.IsDir() {
			directory = filepath.Dir(catalogPath)
		}
		if err := m.updateCurrentFilePanelDir(directory); err != nil {
			continue
		}
		m.getFocusedFilePanel().UpdateElementsIfNeeded(true, m.fileModel.DisplayDotFiles)
		if !info.IsDir() {
			if index := m.getFocusedFilePanel().FindElementIndexByLocation(catalogPath); index >= 0 {
				m.getFocusedFilePanel().FocusIndex(index)
			}
		}
		return
	}
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
