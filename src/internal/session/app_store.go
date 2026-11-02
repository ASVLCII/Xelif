package session

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

// NewWorkspaceID generates an identity independent of its title or locations.
func NewWorkspaceID() (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate workspace identity: %w", err)
	}
	return "workspace-" + hex.EncodeToString(random[:]), nil
}

// LoadAppLast loads the workspace session. A version-1 session is wrapped in one
// workspace without changing the source file or pruning offline paths.
func (s *Store) LoadAppLast() (AppState, error) {
	data, err := readStateFile(s.lastSessionFile)
	if err != nil {
		return AppState{}, err
	}
	return decodeAppState(data, s.lastSessionFile)
}

// LoadAppBackup explicitly loads the previous verified primary session. It never
// replaces the primary file; callers must keep a corrupt-primary error visible.
func (s *Store) LoadAppBackup() (AppState, error) {
	path := s.lastSessionFile + ".bak"
	data, err := readStateFile(path)
	if err != nil {
		return AppState{}, err
	}
	return decodeAppState(data, path)
}

// SaveAppLast atomically saves the ordered workspace session. Existing corrupt or
// unknown-version data is never overwritten. Before migrating a legacy file its
// exact bytes are preserved in .v1.bak (or a content-addressed backup if occupied).
// A .bak file also retains the previous verified primary for explicit recovery.
func (s *Store) SaveAppLast(state AppState) error {
	if err := validateAppState(state); err != nil {
		return err
	}
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode workspace session: %w", err)
	}
	data = append(data, '\n')
	if len(data) > MaxStateFileBytes {
		return fmt.Errorf("%w: encoded workspace session exceeds %d bytes", ErrInvalidState, MaxStateFileBytes)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	previous, err := readStateFile(s.lastSessionFile)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	if err == nil {
		if _, decodeErr := decodeAppState(previous, s.lastSessionFile); decodeErr != nil {
			return fmt.Errorf("preserve existing last session: %w", decodeErr)
		}
		version, _ := stateVersion(previous, s.lastSessionFile) // Already decoded above.
		if version == CurrentVersion {
			if err := preserveLegacyBackup(s.lastSessionFile, previous); err != nil {
				return fmt.Errorf("preserve legacy session before migration: %w", err)
			}
		}
		if err := s.writeFile(s.lastSessionFile+".bak", previous); err != nil {
			return fmt.Errorf("preserve previous verified session: %w", err)
		}
	}
	if err := s.writeFile(s.lastSessionFile, data); err != nil {
		return fmt.Errorf("save workspace session: %w", err)
	}
	return nil
}

func decodeAppState(data []byte, path string) (AppState, error) {
	version, err := stateVersion(data, path)
	if err != nil {
		return AppState{}, err
	}
	if version == CurrentVersion {
		layout, err := decodeState(data, path)
		if err != nil {
			return AppState{}, err
		}
		const legacyID = "legacy-workspace"
		return AppState{
			Version:           CurrentAppVersion,
			ActiveWorkspaceID: legacyID,
			Workspaces: []WorkspaceState{{
				ID: legacyID, Title: legacyWorkspaceTitle(layout), Layout: layout,
			}},
		}, nil
	}
	if version != CurrentAppVersion {
		return AppState{}, fmt.Errorf(
			"load %q: %w: app version %d, want %d",
			path,
			ErrUnsupportedVersion,
			version,
			CurrentAppVersion,
		)
	}
	var state AppState
	if err := decodeKnownState(data, &state, path); err != nil {
		return AppState{}, err
	}
	if err := validateAppState(state); err != nil {
		return AppState{}, fmt.Errorf("load %q: %w: %w", path, ErrCorrupt, err)
	}
	return state, nil
}

