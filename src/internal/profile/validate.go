package profile

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/pelletier/go-toml/v2"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/savedsearch"
	"github.com/yorukot/superfile/src/internal/session"
)

var ErrUnsupported = errors.New("unsupported profile data")

const (
	kindPreferences = "preferences"
	kindBindings    = "bindings"
	kindTheme       = "theme"
	kindFavourites  = "favourites"
	kindRecent      = "recent"
	kindSearches    = "saved-searches"
	kindCatalog     = "catalog"
	kindSession     = "session"
	kindWorkspace   = "workspace"
	kindToggle      = "toggle"

	defaultFilePanelNamePercent = 60
)

func decodeTOML(data []byte, target any) error {
	err := toml.NewDecoder(bytes.NewReader(data)).DisallowUnknownFields().Decode(target)
	var unknown *toml.StrictMissingError
	if errors.As(err, &unknown) {
		return ErrUnsupported
	}
	return err
}

func decodeJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		if strings.HasPrefix(err.Error(), "json: unknown field ") {
			return ErrUnsupported
		}
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("trailing JSON data")
	}
	return nil
}

func validateData(kind string, data []byte) error {
	if !utf8.Valid(data) {
		return errors.New("profile data must be UTF-8")
	}
	switch kind {
	case kindPreferences:
		return validatePreferences(data)
	case kindBindings:
		return validateBindings(data)
	case kindTheme:
		return validateTheme(data)
	case kindFavourites, kindRecent:
		return validateFolders(data)
	case kindSession, kindWorkspace:
		return validateSession(data)
	case kindSearches:
		return validateSearches(data)
	case kindCatalog:
		return validateCatalog(data)
	case kindToggle:
		return validateToggle(data)
	default:
		return ErrUnsupported
	}
}

func validateBindings(data []byte) error {
	var typed common.HotkeysType
	if err := decodeTOML(data, &typed); err != nil {
		return err
	}
	var bindings map[string][]string
	if err := toml.Unmarshal(data, &bindings); err != nil {
		return err
	}
	for _, keys := range bindings {
		if len(keys) == 0 || keys[0] == "" {
			return errors.New("a hotkey list is empty")
		}
	}
	return nil
}

func validateTheme(data []byte) error {
	var theme common.ThemeType
	if err := decodeTOML(data, &theme); err != nil {
		return err
	}
	if len(theme.GradientColor) != common.RequiredGradientColorCount {
		return errors.New("theme must contain exactly two gradient colors")
	}
	return nil
}

func validateFolders(data []byte) error {
	var folders []struct {
		Location string `json:"location"`
		Name     string `json:"name"`
	}
	if err := decodeJSON(data, &folders); err != nil {
		return err
	}
	for _, folder := range folders {
		if !validAbsolutePath(folder.Location) {
			return errors.New("saved folder path must be absolute")
		}
	}
	return nil
}

func validateToggle(data []byte) error {
	if len(data) == 0 || string(data) == "true" || string(data) == "false" {
		return nil
	}
	return errors.New("invalid boolean preference")
}

func validatePreferences(data []byte) error {
	// Seed only the fields whose absent zero values fail validation. Missing
	// legacy fields remain the responsibility of the normal default merger.
	config := common.ConfigType{
		FilePanelNamePercent: defaultFilePanelNamePercent,
		BorderTop:            "-", BorderBottom: "-", BorderLeft: "|", BorderRight: "|",
		BorderTopLeft: "+", BorderTopRight: "+", BorderBottomLeft: "+", BorderBottomRight: "+",
	}
	if err := decodeTOML(data, &config); err != nil {
		return err
	}
	if config.Theme != "" && (!safeName(config.Theme) || filepath.Base(config.Theme) != config.Theme) {
		return errors.New("theme name must not contain a path")
	}
	return common.ValidateConfig(&config)
}

