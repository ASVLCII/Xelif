package metadataedit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	journalVersion            = 1
	journalStateReady         = "ready"
	journalStateRestored      = "restored"
	maxJournalSize            = 64 << 10
	privateJournalPermissions = fs.FileMode(0o600)
)

type journal struct {
	Version int             `json:"version"`
	State   string          `json:"state"`
	Target  string          `json:"target"`
	Before  journalMetadata `json:"before"`
	After   journalMetadata `json:"after"`
}

type journalMetadata struct {
	ModifiedTime        time.Time `json:"modified_time"`
	AccessTime          time.Time `json:"access_time"`
	Permissions         uint32    `json:"permissions"`
	PermissionsEditable bool      `json:"permissions_editable"`
}

func journalFromPreview(preview PreviewResult) journal {
	return journal{
		Version: journalVersion,
		State:   journalStateReady,
		Target:  preview.Before.Path,
		Before:  metadataToJournal(preview.Before),
		After:   metadataToJournal(preview.After),
	}
}

func metadataToJournal(value Metadata) journalMetadata {
	return journalMetadata{
		ModifiedTime:        value.ModifiedTime,
		AccessTime:          value.AccessTime,
		Permissions:         uint32(value.Permissions.Perm()),
		PermissionsEditable: value.PermissionsEditable,
	}
}

func (value journalMetadata) metadata(path string) Metadata {
	return Metadata{
		Path:                path,
		ModifiedTime:        value.ModifiedTime,
		AccessTime:          value.AccessTime,
		Permissions:         fs.FileMode(value.Permissions),
		PermissionsEditable: value.PermissionsEditable,
	}
}

func validateJournalPath(path, target string) (string, error) {
	if path == "" || strings.IndexByte(path, 0) >= 0 {
		return "", newError(
			KindInvalidInput,
			"validate journal",
			path,
			errors.New("an explicit journal path is required"),
		)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", newError(KindInvalidInput, "validate journal", path, err)
	}
	absPath = filepath.Clean(absPath)
	if filepath.Clean(target) == absPath {
		return "", newError(
			KindInvalidInput,
			"validate journal",
			path,
			errors.New("journal path must differ from target path"),
		)
	}
	return absPath, nil
}

func encodeJournal(value journal) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func createJournal(path string, value journal) error {
	data, err := encodeJournal(value)
	if err != nil {
		return newError(KindJournalIO, "encode journal", path, err)
	}
	tempPath, err := writePrivateTemp(path, data)
	if err != nil {
		return err
	}
	linkErr := os.Link(tempPath, path)
	if linkErr != nil {
		cleanupErr := os.Remove(tempPath)
		if errors.Is(linkErr, fs.ErrExist) {
			return newError(KindJournalExists, "create journal", path, errors.Join(linkErr, cleanupErr))
		}
		return newError(KindJournalIO, "create journal", path, errors.Join(linkErr, cleanupErr))
	}
	if err := os.Remove(tempPath); err != nil {
		return newError(KindJournalIO, "remove journal temp", path, err)
	}
	if err := syncDirectory(filepath.Dir(path)); err != nil {
		return newError(KindJournalIO, "sync journal directory", path, err)
	}
	return nil
}

func writePrivateTemp(destination string, data []byte) (string, error) {
	dir := filepath.Dir(destination)
	file, err := os.CreateTemp(dir, ".metadataedit-*.tmp")
	if err != nil {
		return "", newError(KindJournalIO, "create journal temp", destination, err)
	}
	path := file.Name()
	if err := file.Chmod(privateJournalPermissions); err != nil {
		return "", privateTempError(file, path, destination, "secure journal temp", err)
	}
	if _, err := file.Write(data); err != nil {
		return "", privateTempError(file, path, destination, "write journal temp", err)
	}
	if err := file.Sync(); err != nil {
		return "", privateTempError(file, path, destination, "sync journal temp", err)
	}
	if err := file.Close(); err != nil {
		cleanupErr := os.Remove(path)
		return "", newError(KindJournalIO, "close journal temp", destination, errors.Join(err, cleanupErr))
	}
	return path, nil
}

func privateTempError(file *os.File, path, destination, operation string, cause error) error {
	cleanupErr := errors.Join(file.Close(), os.Remove(path))
	return newError(KindJournalIO, operation, destination, errors.Join(cause, cleanupErr))
}

func replaceJournal(path string, value journal) error {
	data, err := encodeJournal(value)
	if err != nil {
		return newError(KindJournalIO, "encode journal", path, err)
	}
	tempPath, err := writePrivateTemp(path, data)
	if err != nil {
		return err
	}
	renameErr := os.Rename(tempPath, path)
	if renameErr != nil {
		cleanupErr := os.Remove(tempPath)
		return newError(KindJournalIO, "replace journal", path, errors.Join(renameErr, cleanupErr))
	}
	if err := syncDirectory(filepath.Dir(path)); err != nil {
		return newError(KindJournalIO, "sync journal directory", path, err)
	}
	return nil
}

