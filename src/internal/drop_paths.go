package internal

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"unicode"

	"github.com/yorukot/superfile/src/internal/ui/notify"
	"github.com/yorukot/superfile/src/pkg/utils"
)

func (m *model) acceptDroppedPaths(content string) bool {
	if m.IsOverlayModelOpen() || m.contextMenu.open || m.fileModel.Renaming ||
		m.sidebarModel.IsRenaming() || m.getFocusedFilePanel().SearchBar.Focused() ||
		m.sidebarModel.SearchBarFocused() {
		return false
	}
	paths := parseDroppedPaths(content)
	if len(paths) == 0 {
		return false
	}
	m.pendingDroppedPaths = paths
	m.notifyModel = notify.New(true, "Copy dropped items?",
		fmt.Sprintf("Copy %d dropped item(s) into %s? Existing names are preserved with a safe suffix.",
			len(paths), m.getFocusedFilePanel().Location), notify.DropPasteAction)
	return true
}

func parseDroppedPaths(content string) []string {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}
	candidates := []string{content}
	if _, err := os.Lstat(strings.Trim(content, `"'`)); err != nil {
		candidates = tokenizeDroppedPaths(content)
	}
	unique := make(map[string]string, len(candidates))
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(strings.Trim(candidate, `"'`))
		if parsed, ok := parseFileURL(candidate); ok {
			candidate = parsed
		}
		if candidate == "" || !filepath.IsAbs(candidate) {
			continue
		}
		absolute, err := filepath.Abs(filepath.Clean(candidate))
		if err != nil {
			continue
		}
		if _, err := os.Lstat(absolute); err != nil {
			continue
		}
		key := absolute
		if runtime.GOOS == utils.OsWindows {
			key = strings.ToLower(key)
		}
		unique[key] = absolute
	}
	paths := make([]string, 0, len(unique))
	for _, path := range unique {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	return paths
}

func tokenizeDroppedPaths(content string) []string {
	var tokens []string
	var builder strings.Builder
	var quote rune
	flush := func() {
		if builder.Len() > 0 {
			tokens = append(tokens, builder.String())
			builder.Reset()
		}
	}
	for _, character := range content {
		switch {
		case quote != 0 && character == quote:
			quote = 0
		case quote == 0 && (character == '"' || character == '\''):
			quote = character
		case quote == 0 && unicode.IsSpace(character):
			flush()
		default:
			builder.WriteRune(character)
		}
	}
	flush()
	return tokens
}

func parseFileURL(value string) (string, bool) {
	parsed, err := url.Parse(value)
	if err != nil || !strings.EqualFold(parsed.Scheme, "file") {
		return "", false
	}
	escapedPath := parsed.EscapedPath()
	if escapedPath == "" && parsed.Opaque != "" {
		escapedPath = parsed.Opaque
	}
	decoded, err := url.PathUnescape(escapedPath)
	if err != nil {
		return "", false
	}
	if runtime.GOOS == utils.OsWindows {
		if parsed.Host != "" {
			if len(parsed.Host) == 2 && parsed.Host[1] == ':' {
				return parsed.Host + filepath.FromSlash(decoded), true
			}
			return `\\` + parsed.Host + filepath.FromSlash(decoded), true
		}
		if len(decoded) >= 3 && decoded[0] == '/' && decoded[2] == ':' {
			decoded = decoded[1:]
		}
	}
	return filepath.FromSlash(decoded), true
}
