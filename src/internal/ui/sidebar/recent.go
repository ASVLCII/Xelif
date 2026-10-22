package sidebar

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/yorukot/superfile/src/pkg/utils"
)

const maxRecentDirectories = 10

type RecentManager struct {
	filePath string
}

func NewRecentFileManager(filePath string) RecentManager {
	if err := utils.InitJSONFile(filePath); err != nil {
		slog.Error("Error initializing recent directories JSON file", "error", err)
	}

	return RecentManager{filePath: filePath}
}

// Load returns valid recent directories in newest-first order.
func (mgr *RecentManager) Load() []directory {
	directories := []directory{}
	jsonData, err := os.ReadFile(mgr.filePath)
	if err != nil {
		slog.Error("Error reading recent directories file", "error", err)
		return directories
	}

	if err := json.Unmarshal(jsonData, &directories); err != nil {
		slog.Error("Error parsing recent directories data", "error", err)
		return []directory{}
	}

	cleaned := cleanRecentDirectories(directories)
	if !slices.Equal(directories, cleaned) {
		if err := mgr.Save(cleaned); err != nil {
			slog.Error("Error saving cleaned recent directories", "error", err)
		}
	}
	return cleaned
}

func (mgr *RecentManager) Save(dirs []directory) error {
	data, err := json.Marshal(dirs)
	if err != nil {
		return fmt.Errorf("error marshaling recent directories: %w", err)
	}
	if err := os.WriteFile(mgr.filePath, data, utils.ConfigFilePerm); err != nil {
		return fmt.Errorf("error writing recent directories file: %w", err)
	}
	return nil
}

// Add records an existing directory as the most recent entry.
func (mgr *RecentManager) Add(path string) error {
	if path == "" {
		return errors.New("recent directory path cannot be empty")
	}

	path, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve recent directory path: %w", err)
	}
	path = filepath.Clean(path)
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat recent directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("recent path is not a directory: %s", path)
	}

	directories := mgr.Load()
	updated := make([]directory, 0, min(len(directories)+1, maxRecentDirectories))
	updated = append(updated, directory{Location: path, Name: filepath.Base(path)})
	for _, dir := range directories {
		if sameRecentPath(dir.Location, path) {
			continue
		}
		updated = append(updated, dir)
		if len(updated) == maxRecentDirectories {
			break
		}
	}
	return mgr.Save(updated)
}

func cleanRecentDirectories(dirs []directory) []directory {
	cleaned := make([]directory, 0, min(len(dirs), maxRecentDirectories))
	for _, dir := range dirs {
		path, err := filepath.Abs(dir.Location)
		if err != nil {
			continue
		}
		path = filepath.Clean(path)
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			continue
		}

		duplicate := false
		for _, existing := range cleaned {
			if sameRecentPath(existing.Location, path) {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}

		name := dir.Name
		if name == "" {
			name = filepath.Base(path)
		}
		cleaned = append(cleaned, directory{Location: path, Name: name})
		if len(cleaned) == maxRecentDirectories {
			break
		}
	}
	return cleaned
}

func sameRecentPath(left, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if runtime.GOOS == utils.OsWindows {
		return strings.EqualFold(left, right)
	}
	return left == right
}
