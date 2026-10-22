package plugincommands

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverRejectsCorruptDuplicateAndUnsafeManifests(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Manifest)
	}{
		{name: "unsupported version", mutate: func(m *Manifest) { m.Version = 2 }},
		{name: "invalid id", mutate: func(m *Manifest) { m.ID = "Not Stable" }},
		{name: "blank name", mutate: func(m *Manifest) { m.Name = " \t" }},
		{name: "control character in name", mutate: func(m *Manifest) { m.Name = "Line\nBreak" }},
		{name: "name too long", mutate: func(m *Manifest) { m.Name = strings.Repeat("n", 129) }},
		{name: "executable traversal", mutate: func(m *Manifest) { m.Executable = filepath.Join("..", "tool") }},
		{name: "command string executable", mutate: func(m *Manifest) { m.Executable = "tool --flag" }},
		{name: "control character in executable", mutate: func(m *Manifest) { m.Executable = "tool\x1bname" }},
		{name: "shell executable", mutate: func(m *Manifest) { m.Executable = "powershell.exe" }},
		{name: "missing argument template", mutate: func(m *Manifest) { m.Args = nil }},
		{name: "too many arguments", mutate: func(m *Manifest) { m.Args = make([]string, maxManifestArguments+1) }},
		{name: "argument traversal", mutate: func(m *Manifest) { m.Args = []string{"../outside"} }},
		{name: "unknown placeholder", mutate: func(m *Manifest) { m.Args = []string{"{unknown}"} }},
		{name: "embedded paths placeholder", mutate: func(m *Manifest) { m.Args = []string{"prefix={paths}"} }},
		{name: "bad extension", mutate: func(m *Manifest) { m.Extensions = []string{"txt"} }},
		{name: "duplicate extension", mutate: func(m *Manifest) { m.Extensions = []string{".txt", ".TXT"} }},
		{name: "timeout too small", mutate: func(m *Manifest) { m.TimeoutMS = 0 }},
		{name: "timeout too large", mutate: func(m *Manifest) { m.TimeoutMS = MaxTimeoutMS + 1 }},
		{name: "output too small", mutate: func(m *Manifest) { m.MaxOutputBytes = 0 }},
		{name: "output too large", mutate: func(m *Manifest) { m.MaxOutputBytes = MaxOutputBytes + 1 }},
		{name: "missing environment allowlist", mutate: func(m *Manifest) { m.EnvAllowlist = nil }},
		{name: "invalid environment name", mutate: func(m *Manifest) { m.EnvAllowlist = []string{"BAD=VALUE"} }},
		{name: "credential environment", mutate: func(m *Manifest) { m.EnvAllowlist = []string{"SERVICE_API_TOKEN"} }},
		{name: "duplicate environment", mutate: func(m *Manifest) { m.EnvAllowlist = []string{"LANG", "LANG"} }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			manifest := validManifest("plugin", "Plugin")
			tt.mutate(&manifest)
			writeManifest(t, dir, "plugin.json", manifest)
			_, err := Discover(dir)
			var target *ManifestError
			if !errors.As(err, &target) {
				t.Fatalf("error = %T %v, want *ManifestError", err, err)
			}
		})
	}

	t.Run("corrupt JSON", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "bad.json"), []byte("{"), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := Discover(dir)
		var target *ManifestError
		if !errors.As(err, &target) {
			t.Fatalf("error = %T %v, want *ManifestError", err, err)
		}
	})

	t.Run("unknown JSON field", func(t *testing.T) {
		dir := t.TempDir()
		data := `{
			"version":1,
			"id":"plugin",
			"name":"Plugin",
			"executable":"tool",
			"args":[],
			"timeout_ms":1000,
			"max_output_bytes":1024,
			"env_allowlist":[],
			"command":"unsafe"
		}`
		if err := os.WriteFile(filepath.Join(dir, "bad.json"), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := Discover(dir)
		var target *ManifestError
		if !errors.As(err, &target) {
			t.Fatalf("error = %T %v, want *ManifestError", err, err)
		}
	})

	t.Run("duplicate ID", func(t *testing.T) {
		dir := t.TempDir()
		writeManifest(t, dir, "one.json", validManifest("same", "One"))
		writeManifest(t, dir, "two.json", validManifest("same", "Two"))
		_, err := Discover(dir)
		var target *DuplicateIDError
		if !errors.As(err, &target) {
			t.Fatalf("error = %T %v, want *DuplicateIDError", err, err)
		}
	})
}

func TestDiscoverRejectsUnsafeDirectoryAndOversizedManifest(t *testing.T) {
	_, err := Discover("relative")
	var directoryErr *DiscoveryError
	if !errors.As(err, &directoryErr) {
		t.Fatalf("relative directory error = %T %v", err, err)
	}

	dir := t.TempDir()
	data := []byte(strings.Repeat(" ", MaxManifestBytes+1))
	if writeErr := os.WriteFile(filepath.Join(dir, "large.json"), data, 0o600); writeErr != nil {
		t.Fatal(writeErr)
	}
	_, err = Discover(dir)
	var manifestErr *ManifestError
	if !errors.As(err, &manifestErr) {
		t.Fatalf("oversized manifest error = %T %v", err, err)
	}
}

func TestValidateExecutableRejectsShellHosts(t *testing.T) {
	shellHosts := []string{
		"sh", "bash", "dash", "zsh", "fish", "ksh", "csh", "tcsh",
		"cmd.exe", "PowerShell.EXE", "pwsh", "wscript.exe", "cscript.exe",
	}
	for _, shellHost := range shellHosts {
		t.Run(shellHost, func(t *testing.T) {
			executable := filepath.Join(t.TempDir(), shellHost)
			if err := validateExecutable(executable); err == nil {
				t.Fatalf("validateExecutable(%q) error = nil, want shell-host rejection", executable)
			}
		})
	}
}
