// Package profile owns explicit, non-destructive Xelif profile setup and import.
// Detection and preview functions are read-only; writes require a separate call.
package profile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/adrg/xdg"

	variable "github.com/yorukot/superfile/src/config"
)

const (
	setupPendingName = "setup-pending"
	initializedName  = "firstUseCheck"
	importDirName    = "legacy-import-pending"
	privateFileMode  = 0o600
	privateDirMode   = 0o700
)

// Paths describes one profile. Explicit file overrides take precedence over
// ConfigDir without relocating themes, data, or state.
type Paths struct {
	ConfigDir   string `json:"config_dir"`
	DataDir     string `json:"data_dir"`
	StateDir    string `json:"state_dir"`
	CacheDir    string `json:"cache_dir"`
	ConfigFile  string `json:"config_file"`
	HotkeysFile string `json:"hotkeys_file"`
}

// ActivePaths returns the running application's paths, including CLI overrides.
func ActivePaths() Paths {
	return Paths{
		ConfigDir: variable.SuperFileMainDir, DataDir: variable.SuperFileDataDir,
		StateDir: variable.SuperFileStateDir, CacheDir: variable.SuperFileCacheDir,
		ConfigFile: variable.ConfigFile, HotkeysFile: variable.HotkeysFile,
	}
}

// LegacyPaths locates upstream-compatible data but does not read or activate it.
func LegacyPaths() Paths {
	return Paths{
		ConfigDir:   filepath.Join(xdg.ConfigHome, "superfile"),
		DataDir:     filepath.Join(xdg.DataHome, "superfile"),
		StateDir:    filepath.Join(xdg.StateHome, "superfile"),
		CacheDir:    filepath.Join(xdg.CacheHome, "superfile"),
		ConfigFile:  filepath.Join(xdg.ConfigHome, "superfile", "config.toml"),
		HotkeysFile: filepath.Join(xdg.ConfigHome, "superfile", "hotkeys.toml"),
	}
}

// Status is a read-only startup assessment. A previously valid profile is not
// forced through setup merely because an explicit setup marker is absent.
type Status struct {
	HasConfig     bool   `json:"has_config"`
	ConfigValid   bool   `json:"config_valid"`
	Initialized   bool   `json:"initialized"`
	PendingSetup  bool   `json:"pending_setup"`
	PendingImport bool   `json:"pending_import"`
	Issue         string `json:"issue,omitempty"`
}

// Inspect does not create files, repair data, or follow profile symlinks.
func Inspect(paths Paths) (Status, error) {
	paths, err := normalizePaths(paths)
	if err != nil {
		return Status{}, err
	}
	status := Status{}
	if status.PendingSetup, err = exists(filepath.Join(paths.DataDir, setupPendingName)); err != nil {
		return status, err
	}
	if status.PendingImport, err = exists(filepath.Join(paths.StateDir, importDirName)); err != nil {
		return status, err
	}
	confirmed, err := exists(filepath.Join(paths.DataDir, initializedName))
	if err != nil {
		return status, err
	}
	data, err := readRegularFile(paths.ConfigFile)
	if errors.Is(err, os.ErrNotExist) {
		status.Initialized = confirmed
		return status, nil
	}
	status.HasConfig = true
	if err != nil {
		status.Issue = "configuration is unreadable or is not a regular profile file"
		return status, err
	}
	if err := validateData(kindPreferences, data); err != nil {
		status.Issue = "configuration is corrupt or unsupported; existing file preserved"
	} else {
		status.ConfigValid = true
	}
	status.Initialized = confirmed || (status.ConfigValid && !status.PendingSetup)
	return status, nil
}

// BeginSetup records that subsequently generated defaults are not an explicit
// appearance choice. Call only for a genuinely fresh profile before defaults.
func BeginSetup(paths Paths) error {
	paths, err := normalizePaths(paths)
	if err != nil {
		return err
	}
	status, err := Inspect(paths)
	if err != nil {
		return err
	}
	if status.Initialized || status.HasConfig || status.PendingSetup {
		return nil
	}
	return atomicWrite(filepath.Join(paths.DataDir, setupPendingName), []byte("pending\n"))
}

func normalizePaths(paths Paths) (Paths, error) {
	for _, root := range []*string{&paths.ConfigDir, &paths.DataDir, &paths.StateDir} {
		if strings.TrimSpace(*root) == "" {
			return Paths{}, errors.New("profile roots must be explicit non-empty paths")
		}
		absolute, err := filepath.Abs(*root)
		if err != nil {
			return Paths{}, fmt.Errorf("resolve profile root: %w", err)
		}
		*root = filepath.Clean(absolute)
	}
	if paths.ConfigFile == "" {
		paths.ConfigFile = filepath.Join(paths.ConfigDir, "config.toml")
	}
	if paths.HotkeysFile == "" {
		paths.HotkeysFile = filepath.Join(paths.ConfigDir, "hotkeys.toml")
	}
	for _, file := range []*string{&paths.ConfigFile, &paths.HotkeysFile} {
		absolute, err := filepath.Abs(*file)
		if err != nil {
			return Paths{}, fmt.Errorf("resolve profile file: %w", err)
		}
		*file = filepath.Clean(absolute)
	}
	return paths, nil
}

func exists(path string) (bool, error) {
	_, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}
