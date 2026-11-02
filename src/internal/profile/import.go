package profile

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

var (
	ErrImportPending = errors.New("an interrupted import needs explicit recovery")
	ErrChanged       = errors.New("profile changed after import preparation; existing files preserved")
	importMu         sync.Mutex //nolint:gochecknoglobals // Serializes imports across process-wide profile paths.
)

// ImportItem describes a candidate without exposing its contents. Unsupported
// or corrupt source files remain at the source and are never activated.
type ImportItem struct {
	Kind            string `json:"kind"`
	Name            string `json:"name,omitempty"`
	Source          string `json:"source"`
	Destination     string `json:"destination"`
	Validation      string `json:"validation"`
	Issue           string `json:"issue,omitempty"`
	ReplaceExisting bool   `json:"replace_existing"`
	SHA256          string `json:"sha256,omitempty"`
}

type ImportPreview struct {
	Found         bool         `json:"found"`
	ReadyCount    int          `json:"ready_count"`
	RejectedCount int          `json:"rejected_count"`
	Items         []ImportItem `json:"items"`
	Excluded      []string     `json:"excluded"`
}

type ImportResult struct {
	Imported        []string     `json:"imported"`
	Rejected        []ImportItem `json:"rejected"`
	BackupDir       string       `json:"backup_dir,omitempty"`
	Recovered       bool         `json:"recovered"`
	SourcePreserved bool         `json:"source_preserved"`
}

type candidate struct{ kind, name string }

const validationValid = "valid"

// PreviewLegacyImport detects and validates only known portable preferences,
// themes, bindings, favourites, searches, catalogs, sessions and workspaces.
// It performs no writes and does not inspect credentials or plugin manifests.
func PreviewLegacyImport(source, destination Paths) (ImportPreview, error) {
	source, destination, err := importPaths(source, destination)
	if err != nil {
		return ImportPreview{}, err
	}
	preview := ImportPreview{
		Items: []ImportItem{},
		Excluded: []string{
			"credentials and remote accounts",
			"caches and logs",
			"undo and operation journals",
			"executable command-plugin manifests",
		},
	}
	candidates, err := importCandidates(source)
	if err != nil {
		return preview, err
	}
	seen := make(map[string]bool)
	for _, entry := range candidates {
		item, present, itemErr := previewCandidate(source, destination, entry, seen)
		if itemErr != nil {
			return preview, itemErr
		}
		if !present {
			continue
		}
		preview.Items = append(preview.Items, item)
		if item.Validation == validationValid {
			preview.ReadyCount++
		} else {
			preview.RejectedCount++
		}
	}
	preview.Found = len(preview.Items) > 0
	return preview, nil
}

func previewCandidate(source, destination Paths, entry candidate, seen map[string]bool) (ImportItem, bool, error) {
	from, err := candidatePath(source, entry)
	if err != nil {
		return ImportItem{}, false, err
	}
	present, err := exists(from)
	if err != nil {
		return ImportItem{}, false, err
	}
	if !present {
		return ImportItem{}, false, nil
	}
	to, err := candidatePath(destination, entry)
	if err != nil {
		return ImportItem{}, false, err
	}
	key := pathKey(to)
	if seen[key] {
		return ImportItem{}, false, errors.New("import destinations overlap")
	}
	seen[key] = true
	item := ImportItem{
		Kind: entry.kind, Name: entry.name, Source: from, Destination: to, Validation: validationValid,
	}
	data, readErr := readRegularFile(from)
	if readErr != nil {
		item.Validation, item.Issue = "unreadable", "not a readable regular profile file; source preserved"
	} else if validationErr := validateData(entry.kind, data); validationErr != nil {
		item.Validation, item.Issue = "corrupt", "corrupt data; source preserved"
		if errors.Is(validationErr, ErrUnsupported) {
			item.Validation, item.Issue = "unsupported", "unsupported fields or schema version; source preserved"
		}
	} else {
		item.SHA256 = digest(data)
		item.ReplaceExisting, err = exists(to)
		if err != nil {
			return ImportItem{}, false, err
		}
		if err = safePath(to); err == nil && item.ReplaceExisting {
			_, err = readRegularFile(to)
		}
		if err != nil {
			item.Validation, item.Issue = "unsafe", "destination cannot be safely backed up; no replacement"
		}
	}
	return item, true, nil
}

