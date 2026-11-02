package session

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"unicode/utf8"
)

const maxJSONDepth = 16

func readStateFile(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("load %q: %w: %w", path, ErrNotFound, err)
		}
		return nil, fmt.Errorf("load %q: %w", path, err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, MaxStateFileBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", path, err)
	}
	if len(data) > MaxStateFileBytes {
		return nil, fmt.Errorf("load %q: %w: file exceeds %d bytes", path, ErrCorrupt, MaxStateFileBytes)
	}
	return data, nil
}

func stateVersion(data []byte, path string) (int, error) {
	if !utf8.Valid(data) {
		return 0, fmt.Errorf("decode %q: %w: invalid UTF-8", path, ErrCorrupt)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := checkUniqueJSONValue(decoder, 0); err != nil {
		return 0, fmt.Errorf("decode %q: %w: %w", path, ErrCorrupt, err)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return 0, fmt.Errorf("decode %q: %w: %w", path, ErrCorrupt, err)
	}
	raw, present := object["version"]
	if !present || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return 0, fmt.Errorf("decode %q: %w: missing schema version", path, ErrCorrupt)
	}
	var version int
	if err := json.Unmarshal(raw, &version); err != nil {
		return 0, fmt.Errorf("decode %q: %w: invalid schema version: %w", path, ErrCorrupt, err)
	}
	return version, nil
}

// Reject ambiguous fields rather than silently selecting the final occurrence.
// In particular a duplicate version must not bypass future-version protection.
func checkUniqueJSONValue(decoder *json.Decoder, depth int) error {
	if depth > maxJSONDepth {
		return errors.New("JSON nesting exceeds durable-state limit")
	}
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, compound := token.(json.Delim)
	if !compound {
		return nil
	}
	if delimiter == '{' {
		return checkUniqueJSONObject(decoder, depth)
	}
	if delimiter != '[' {
		return errors.New("unexpected JSON delimiter")
	}
	for decoder.More() {
		if childErr := checkUniqueJSONValue(decoder, depth+1); childErr != nil {
			return childErr
		}
	}
	_, err = decoder.Token()
	return err
}

func checkUniqueJSONObject(decoder *json.Decoder, depth int) error {
	seen := make(map[string]bool)
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		key, valid := token.(string)
		if !valid || seen[key] {
			return errors.New("invalid or duplicate JSON object field")
		}
		seen[key] = true
		if err := checkUniqueJSONValue(decoder, depth+1); err != nil {
			return err
		}
	}
	_, err := decoder.Token()
	return err
}

func decodeState(data []byte, path string) (State, error) {
	version, err := stateVersion(data, path)
	if err != nil {
		return State{}, err
	}
	if version != CurrentVersion {
		return State{}, fmt.Errorf(
			"load %q: %w: layout version %d, want %d",
			path,
			ErrUnsupportedVersion,
			version,
			CurrentVersion,
		)
	}
	var state State
	if err := decodeKnownState(data, &state, path); err != nil {
		return State{}, err
	}
	if err := validateState(state); err != nil {
		return State{}, fmt.Errorf("load %q: %w: %w", path, ErrCorrupt, err)
	}
	return state, nil
}

func decodeKnownState(data []byte, target any, path string) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode %q: %w: %w", path, ErrCorrupt, err)
	}
	return nil // stateVersion already rejected trailing JSON values.
}

func validateTab(tab TabState) error {
	if err := validateLocation(tab.Location); err != nil {
		return err
	}
	for _, history := range [][]string{tab.BackHistory, tab.ForwardHistory} {
		if len(history) > MaxHistoryEntries {
			return fmt.Errorf("%w: navigation history exceeds %d entries", ErrInvalidState, MaxHistoryEntries)
		}
		for _, location := range history {
			if err := validateLocation(location); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateLocation(location string) error {
	if len(location) > MaxLocationBytes || !utf8.ValidString(location) || strings.ContainsRune(location, 0) {
		return fmt.Errorf("%w: invalid or overlong location", ErrInvalidState)
	}
	return nil
}