func validateSession(data []byte) error {
	var state session.State
	if err := decodeJSON(data, &state); err != nil {
		return err
	}
	if state.Version != session.CurrentVersion {
		return ErrUnsupported
	}
	if !validIndex(state.FocusedPanel, len(state.Panels)) {
		return errors.New("invalid focused panel index")
	}
	for _, panel := range state.Panels {
		if !validIndex(panel.ActiveTab, len(panel.Tabs)) || panel.SortKind < 0 || panel.SortKind > 4 {
			return errors.New("invalid session panel")
		}
		for _, tab := range panel.Tabs {
			for _, path := range append(append([]string{tab.Location}, tab.BackHistory...), tab.ForwardHistory...) {
				if path != "" && !validAbsolutePath(path) {
					return errors.New("session paths must be absolute")
				}
			}
		}
	}
	return nil
}

func validAbsolutePath(path string) bool {
	return filepath.IsAbs(path) && !strings.ContainsRune(path, 0)
}

func validIndex(index, count int) bool {
	return index >= 0 && ((count == 0 && index == 0) || index < count)
}

func validateSearches(data []byte) error {
	var doc struct {
		Version  int                  `json:"version"`
		Searches []savedsearch.Search `json:"searches"`
	}
	if err := decodeJSON(data, &doc); err != nil {
		return err
	}
	if doc.Version != 1 {
		return ErrUnsupported
	}
	seen := make(map[string]bool)
	for _, search := range doc.Searches {
		name := strings.ToLower(search.Name)
		if !safeName(search.Name) || len(search.Name) > 128 || seen[name] ||
			strings.TrimSpace(search.Query) == "" || len(search.Query) > 4096 {
			return errors.New("invalid saved search")
		}
		seen[name] = true
	}
	return nil
}

func validateCatalog(data []byte) error {
	var doc struct {
		Version int `json:"version"`
		Paths   []struct {
			Path string   `json:"path"`
			Tags []string `json:"tags"`
		} `json:"paths"`
		Collections []struct {
			Name  string   `json:"name"`
			Paths []string `json:"paths"`
		} `json:"collections"`
	}
	if err := decodeJSON(data, &doc); err != nil {
		return err
	}
	if doc.Version != 1 {
		return ErrUnsupported
	}
	for _, item := range doc.Paths {
		if !validAbsolutePath(item.Path) {
			return errors.New("catalog path must be absolute")
		}
		for _, tag := range item.Tags {
			if !catalogName(tag) {
				return errors.New("invalid catalog tag")
			}
		}
	}
	seen := make(map[string]bool)
	for _, collection := range doc.Collections {
		if err := validateCollection(collection.Name, collection.Paths, seen); err != nil {
			return err
		}
	}
	return nil
}

func validateCollection(name string, paths []string, seen map[string]bool) error {
	if !catalogName(name) || seen[name] {
		return errors.New("invalid or duplicate collection")
	}
	seen[name] = true
	for _, path := range paths {
		if !validAbsolutePath(path) {
			return errors.New("collection path must be absolute")
		}
	}
	return nil
}

func catalogName(name string) bool {
	if strings.TrimSpace(name) == "" || name == "." || name == ".." {
		return false
	}
	for _, char := range name {
		if char == '/' || char == '\\' || unicode.IsControl(char) {
			return false
		}
	}
	return true
}

func safeName(name string) bool {
	if name == "" || strings.TrimSpace(name) != name || name == "." || name == ".." ||
		strings.HasSuffix(name, ".") {
		return false
	}
	for _, char := range name {
		if unicode.IsControl(char) || strings.ContainsRune(`<>:"/\|?*`, char) {
			return false
		}
	}
	base, _, _ := strings.Cut(name, ".")
	base = strings.ToUpper(base)
	switch base {
	case "CON", "PRN", "AUX", "NUL", "CLOCK$":
		return false
	}
	for index := 1; index <= 9; index++ {
		if base == fmt.Sprintf("COM%d", index) || base == fmt.Sprintf("LPT%d", index) {
			return false
		}
	}
	return true
}