func importCandidates(source Paths) ([]candidate, error) {
	items := []candidate{
		{kindPreferences, ""}, {kindBindings, ""}, {kindFavourites, ""}, {kindRecent, ""},
		{kindSearches, ""}, {kindCatalog, ""}, {kindSession, ""},
		{kindToggle, "toggleDotFile"}, {kindToggle, "toggleFooter"},
	}
	for _, directory := range []struct{ path, kind, extension string }{
		{filepath.Join(source.ConfigDir, "theme"), kindTheme, ".toml"},
		{filepath.Join(source.DataDir, "workspaces"), kindWorkspace, ".json"},
	} {
		if err := safePath(directory.path); err != nil {
			return nil, err
		}
		entries, err := os.ReadDir(directory.path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if filepath.Ext(entry.Name()) == directory.extension && safeName(entry.Name()) {
				items = append(items, candidate{directory.kind, entry.Name()})
			}
		}
	}
	return items, nil
}

func candidatePath(paths Paths, item candidate) (string, error) {
	if item.name == "" {
		return unnamedCandidatePath(paths, item.kind)
	}
	switch item.kind {
	case kindTheme:
		if safeName(item.name) && filepath.Ext(item.name) == ".toml" {
			return filepath.Join(paths.ConfigDir, "theme", item.name), nil
		}
	case kindWorkspace:
		if safeName(item.name) && filepath.Ext(item.name) == ".json" {
			return filepath.Join(paths.DataDir, "workspaces", item.name), nil
		}
	case kindToggle:
		if item.name == "toggleDotFile" || item.name == "toggleFooter" {
			return filepath.Join(paths.DataDir, item.name), nil
		}
	}
	return "", ErrUnsupported
}

func unnamedCandidatePath(paths Paths, kind string) (string, error) {
	switch kind {
	case kindPreferences:
		return paths.ConfigFile, nil
	case kindBindings:
		return paths.HotkeysFile, nil
	case kindFavourites:
		return filepath.Join(paths.DataDir, "pinned.json"), nil
	case kindRecent:
		return filepath.Join(paths.DataDir, "recent.json"), nil
	case kindSearches:
		return filepath.Join(paths.DataDir, "saved-searches.json"), nil
	case kindCatalog:
		return filepath.Join(paths.DataDir, "catalog.json"), nil
	case kindSession:
		return filepath.Join(paths.StateDir, "session.json"), nil
	}
	return "", ErrUnsupported
}

func importPaths(source, destination Paths) (Paths, Paths, error) {
	source, err := normalizePaths(source)
	if err != nil {
		return Paths{}, Paths{}, err
	}
	destination, err = normalizePaths(destination)
	if err != nil {
		return Paths{}, Paths{}, err
	}
	from := []string{source.ConfigDir, source.DataDir, source.StateDir, source.ConfigFile, source.HotkeysFile}
	to := []string{
		destination.ConfigDir,
		destination.DataDir,
		destination.StateDir,
		destination.ConfigFile,
		destination.HotkeysFile,
	}
	for _, a := range from {
		for _, b := range to {
			if pathWithin(a, b) || pathWithin(b, a) {
				return Paths{}, Paths{}, errors.New("source and destination profiles must not overlap")
			}
		}
	}
	return source, destination, nil
}

func pathWithin(parent, child string) bool {
	parent, child = pathKey(parent), pathKey(child)
	return parent == child || strings.HasPrefix(child, parent+string(filepath.Separator))
}

func pathKey(path string) string {
	path = filepath.Clean(path)
	if runtime.GOOS == "windows" {
		return strings.ToLower(path)
	}
	return path
}

