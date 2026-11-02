package main

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLocalExtractionVersionFollowsOnlyActiveLocalReplacement(t *testing.T) {
	for _, item := range []struct {
		requirement, replacement, want string
		invalid                        bool
	}{
		{"v0.3.2", "./third_party/xtractr", "v0.3.2", false},
		{"v0.4.1", "./third_party/xtractr", "v0.4.1", false},
		{"v0.3.2", "example.com/other v0.4.1", "", false},
		{"", "./third_party/xtractr", "", true},
	} {
		module := "module example.com/app\n\ngo 1.26\n"
		if item.requirement != "" {
			module += "require golift.io/xtractr " + item.requirement + "\n"
		}
		module += "replace golift.io/xtractr => " + item.replacement + "\n"
		version, err := localExtractionVersion([]byte(module))
		require.Equal(t, item.invalid, err != nil)
		require.Equal(t, item.want, version)
	}
	version, err := localExtractionVersion([]byte("module example.com/app\nrequire golift.io/xtractr v0.3.2\n" +
		"replace golift.io/xtractr v0.4.1 => ./third_party/xtractr\n"))
	require.NoError(t, err)
	require.Empty(t, version)
	_, err = localExtractionVersion([]byte("not a valid module"))
	require.Error(t, err)
}

func TestNormalizeLocalExtractionNoticePreservesLicenseBytes(t *testing.T) {
	license := "MIT License\n\nCopyright 2020-2026 Go Lift\n- Version: Unknown\n](Unknown)\n"
	format := "## golift.io/xtractr\n\n- Version: %s\n- License: [MIT](%s)\n\n```text\n" + license + "```\n"
	section := fmt.Sprintf(format, "Unknown", "Unknown")
	want := fmt.Sprintf(format, "v0.3.2 (locally patched)", "third_party/xtractr/LICENSE")
	require.Equal(t, want, normalizeLocalExtractionNotice(localExtractionModule, section, "v0.3.2"))
	require.Equal(t, section, normalizeLocalExtractionNotice("example.com/other", section, "v0.3.2"))
	require.Equal(t, section, normalizeLocalExtractionNotice(localExtractionModule, section, ""))
	known := fmt.Sprintf(format, "v0.3.2", "https://example.com/LICENSE")
	require.Equal(t, known, normalizeLocalExtractionNotice(localExtractionModule, known, "v0.3.2"))
}

func TestNoticeURLNormalizationLeavesLegalTextUntouched(t *testing.T) {
	section := "## example.com/module\n- License: [MIT](https://example.com/nested\\LICENSE)\n\n```text\n" +
		"Legal text with \\ and - License: [MIT](keep\\this)\n```\n"
	want := "## example.com/module\n- License: [MIT](https://example.com/nested/LICENSE)\n\n```text\n" +
		"Legal text with \\ and - License: [MIT](keep\\this)\n```\n"
	require.Equal(t, want, normalizeNoticeLicenseURLs(section))
}
