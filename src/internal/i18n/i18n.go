// Package i18n provides a deliberately small translation layer for Xelif UI text.
package i18n

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
)

type Language string

const (
	English            Language = "en"
	TraditionalChinese Language = "zh-TW"
)

var ErrUnsupportedLanguage = errors.New("unsupported language")

var translations = map[Language]map[string]string{ //nolint:gochecknoglobals // Immutable translation catalog.
	English: {
		"details": "Details", "close": "Close", "search_everywhere": "Search Everywhere",
		"type_to_search": "Type to search", "searching": "Searching...", "no_results": "No results found",
		"search_filters": `Filters: type:file ext:pdf size:>1m modified:<7d content:"text"`,
		"archive":        "Archive", "open": "Open", "extract": "Extract", "back": "Back",
		"double_click_open": "Double-click Open",
		"remote_files":      "Remote files", "configured_remotes": "Configured remotes", "download": "Download",
		"back_retry": "Back / Retry", "loading": "Loading...", "empty_remote_view": "Nothing here",
	},
	TraditionalChinese: traditionalChineseTranslations(),
}

var defaultBundle = struct { //nolint:gochecknoglobals // Process-wide UI locale selected at startup.
	mu       sync.RWMutex
	language Language
}{language: English}

func SetLanguage(requested string) error {
	language, err := resolve(requested)
	defaultBundle.mu.Lock()
	defaultBundle.language = language
	defaultBundle.mu.Unlock()
	return err
}

func Current() Language {
	defaultBundle.mu.RLock()
	defer defaultBundle.mu.RUnlock()
	return defaultBundle.language
}

func T(key string, args ...any) string {
	defaultBundle.mu.RLock()
	language := defaultBundle.language
	defaultBundle.mu.RUnlock()
	value, ok := translations[language][key]
	if !ok {
		value, ok = translations[English][key]
	}
	if !ok {
		value = key
	}
	if len(args) > 0 {
		return fmt.Sprintf(value, args...)
	}
	return value
}

func Supported() []Language { return []Language{English, TraditionalChinese} }

func resolve(requested string) (Language, error) {
	requested = strings.TrimSpace(requested)
	if requested == "" || strings.EqualFold(requested, "auto") {
		return languageFromEnvironment(), nil
	}
	switch strings.ToLower(strings.ReplaceAll(requested, "_", "-")) {
	case "en", "en-us", "en-gb":
		return English, nil
	case "zh-tw", "zh-hant", "zh-hant-tw":
		return TraditionalChinese, nil
	default:
		return English, fmt.Errorf("%w: %s", ErrUnsupportedLanguage, requested)
	}
}

func languageFromEnvironment() Language {
	for _, name := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		value := strings.ToLower(strings.ReplaceAll(os.Getenv(name), "_", "-"))
		if strings.HasPrefix(value, "zh-tw") || strings.HasPrefix(value, "zh-hant") {
			return TraditionalChinese
		}
	}
	return English
}