type importJournal struct {
	Version     int            `json:"version"`
	ID          string         `json:"id"`
	Destination Paths          `json:"destination"`
	Entries     []journalEntry `json:"entries"`
}

type journalEntry struct {
	Kind         string `json:"kind"`
	Name         string `json:"name,omitempty"`
	SHA256       string `json:"sha256"`
	Existed      bool   `json:"existed"`
	BeforeSHA256 string `json:"before_sha256,omitempty"`
}

// ImportLegacy is an explicit copy operation. All accepted files and existing
// destination backups are staged before any activation. It never moves, edits,
// or deletes a source file. Invalid files are reported separately, not copied.
func ImportLegacy(source, destination Paths) (ImportResult, error) {
	importMu.Lock()
	defer importMu.Unlock()
	result := ImportResult{Imported: []string{}, Rejected: []ImportItem{}, SourcePreserved: true}
	source, destination, err := importPaths(source, destination)
	if err != nil {
		return result, err
	}
	preview, err := PreviewLegacyImport(source, destination)
	if err != nil {
		return result, err
	}
	for _, item := range preview.Items {
		if item.Validation != validationValid {
			result.Rejected = append(result.Rejected, item)
		}
	}
	if preview.ReadyCount == 0 {
		return result, nil
	}
	dir, err := prepareImportDir(destination)
	if err != nil {
		return result, err
	}
	journal, err := stageImport(destination, dir, preview.Items)
	if err != nil {
		return result, err
	}
	return applyImport(destination, dir, journal, result)
}

func prepareImportDir(destination Paths) (string, error) {
	dir := filepath.Join(destination.StateDir, importDirName)
	if err := safePath(dir); err != nil {
		return "", err
	}
	if err := os.MkdirAll(destination.StateDir, privateDirMode); err != nil {
		return "", err
	}
	if err := os.Mkdir(dir, privateDirMode); err != nil {
		if errors.Is(err, os.ErrExist) {
			return "", ErrImportPending
		}
		return "", err
	}
	return dir, nil
}

func stageImport(destination Paths, dir string, items []ImportItem) (importJournal, error) {
	id, err := importID()
	if err != nil {
		return importJournal{}, err
	}
	journal := importJournal{Version: 1, ID: id, Destination: destination, Entries: []journalEntry{}}
	for _, item := range items {
		if item.Validation != validationValid {
			continue
		}
		if err = stageImportItem(dir, len(journal.Entries), item, &journal); err != nil {
			return importJournal{}, err
		}
	}
	data, err := json.MarshalIndent(journal, "", "  ")
	if err != nil {
		return importJournal{}, err
	}
	if err = atomicWrite(filepath.Join(dir, "journal.json"), append(data, '\n')); err != nil {
		return importJournal{}, err
	}
	return journal, nil
}