func readJournal(path string) (journal, error) {
	file, err := os.Open(path)
	if err != nil {
		return journal{}, newError(KindJournalIO, "open journal", path, err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxJournalSize+1))
	if err != nil {
		return journal{}, newError(KindJournalIO, "read journal", path, err)
	}
	if len(data) > maxJournalSize {
		return journal{}, newError(
			KindCorruptJournal,
			"read journal",
			path,
			errors.New("journal exceeds size limit"),
		)
	}
	var envelope struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return journal{}, newError(KindCorruptJournal, "decode journal", path, err)
	}
	if envelope.Version != journalVersion {
		return journal{}, newError(
			KindJournalVersion,
			"decode journal",
			path,
			fmt.Errorf("unsupported version %d", envelope.Version),
		)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var value journal
	if err := decoder.Decode(&value); err != nil {
		return journal{}, newError(KindCorruptJournal, "decode journal", path, err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("journal contains multiple JSON values")
		}
		return journal{}, newError(KindCorruptJournal, "decode journal", path, err)
	}
	if err := validateJournal(value); err != nil {
		return journal{}, newError(KindCorruptJournal, "validate journal", path, err)
	}
	return value, nil
}

func validateJournal(value journal) error {
	if value.State != journalStateReady && value.State != journalStateRestored {
		return fmt.Errorf("invalid state %q", value.State)
	}
	if value.Target == "" || !filepath.IsAbs(value.Target) || filepath.Clean(value.Target) != value.Target {
		return errors.New("target must be a clean absolute path")
	}
	for name, metadata := range map[string]journalMetadata{"before": value.Before, "after": value.After} {
		if err := validateTime(metadata.ModifiedTime); err != nil {
			return fmt.Errorf("%s modified time: %w", name, err)
		}
		if err := validateTime(metadata.AccessTime); err != nil {
			return fmt.Errorf("%s access time: %w", name, err)
		}
		if metadata.Permissions > uint32(maximumPermissions) {
			return fmt.Errorf("%s permissions exceed 0777", name)
		}
	}
	return nil
}

// Restore reapplies a journal's original metadata and atomically marks it restored.
// A successfully restored journal cannot be replayed.
func Restore(ctx context.Context, journalPath string) (PreviewResult, error) {
	return restore(ctx, journalPath, operatingSystemFileOps())
}

func restore(ctx context.Context, journalPath string, ops fileOps) (PreviewResult, error) {
	if err := checkContext(ctx, "restore", journalPath); err != nil {
		return PreviewResult{}, err
	}
	validatedPath, err := validateJournalPath(journalPath, "")
	if err != nil {
		return PreviewResult{}, err
	}
	value, err := readJournal(validatedPath)
	if err != nil {
		return PreviewResult{}, err
	}
	if value.State == journalStateRestored {
		return PreviewResult{}, newError(
			KindAlreadyRestored,
			"restore",
			validatedPath,
			errors.New("journal has already been restored"),
		)
	}
	if value.Before.PermissionsEditable && !permissionsSupported {
		return PreviewResult{}, newError(
			KindUnsupported,
			"restore",
			value.Target,
			errors.New("journal requires permission changes unsupported on this platform"),
		)
	}
	current, err := Inspect(ctx, value.Target)
	if err != nil {
		return PreviewResult{}, err
	}
	original := value.Before.metadata(value.Target)
	changes := changeSet{permissions: value.Before.PermissionsEditable, times: true}
	mutationErr := applyChanges(ctx, value.Target, current, original, changes, ops)
	if mutationErr != nil {
		return PreviewResult{}, mutationErr
	}
	contextErr := checkContext(ctx, "restore", value.Target)
	if contextErr != nil {
		if rollbackErr := rollbackChanges(value.Target, current, changes, ops); rollbackErr != nil {
			return PreviewResult{}, newError(
				KindRollback,
				"restore cancellation",
				value.Target,
				errors.Join(contextErr, rollbackErr),
			)
		}
		return PreviewResult{}, contextErr
	}
	value.State = journalStateRestored
	replaceErr := replaceJournal(validatedPath, value)
	if replaceErr != nil {
		if rollbackErr := rollbackChanges(value.Target, current, changes, ops); rollbackErr != nil {
			return PreviewResult{}, newError(
				KindRollback,
				"restore journal rollback",
				value.Target,
				errors.Join(replaceErr, rollbackErr),
			)
		}
		return PreviewResult{}, replaceErr
	}
	actual, err := Inspect(ctx, value.Target)
	if err != nil {
		return PreviewResult{}, err
	}
	return PreviewResult{Before: current, After: actual}, nil
}
