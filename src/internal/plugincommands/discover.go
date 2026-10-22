package plugincommands

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	stableIDPattern    = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[._-][a-z0-9]+)*$`)
	envNamePattern     = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	placeholderPattern = regexp.MustCompile(`\{[^{}]*\}`)
)

// Discover reads and validates direct *.json children of an absolute, clean
// directory. It performs no plugin execution.
func Discover(dir string) (*Registry, error) {
	if !filepath.IsAbs(dir) || filepath.Clean(dir) != dir {
		return nil, &DiscoveryError{Dir: dir, Err: errors.New("directory must be an absolute, clean path")}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, &DiscoveryError{Dir: dir, Err: err}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	registry := &Registry{plugins: make(map[string]Manifest)}
	sources := make(map[string]string)
	for _, entry := range entries {
		if !strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		info, infoErr := entry.Info()
		if infoErr != nil {
			return nil, &ManifestError{File: path, Err: infoErr}
		}
		if entry.Type()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return nil, &ManifestError{File: path, Err: errors.New("manifest must be a regular, non-symlink file")}
		}
		if info.Size() > MaxManifestBytes {
			return nil, &ManifestError{File: path, Err: fmt.Errorf("manifest exceeds %d bytes", MaxManifestBytes)}
		}

		manifest, decodeErr := decodeManifest(path)
		if decodeErr != nil {
			return nil, &ManifestError{File: path, Err: decodeErr}
		}
		if field, validationErr := validateManifest(&manifest); validationErr != nil {
			return nil, &ManifestError{File: path, Field: field, Err: validationErr}
		}
		if first, exists := sources[manifest.ID]; exists {
			return nil, &DuplicateIDError{ID: manifest.ID, FirstFile: first, SecondFile: path}
		}
		sources[manifest.ID] = path
		registry.plugins[manifest.ID] = cloneManifest(manifest)
		registry.ordered = append(registry.ordered, manifest.ID)
	}
	sort.Strings(registry.ordered)
	return registry, nil
}

func decodeManifest(path string) (Manifest, error) {
	file, err := os.Open(path)
	if err != nil {
		return Manifest{}, err
	}
	defer file.Close()

	decoder := json.NewDecoder(io.LimitReader(file, MaxManifestBytes+1))
	decoder.DisallowUnknownFields()
	var manifest Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return Manifest{}, errors.New("multiple JSON values are not allowed")
		}
		return Manifest{}, err
	}
	return manifest, nil
}

func validateManifest(manifest *Manifest) (string, error) {
	checks := []struct {
		field    string
		validate func() error
	}{
		{field: "version", validate: func() error { return validateManifestVersion(manifest.Version) }},
		{field: "id", validate: func() error { return validateManifestID(manifest.ID) }},
		{field: "name", validate: func() error { return validateManifestName(manifest.Name) }},
		{field: "executable", validate: func() error { return validateExecutable(manifest.Executable) }},
		{field: manifestArgsField, validate: func() error { return validateArguments(manifest.Args) }},
		{field: "extensions", validate: func() error { return validateExtensions(manifest) }},
		{field: "timeout_ms", validate: func() error { return validateTimeout(manifest.TimeoutMS) }},
		{field: "max_output_bytes", validate: func() error { return validateOutputLimit(manifest.MaxOutputBytes) }},
		{field: "env_allowlist", validate: func() error { return validateEnvironmentAllowlist(manifest.EnvAllowlist) }},
	}
	for _, check := range checks {
		if err := check.validate(); err != nil {
			return check.field, err
		}
	}
	return "", nil
}

func validateManifestVersion(version int) error {
	if version != ManifestVersion {
		return fmt.Errorf("unsupported version %d", version)
	}
	return nil
}

func validateManifestID(id string) error {
	if len(id) > 64 || !stableIDPattern.MatchString(id) {
		return errors.New("must be a lowercase stable ID of at most 64 characters")
	}
	return nil
}

func validateManifestName(name string) error {
	if name != strings.TrimSpace(name) || name == "" || utf8.RuneCountInString(name) > 128 ||
		strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return errors.New("must be non-blank, trimmed, control-free, and at most 128 characters")
	}
	return nil
}

func validateArguments(args []string) error {
	if args == nil {
		return errors.New("must be an explicit JSON array")
	}
	if len(args) > maxManifestArguments {
		return fmt.Errorf("must contain at most %d arguments", maxManifestArguments)
	}
	for _, arg := range args {
		if len(arg) > 4096 || strings.ContainsRune(arg, 0) {
			return errors.New("each argument must be NUL-free and at most 4096 bytes")
		}
		if hasParentTraversal(arg) {
			return errors.New("parent-directory traversal is not allowed")
		}
		if err := validatePlaceholders(arg); err != nil {
			return err
		}
	}
	return nil
}

func validateTimeout(timeoutMS int) error {
	if timeoutMS < 1 || timeoutMS > MaxTimeoutMS {
		return fmt.Errorf("must be between 1 and %d", MaxTimeoutMS)
	}
	return nil
}

func validateOutputLimit(maxOutputBytes int) error {
	if maxOutputBytes < 1 || maxOutputBytes > MaxOutputBytes {
		return fmt.Errorf("must be between 1 and %d", MaxOutputBytes)
	}
	return nil
}

func validateEnvironmentAllowlist(names []string) error {
	if names == nil {
		return errors.New("must be an explicit JSON array")
	}
	return validateEnvironment(names)
}

func validateExecutable(executable string) error {
	if executable == "" || executable != strings.TrimSpace(executable) || len(executable) > 4096 ||
		strings.IndexFunc(executable, unicode.IsControl) >= 0 {
		return errors.New("must be a non-blank, trimmed, NUL-free path or program name")
	}
	if hasParentTraversal(executable) {
		return errors.New("parent-directory traversal is not allowed")
	}
	if filepath.IsAbs(executable) {
		if filepath.Clean(executable) != executable {
			return errors.New("absolute executable path must be clean")
		}
	} else if strings.ContainsAny(executable, `/\\`) ||
		strings.IndexFunc(executable, func(r rune) bool {
			return r == ' ' || r == '\t' || r == '\r' || r == '\n'
		}) >= 0 {
		return errors.New("relative executable must be a single program name, not a command string")
	}
	base := strings.ToLower(filepath.Base(executable))
	base = strings.TrimSuffix(base, filepath.Ext(base))
	switch base {
	case "sh", "bash", "dash", "zsh", "fish", "ksh", "csh", "tcsh", "cmd", "powershell", "pwsh", "wscript", "cscript":
		return errors.New("shell and script-host executables are not allowed")
	}
	return nil
}

func validatePlaceholders(arg string) error {
	if strings.Contains(arg, pathsPlaceholder) && arg != pathsPlaceholder {
		return fmt.Errorf("%s must occupy a complete argument", pathsPlaceholder)
	}
	withoutKnown := placeholderPattern.ReplaceAllStringFunc(arg, func(value string) string {
		switch value {
		case pathPlaceholder, pathsPlaceholder, dirPlaceholder:
			return ""
		default:
			return value
		}
	})
	if strings.ContainsAny(withoutKnown, "{}") {
		return errors.New("contains an unknown or malformed placeholder")
	}
	return nil
}

func validateExtensions(manifest *Manifest) error {
	seen := make(map[string]struct{}, len(manifest.Extensions))
	for index, extension := range manifest.Extensions {
		normalized := strings.ToLower(extension)
		invalidRune := strings.IndexFunc(extension, func(r rune) bool {
			return r == '/' || r == '\\' || r == 0 || r == ' ' || r == '\t' || r == '\r' || r == '\n'
		}) >= 0
		if len(extension) < 2 || len(extension) > 32 || extension[0] != '.' || invalidRune {
			return errors.New("each extension must begin with a dot and contain no path separators or whitespace")
		}
		if _, exists := seen[normalized]; exists {
			return fmt.Errorf("duplicate extension %q", extension)
		}
		seen[normalized] = struct{}{}
		manifest.Extensions[index] = normalized
	}
	return nil
}

func validateEnvironment(names []string) error {
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		if !envNamePattern.MatchString(name) {
			return fmt.Errorf("invalid environment-variable name %q", name)
		}
		canonical := name
		if runtime.GOOS == windowsOS {
			canonical = strings.ToUpper(name)
		}
		if _, exists := seen[canonical]; exists {
			return fmt.Errorf("duplicate environment-variable name %q", name)
		}
		seen[canonical] = struct{}{}
		upper := strings.ToUpper(name)
		credentialWords := []string{
			"TOKEN", "SECRET", "PASSWORD", "CREDENTIAL", "API_KEY", "PRIVATE_KEY", "ACCESS_KEY",
		}
		for _, credentialWord := range credentialWords {
			if strings.Contains(upper, credentialWord) {
				return fmt.Errorf("credential-bearing environment variable %q is not allowed", name)
			}
		}
	}
	return nil
}

func hasParentTraversal(value string) bool {
	for _, segment := range strings.FieldsFunc(value, func(r rune) bool { return r == '/' || r == '\\' }) {
		if segment == ".." {
			return true
		}
	}
	return false
}
