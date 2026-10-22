package sidebar

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/yorukot/superfile/src/pkg/utils"
)

type PinnedManager struct {
	filePath string
}

func NewPinnedFileManager(filePath string) PinnedManager {
	if err := utils.InitJSONFile(filePath); err != nil {
		slog.Error("Error initializing pinned JSON file", "error", err)
	}

	return PinnedManager{
		filePath: filePath,
	}
}

// Load is read-only. A saved folder may be on an offline disk or network share.
func (mgr *PinnedManager) Load() []directory {
	directories, err := mgr.read()
	if err != nil {
		slog.Error("Could not read favourites; existing storage preserved", "error", err)
		return []directory{}
	}
	return directories
}

func (mgr *PinnedManager) read() ([]directory, error) {
	directories := []directory{}
	jsonData, err := os.ReadFile(mgr.filePath)
	if errors.Is(err, os.ErrNotExist) {
		return directories, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read favourites: %w", err)
	}
	if err := json.Unmarshal(jsonData, &directories); err != nil {
		return nil, fmt.Errorf("parse favourites; existing storage preserved: %w", err)
	}
	if directories == nil {
		directories = []directory{}
	}
	return directories, nil
}

// Save marshals and writes the pinned directories to file.
func (mgr *PinnedManager) Save(dirs []directory) error {
	data, err := json.Marshal(dirs)
	if err != nil {
		return fmt.Errorf("error marshaling pinned directories: %w", err)
	}

	if err := writePinnedFile(mgr.filePath, data); err != nil {
		return fmt.Errorf("error writing pinned directories file: %w", err)
	}

	return nil
}

// Toggle adds or removes a directory from the pinned directories list
func (mgr *PinnedManager) Toggle(dir string) error {
	dirs, err := mgr.read()
	if err != nil {
		return err
	}
	unPinned := false

	for i, other := range dirs {
		if other.Location == dir {
			dirs = append(dirs[:i], dirs[i+1:]...)
			unPinned = true
			break
		}
	}

	if !unPinned {
		info, err := os.Stat(dir)
		if err != nil {
			return fmt.Errorf("open favourite folder: %w", err)
		}
		if !info.IsDir() {
			return errors.New("favourites require a folder, not a file")
		}
		name := filepath.Base(dir)
		if filepath.Dir(dir) == dir {
			name = dir
		}
		dirs = append(dirs, directory{
			Location: dir,
			Name:     name,
		})
	}

	if err := mgr.Save(dirs); err != nil {
		return fmt.Errorf("error saving pinned directories: %w", err)
	}

	return nil
}

func (mgr *PinnedManager) Rename(location, name string) error {
	dirs, err := mgr.read()
	if err != nil {
		return err
	}
	for i := range dirs {
		if dirs[i].Location == location {
			dirs[i].Name = name
			return mgr.Save(dirs)
		}
	}
	return errors.New("favourite is no longer saved")
}

// Write beside the destination, then replace it only after successful writes.
// A failed write must not truncate the existing favourites list.
func writePinnedFile(path string, data []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".favourites-*")
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
		_ = os.Remove(file.Name())
	}()
	if err := file.Chmod(utils.ConfigFilePerm); err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}
