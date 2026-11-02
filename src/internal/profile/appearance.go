package profile

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2/unstable"
)

// SaveAppearance confirms an explicit rich or portable appearance choice.
// Existing TOML text, comments, commands, and unrelated options are preserved.
// It uses the existing nerdfont/plain_mode authorities, not a second setting.
func SaveAppearance(paths Paths, mode string) error {
	if mode != "rich" && mode != "portable" {
		return errors.New("appearance must be rich or portable")
	}
	paths, err := normalizePaths(paths)
	if err != nil {
		return err
	}
	data, err := readRegularFile(paths.ConfigFile)
	if err != nil {
		return err
	}
	if err = validatePreferences(data); err != nil {
		return errors.New("configuration is corrupt or unsupported; no appearance settings changed")
	}
	updated, err := setAppearance(data, mode == "rich")
	if err != nil {
		return err
	}
	if err = validatePreferences(updated); err != nil {
		return err
	}
	if err = atomicWrite(paths.ConfigFile, updated); err != nil {
		return err
	}
	if err = atomicWrite(filepath.Join(paths.DataDir, initializedName), []byte("confirmed\n")); err != nil {
		return err
	}
	if err = os.Remove(filepath.Join(paths.DataDir, setupPendingName)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func setAppearance(data []byte, rich bool) ([]byte, error) {
	type edit struct {
		offset, length int
		value          string
	}
	edits := []edit{}
	values := map[string]bool{"nerdfont": rich, "plain_mode": !rich}
	seen := make(map[string]bool)
	var parser unstable.Parser
	parser.Reset(data)
	inTable := false
	for parser.NextExpression() {
		node := parser.Expression()
		if node.Kind == unstable.Table || node.Kind == unstable.ArrayTable {
			inTable = true
		}
		if inTable || node.Kind != unstable.KeyValue {
			continue
		}
		keys := node.Key()
		if !keys.Next() {
			continue
		}
		key := string(keys.Node().Data)
		if keys.Next() {
			continue
		}
		value, wanted := values[key]
		if !wanted {
			continue
		}
		valueNode := node.Value()
		if valueNode.Kind != unstable.Bool {
			return nil, errors.New("appearance settings must be boolean values")
		}
		// Boolean nodes expose their source through Data; Raw is not populated
		// by go-toml v2.3.1 for these tokens.
		raw := parser.Range(valueNode.Data)
		edits = append(edits, edit{int(raw.Offset), int(raw.Length), strconv.FormatBool(value)})
		seen[key] = true
	}
	if err := parser.Error(); err != nil {
		return nil, err
	}
	sort.Slice(edits, func(i, j int) bool { return edits[i].offset > edits[j].offset })
	updated := append([]byte(nil), data...)
	for _, change := range edits {
		next := append([]byte{}, updated[:change.offset]...)
		next = append(next, change.value...)
		next = append(next, updated[change.offset+change.length:]...)
		updated = next
	}
	// Missing root options must precede any table declarations, not be appended
	// into a user's [open_with] table or mistaken for multiline string contents.
	var prefix strings.Builder
	for _, key := range []string{"nerdfont", "plain_mode"} {
		if !seen[key] {
			prefix.WriteString(key)
			prefix.WriteString(" = ")
			prefix.WriteString(strconv.FormatBool(values[key]))
			prefix.WriteByte('\n')
		}
	}
	return append([]byte(prefix.String()), updated...), nil
}
