package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"golang.org/x/mod/modfile"
)

type target struct {
	goos   string
	goarch string
}

const (
	goArchAMD64            = "amd64"
	goArchARM64            = "arm64"
	localExtractionModule  = "golift.io/xtractr"
	localExtractionReplace = "./third_party/xtractr"
)

func main() {
	moduleData, err := os.ReadFile("go.mod")
	if err != nil {
		fmt.Fprintf(os.Stderr, "read notice module metadata: %v\n", err)
		os.Exit(1)
	}
	localVersion, err := localExtractionVersion(moduleData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve local notice metadata: %v\n", err)
		os.Exit(1)
	}
	targets := []target{
		{goos: "linux", goarch: goArchAMD64},
		{goos: "linux", goarch: goArchARM64},
		{goos: "darwin", goarch: goArchAMD64},
		{goos: "darwin", goarch: goArchARM64},
		{goos: "windows", goarch: goArchAMD64},
		{goos: "windows", goarch: goArchARM64},
	}
	sections := map[string]string{}

	for _, target := range targets {
		report, err := runGoLicenses(target)
		if err != nil {
			fmt.Fprintf(os.Stderr, "generate notice for %s/%s: %v\n", target.goos, target.goarch, err)
			os.Exit(1)
		}

		for name, section := range parseSections(report) {
			section = normalizeNoticeLicenseURLs(section)
			section = normalizeLocalExtractionNotice(name, section, localVersion)
			existing, ok := sections[name]
			if !ok || shouldReplace(existing, section) {
				sections[name] = section
			}
		}
	}

	names := make([]string, 0, len(sections))
	for name := range sections {
		names = append(names, name)
	}
	sort.Strings(names)

	var output bytes.Buffer
	output.WriteString("# Notices\n\n\n")
	for index, name := range names {
		if index > 0 {
			output.WriteString("\n\n")
		}
		output.WriteString(strings.TrimRight(sections[name], "\n"))
	}
	output.WriteByte('\n')

	if err := os.WriteFile("NOTICE.md", output.Bytes(), 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "write NOTICE.md: %v\n", err)
		os.Exit(1)
	}
}

// The local extraction fork retains its declared upstream base, not an invented
// release version. Resolve metadata from go.mod so dependency updates stay honest.
func localExtractionVersion(data []byte) (string, error) {
	module, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		return "", fmt.Errorf("parse go.mod: %w", err)
	}
	version := ""
	for _, required := range module.Require {
		if required.Mod.Path == localExtractionModule {
			version = required.Mod.Version
		}
	}
	for _, replacement := range module.Replace {
		if replacement.Old.Path != localExtractionModule || replacement.New.Path != localExtractionReplace ||
			replacement.New.Version != "" {
			continue
		}
		if version == "" {
			return "", fmt.Errorf("local replacement %s has no required base version", localExtractionModule)
		}
		if replacement.Old.Version == "" || replacement.Old.Version == version {
			return version, nil
		}
	}
	return "", nil
}

func normalizeLocalExtractionNotice(name, section, version string) string {
	if name != localExtractionModule || version == "" {
		return section
	}
	cut := strings.Index(section, "\n```")
	if cut < 0 {
		return section
	}
	head, body := section[:cut], section[cut:]
	head = strings.ReplaceAll(head, "- Version: Unknown", "- Version: "+version+" (locally patched)")
	head = strings.ReplaceAll(head, "](Unknown)", "](third_party/xtractr/LICENSE)")
	return head + body
}

// go-licenses constructs nested file URLs with native separators on Windows.
// Normalize metadata only: legal text inside the fence is never rewritten.
func normalizeNoticeLicenseURLs(section string) string {
	cut := strings.Index(section, "\n```")
	if cut < 0 {
		return section
	}
	lines := strings.Split(section[:cut], "\n")
	for index, line := range lines {
		if strings.HasPrefix(line, "- License: ") {
			lines[index] = strings.ReplaceAll(line, "\\", "/")
		}
	}
	return strings.Join(lines, "\n") + section[cut:]
}

func runGoLicenses(target target) (string, error) {
	cmd := exec.Command("go-licenses", "report", "./", "--template=notice.tmpl")
	cmd.Env = append(os.Environ(), "GOOS="+target.goos, "GOARCH="+target.goarch)
	cmd.Stderr = os.Stderr

	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return string(output), nil
}

func parseSections(report string) map[string]string {
	report = strings.ReplaceAll(report, "\r\n", "\n")
	lines := strings.Split(report, "\n")
	sections := map[string]string{}

	var currentName string
	var current []string

	flush := func() {
		if currentName == "" {
			return
		}
		sections[currentName] = strings.Join(current, "\n")
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			flush()
			currentName = strings.TrimSpace(strings.TrimPrefix(line, "## "))
			current = []string{line}
			continue
		}

		if currentName != "" {
			current = append(current, line)
		}
	}
	flush()

	return sections
}

func shouldReplace(existing, candidate string) bool {
	existingUnknown := strings.Contains(existing, "](Unknown)")
	candidateUnknown := strings.Contains(candidate, "](Unknown)")

	if existingUnknown && !candidateUnknown {
		return true
	}
	if !existingUnknown && candidateUnknown {
		return false
	}

	return len(candidate) > len(existing)
}