func stageImportItem(dir string, index int, item ImportItem, journal *importJournal) error {
	data, err := readRegularFile(item.Source)
	if err != nil {
		return err
	}
	if digest(data) != item.SHA256 {
		return ErrChanged
	}
	if err = validateData(item.Kind, data); err != nil {
		return err
	}
	entry := journalEntry{Kind: item.Kind, Name: item.Name, SHA256: digest(data)}
	if err = atomicWrite(stagePath(dir, index), data); err != nil {
		return err
	}
	before, err := readRegularFile(item.Destination)
	if err == nil {
		entry.Existed, entry.BeforeSHA256 = true, digest(before)
		if err = atomicWrite(backupPath(dir, index), before); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	journal.Entries = append(journal.Entries, entry)
	return nil
}

// RecoverImport explicitly resumes a validated prepared import. Incomplete or
// corrupt journals are archived intact and reported, never guessed or activated.
func RecoverImport(destination Paths) (ImportResult, error) {
	importMu.Lock()
	defer importMu.Unlock()
	result := ImportResult{Imported: []string{}, Rejected: []ImportItem{}, Recovered: true, SourcePreserved: true}
	destination, err := normalizePaths(destination)
	if err != nil {
		return result, err
	}
	dir := filepath.Join(destination.StateDir, importDirName)
	present, err := exists(dir)
	if err != nil || !present {
		return result, err
	}
	if err = safePath(dir); err != nil {
		return result, err
	}
	data, err := readRegularFile(filepath.Join(dir, "journal.json"))
	var journal importJournal
	if err == nil {
		err = decodeJSON(data, &journal)
	}
	if err != nil || journal.Version != 1 || len(journal.ID) != 32 || !safeName(journal.ID) {
		id, idErr := importID()
		if idErr != nil {
			return result, idErr
		}
		result.BackupDir, idErr = archiveImport(destination, dir, id+"-incomplete")
		if idErr != nil {
			return result, idErr
		}
		return result, errors.New(
			"incomplete or unsupported import journal preserved in backup directory; no files activated",
		)
	}
	journalDestination, normalizeErr := normalizePaths(journal.Destination)
	if normalizeErr != nil || journalDestination != destination {
		return result, errors.New("import belongs to a different destination profile")
	}
	return applyImport(destination, dir, journal, result)
}

func applyImport(paths Paths, dir string, journal importJournal, result ImportResult) (ImportResult, error) {
	if err := preflightImport(paths, dir, journal); err != nil {
		return result, err
	}
	if err := activateImport(paths, dir, journal, &result); err != nil {
		return result, err
	}
	var err error
	result.BackupDir, err = archiveImport(paths, dir, journal.ID)
	return result, err
}

func preflightImport(paths Paths, dir string, journal importJournal) error {
	seen := make(map[string]bool)
	// Preflight the entire journal before resuming a single activation.
	for index, entry := range journal.Entries {
		target, err := candidatePath(paths, candidate{entry.Kind, entry.Name})
		if err != nil || seen[pathKey(target)] {
			return errors.New("invalid or overlapping import journal targets")
		}
		seen[pathKey(target)] = true
		data, err := readRegularFile(stagePath(dir, index))
		if err != nil || digest(data) != entry.SHA256 {
			return errors.New("staged import data is missing or corrupt; existing files preserved")
		}
		if err = validateData(entry.Kind, data); err != nil {
			return errors.New("staged import data is corrupt or unsupported; not activated")
		}
		if entry.Existed {
			before, backupErr := readRegularFile(backupPath(dir, index))
			if backupErr != nil || digest(before) != entry.BeforeSHA256 {
				return errors.New("import backup is missing or corrupt; existing files preserved")
			}
		}
		if err = checkTarget(target, entry); err != nil {
			return err
		}
	}
	return nil
}

func activateImport(paths Paths, dir string, journal importJournal, result *ImportResult) error {
	for index, entry := range journal.Entries {
		target, err := candidatePath(paths, candidate{entry.Kind, entry.Name})
		if err != nil {
			return err
		}
		if err = checkTarget(target, entry); err != nil {
			return err
		}
		data, err := readRegularFile(stagePath(dir, index))
		if err != nil {
			return err
		}
		if err = atomicWrite(target, data); err != nil {
			return err
		}
		result.Imported = append(result.Imported, target)
	}
	return nil
}

func checkTarget(target string, entry journalEntry) error {
	current, err := readRegularFile(target)
	if errors.Is(err, os.ErrNotExist) && !entry.Existed {
		return nil
	}
	if err != nil {
		return err
	}
	hash := digest(current)
	if hash == entry.SHA256 || (entry.Existed && hash == entry.BeforeSHA256) {
		return nil
	}
	return ErrChanged
}

func archiveImport(paths Paths, dir, id string) (string, error) {
	target := filepath.Join(paths.StateDir, "legacy-import-backups", id)
	if err := safePath(target); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(target), privateDirMode); err != nil {
		return "", err
	}
	if err := os.Rename(dir, target); err != nil {
		return "", err
	}
	return target, nil
}

func stagePath(dir string, index int) string {
	return filepath.Join(dir, "staged", fmt.Sprintf("%04d", index))
}
func backupPath(dir string, index int) string {
	return filepath.Join(dir, "before", fmt.Sprintf("%04d", index))
}

func importID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}