func validateAppState(state AppState) error {
	if state.Version != CurrentAppVersion {
		return fmt.Errorf("%w: app version %d, want %d", ErrUnsupportedVersion, state.Version, CurrentAppVersion)
	}
	if len(state.Workspaces) == 0 || len(state.Workspaces) > MaxWorkspaces {
		return fmt.Errorf("%w: workspace count must be in [1,%d]", ErrInvalidState, MaxWorkspaces)
	}
	seen := make(map[string]bool, len(state.Workspaces))
	activeFound := false
	for index, workspace := range state.Workspaces {
		if !validWorkspaceID(workspace.ID) || seen[workspace.ID] {
			return fmt.Errorf("%w: workspace %d has an invalid or duplicate identity", ErrInvalidState, index)
		}
		seen[workspace.ID] = true
		activeFound = activeFound || workspace.ID == state.ActiveWorkspaceID
		if !validWorkspaceTitle(workspace.Title) {
			return fmt.Errorf("%w: workspace %d has an invalid title", ErrInvalidState, index)
		}
		if err := validateState(workspace.Layout); err != nil {
			return fmt.Errorf("workspace %d: %w", index, err)
		}
	}
	if !activeFound {
		return fmt.Errorf("%w: active workspace identity is not present", ErrInvalidState)
	}
	return nil
}

func validWorkspaceID(id string) bool {
	if id == "" || len(id) > MaxWorkspaceIDBytes {
		return false
	}
	for _, character := range id {
		letter := character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z'
		digit := character >= '0' && character <= '9'
		if !letter && !digit && character != '-' && character != '_' && character != '.' {
			return false
		}
	}
	return true
}

func validWorkspaceTitle(title string) bool {
	if title == "" || len(title) > MaxWorkspaceTitleBytes || !utf8.ValidString(title) ||
		strings.TrimSpace(title) == "" {
		return false
	}
	for _, character := range title {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func legacyWorkspaceTitle(layout State) string {
	const fallback = "Workspace 1"
	if len(layout.Panels) == 0 {
		return fallback
	}
	panel := layout.Panels[layout.FocusedPanel]
	if len(panel.Tabs) == 0 {
		return fallback
	}
	location := strings.TrimRight(strings.ReplaceAll(panel.Tabs[panel.ActiveTab].Location, `\`, "/"), "/")
	title := location[strings.LastIndexByte(location, '/')+1:]
	if title == "." || title == ".." || strings.HasSuffix(title, ":") || !validWorkspaceTitle(title) {
		return fallback
	}
	return title
}

func preserveLegacyBackup(path string, data []byte) error {
	backupPath := path + ".v1.bak"
	matched, err := createImmutableBackup(backupPath, data)
	if err != nil || matched {
		return err
	}
	// Preserve an earlier migration backup as well as this distinct legacy source.
	digest := sha256.Sum256(data)
	backupPath = path + ".v1." + hex.EncodeToString(digest[:]) + ".bak"
	matched, err = createImmutableBackup(backupPath, data)
	if err != nil {
		return err
	}
	if !matched {
		return fmt.Errorf("legacy backup %q already contains different bytes", backupPath)
	}
	return nil
}

func createImmutableBackup(path string, data []byte) (bool, error) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, privateFileMode)
	if errors.Is(err, fs.ErrExist) {
		info, statErr := os.Lstat(path)
		if statErr != nil {
			return false, statErr
		}
		if !info.Mode().IsRegular() {
			return false, fmt.Errorf("legacy backup %q is not a regular file", path)
		}
		existing, readErr := readStateFile(path)
		if readErr != nil {
			return false, readErr
		}
		return bytes.Equal(existing, data), nil
	}
	if err != nil {
		return false, err
	}
	complete := false
	defer func() {
		_ = file.Close()
		if !complete {
			_ = os.Remove(path)
		}
	}()
	if _, err := file.Write(data); err != nil {
		return false, err
	}
	if err := file.Sync(); err != nil {
		return false, err
	}
	if err := file.Close(); err != nil {
		return false, err
	}
	complete = true
	if err := syncDirectory(filepath.Dir(path)); err != nil {
		return false, err
	}
	return true, nil
}
