package session

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

const workspaceExtension = ".json"

var (
	// ErrNotFound identifies a missing saved session or workspace.
	ErrNotFound = errors.New("session state not found")
	// ErrCorrupt identifies saved data that is not a valid current-version state.
	ErrCorrupt = errors.New("session state is corrupt")
	// ErrUnsupportedVersion identifies data written with an unknown schema version.
	ErrUnsupportedVersion = errors.New("unsupported session schema version")
	// ErrInvalidState identifies a state whose indexes cannot describe its contents.
	ErrInvalidState = errors.New("invalid session state")
	// ErrInvalidWorkspaceName identifies an unsafe workspace file name.
	ErrInvalidWorkspaceName = errors.New("invalid workspace name")
)

// Store persists a last session and named workspaces at caller-selected paths.
type Store struct {
	lastSessionFile string
	workspaceDir    string
	writeFile       func(string, []byte) error
}

// NewStore constructs a store rooted at lastSessionFile and workspaceDir.
func NewStore(lastSessionFile, workspaceDir string) *Store {
	return &Store{
		lastSessionFile: lastSessionFile,
		workspaceDir:    workspaceDir,
		writeFile:       atomicWriteFile,
	}
}

// LoadLast loads the last saved session.
func (s *Store) LoadLast() (State, error) {
	return loadState(s.lastSessionFile)
}

// SaveLast atomically replaces the last saved session.
func (s *Store) SaveLast(state State) error {
	return s.save(s.lastSessionFile, state)
}

// ListWorkspaces returns valid saved workspace names in lexical order.
func (s *Store) ListWorkspaces() ([]string, error) {
	entries, err := os.ReadDir(s.workspaceDir)
	if errors.Is(err, fs.ErrNotExist) {
		return []string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list workspaces: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.Type().IsRegular() || filepath.Ext(entry.Name()) != workspaceExtension {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), workspaceExtension)
		if validateWorkspaceName(name) == nil {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names, nil
}

// SaveWorkspace atomically creates or replaces a named workspace.
func (s *Store) SaveWorkspace(name string, state State) error {
	path, err := s.workspacePath(name)
	if err != nil {
		return err
	}
	return s.save(path, state)
}

// LoadWorkspace loads a named workspace.
func (s *Store) LoadWorkspace(name string) (State, error) {
	path, err := s.workspacePath(name)
	if err != nil {
		return State{}, err
	}
	return loadState(path)
}

// DeleteWorkspace deletes a named workspace without affecting any other state.
func (s *Store) DeleteWorkspace(name string) error {
	path, err := s.workspacePath(name)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("delete workspace %q: %w: %w", name, ErrNotFound, err)
		}
		return fmt.Errorf("delete workspace %q: %w", name, err)
	}
	if err := syncDirectory(s.workspaceDir); err != nil {
		return fmt.Errorf("sync workspace directory after deleting %q: %w", name, err)
	}
	return nil
}

func (s *Store) save(path string, state State) error {
	if err := validateState(state); err != nil {
		return err
	}
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode session state: %w", err)
	}
	data = append(data, '\n')
	if err := s.writeFile(path, data); err != nil {
		return fmt.Errorf("save session state: %w", err)
	}
	return nil
}

func loadState(path string) (State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return State{}, fmt.Errorf("load %q: %w: %w", path, ErrNotFound, err)
		}
		return State{}, fmt.Errorf("load %q: %w", path, err)
	}

	var state State
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&state); err != nil {
		return State{}, fmt.Errorf("decode %q: %w: %w", path, ErrCorrupt, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("multiple JSON values")
		}
		return State{}, fmt.Errorf("decode %q: %w: %w", path, ErrCorrupt, err)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return State{}, fmt.Errorf("decode %q: %w: %w", path, ErrCorrupt, err)
	}
	version, versionPresent := object["version"]
	if !versionPresent || bytes.Equal(version, []byte("null")) {
		return State{}, fmt.Errorf("decode %q: %w: missing schema version", path, ErrCorrupt)
	}
	if state.Version != CurrentVersion {
		return State{}, fmt.Errorf(
			"load %q: %w: got %d, want %d",
			path,
			ErrUnsupportedVersion,
			state.Version,
			CurrentVersion,
		)
	}
	if err := validateState(state); err != nil {
		return State{}, fmt.Errorf("load %q: %w: %w", path, ErrCorrupt, err)
	}
	return state, nil
}

func validateState(state State) error {
	if state.Version != CurrentVersion {
		return fmt.Errorf("%w: got %d, want %d", ErrUnsupportedVersion, state.Version, CurrentVersion)
	}
	if len(state.Panels) == 0 {
		if state.FocusedPanel != 0 {
			return fmt.Errorf("%w: focused panel %d with no panels", ErrInvalidState, state.FocusedPanel)
		}
		return nil
	}
	if state.FocusedPanel < 0 || state.FocusedPanel >= len(state.Panels) {
		return fmt.Errorf("%w: focused panel %d outside [0,%d)", ErrInvalidState, state.FocusedPanel, len(state.Panels))
	}
	for index, panel := range state.Panels {
		if len(panel.Tabs) == 0 {
			if panel.ActiveTab != 0 {
				return fmt.Errorf(
					"%w: panel %d has active tab %d with no tabs",
					ErrInvalidState,
					index,
					panel.ActiveTab,
				)
			}
			continue
		}
		if panel.ActiveTab < 0 || panel.ActiveTab >= len(panel.Tabs) {
			return fmt.Errorf(
				"%w: panel %d active tab %d outside [0,%d)",
				ErrInvalidState,
				index,
				panel.ActiveTab,
				len(panel.Tabs),
			)
		}
	}
	return nil
}

func (s *Store) workspacePath(name string) (string, error) {
	if err := validateWorkspaceName(name); err != nil {
		return "", err
	}
	return filepath.Join(s.workspaceDir, name+workspaceExtension), nil
}

func validateWorkspaceName(name string) error {
	if name == "" || !utf8.ValidString(name) || strings.TrimSpace(name) != name ||
		name == "." || name == ".." || strings.HasSuffix(name, ".") {
		return fmt.Errorf("%w: %q", ErrInvalidWorkspaceName, name)
	}
	for _, character := range name {
		if unicode.IsControl(character) || strings.ContainsRune(`<>:"/\|?*`, character) {
			return fmt.Errorf("%w: %q", ErrInvalidWorkspaceName, name)
		}
	}
	base := name
	if index := strings.IndexByte(base, '.'); index >= 0 {
		base = base[:index]
	}
	base = strings.TrimRight(base, " .")
	if isReservedWorkspaceName(base) {
		return fmt.Errorf("%w: %q", ErrInvalidWorkspaceName, name)
	}
	return nil
}

func isReservedWorkspaceName(name string) bool {
	upper := strings.ToUpper(name)
	switch upper {
	case "CON", "PRN", "AUX", "NUL", "CLOCK$":
		return true
	}
	if len(upper) == 4 && upper[3] >= '1' && upper[3] <= '9' {
		return strings.HasPrefix(upper, "COM") || strings.HasPrefix(upper, "LPT")
	}
	return false
}
