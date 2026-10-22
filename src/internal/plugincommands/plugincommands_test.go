package plugincommands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestHelperProcess(t *testing.T) {
	separator := slices.Index(os.Args, "--")
	if separator < 0 || separator+1 >= len(os.Args) {
		return
	}

	args := os.Args[separator+1:]
	switch args[0] {
	case "args":
		for _, arg := range args[1:] {
			fmt.Println(arg)
		}
	case "environment":
		for _, name := range args[1:] {
			fmt.Printf("%s=%s\n", name, os.Getenv(name))
		}
	case "sleep":
		duration, err := time.ParseDuration(args[1])
		if err != nil {
			os.Exit(2)
		}
		time.Sleep(duration)
	case "output":
		fmt.Print(strings.Repeat("x", 64*1024))
	case "spawn-pipe-holder":
		cmd := exec.Command(os.Args[0], "-test.run=^TestHelperProcess$", "--", "pipe-holder")
		// Keep the intended inherited-pipe lifetime without locking the parent's
		// test-owned working directory during Windows TempDir cleanup.
		cmd.Dir = os.TempDir()
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		fmt.Println(cmd.Process.Pid)
	case "pipe-holder":
		time.Sleep(1500 * time.Millisecond)
	case "exit":
		fmt.Fprint(os.Stderr, "helper failed")
		os.Exit(7)
	default:
		os.Exit(2)
	}
	os.Exit(0)
}

func TestDiscoverListsPluginsDeterministicallyAndDefensively(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "z.json", validManifest("zeta", "Zeta"))
	writeManifest(t, dir, "a.json", validManifest("alpha", "Alpha"))
	if err := os.WriteFile(filepath.Join(dir, "README.txt"), []byte("ignored"), 0o600); err != nil {
		t.Fatal(err)
	}

	registry, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	got := registry.List()
	if ids := []string{got[0].ID, got[1].ID}; !slices.Equal(ids, []string{"alpha", "zeta"}) {
		t.Fatalf("List() IDs = %v", ids)
	}

	got[0].Args[0] = "mutated"
	got[0].Extensions[0] = ".bad"
	got[0].EnvAllowlist[0] = "BAD"
	again := registry.List()[0]
	if again.Args[0] == "mutated" || again.Extensions[0] == ".bad" || again.EnvAllowlist[0] == "BAD" {
		t.Fatal("List() exposed registry-owned slices")
	}
}

func TestExecuteSubstitutesPlaceholdersWithoutShellParsing(t *testing.T) {
	dir := t.TempDir()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	m := validManifest("inspect", "Inspect")
	m.Executable = executable
	m.Extensions = []string{".txt"}
	m.Args = []string{
		"-test.run=^TestHelperProcess$", "--", "args",
		"first={path}", "{paths}", "directory={dir}", "literal=hello && echo injected", "unicode=夜 空",
	}
	writeManifest(t, dir, "inspect.json", m)
	registry := mustDiscover(t, dir)

	parent := filepath.Join(t.TempDir(), "folder with spaces 夜")
	if mkdirErr := os.MkdirAll(parent, 0o700); mkdirErr != nil {
		t.Fatal(mkdirErr)
	}
	paths := []string{filepath.Join(parent, "one file.txt"), filepath.Join(parent, "二.txt")}
	result, err := registry.Execute(
		context.Background(), "inspect", paths, parent,
	)
	if err != nil {
		t.Fatalf("Execute() error = %v; output = %q", err, result.Output)
	}
	want := strings.Join([]string{
		"first=" + paths[0], paths[0], paths[1], "directory=" + parent,
		"literal=hello && echo injected", "unicode=夜 空", "",
	}, "\n")
	if string(result.Output) != want {
		t.Fatalf("output:\n%q\nwant:\n%q", result.Output, want)
	}
	if result.PluginID != "inspect" || result.Executable != executable || result.ExitCode != 0 {
		t.Fatalf("unexpected result metadata: %+v", result)
	}
}

func TestExecuteFiltersEnvironment(t *testing.T) {
	dir := t.TempDir()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	m := validManifest("env", "Environment")
	m.Executable = executable
	m.EnvAllowlist = []string{"PLUGINCOMMANDS_VISIBLE", "GOCOVERDIR"}
	environmentNames := []string{"PLUGINCOMMANDS_VISIBLE", "PLUGINCOMMANDS_HIDDEN"}
	if runtime.GOOS == "windows" {
		environmentNames = append(environmentNames, "PATH", "SystemRoot")
	}
	m.Args = append([]string{"-test.run=^TestHelperProcess$", "--", "environment"}, environmentNames...)
	writeManifest(t, dir, "env.json", m)
	t.Setenv("PLUGINCOMMANDS_VISIBLE", "allowed 夜")
	t.Setenv("PLUGINCOMMANDS_HIDDEN", "must-not-leak")

	result, err := mustDiscover(t, dir).Execute(
		context.Background(), "env", []string{absoluteSelection(t, ".txt")}, dir,
	)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	want := "PLUGINCOMMANDS_VISIBLE=allowed 夜\nPLUGINCOMMANDS_HIDDEN=\n"
	if runtime.GOOS == "windows" {
		want += "PATH=" + os.Getenv("PATH") + "\nSystemRoot=" + os.Getenv("SystemRoot") + "\n"
	}
	if got := string(result.Output); got != want {
		t.Fatalf("environment output = %q", got)
	}
}

func TestExecuteRejectsInvalidSelectionAndExtension(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "plugin.json", validManifest("plugin", "Plugin"))
	registry := mustDiscover(t, dir)

	tests := []struct {
		name  string
		paths []string
		cwd   string
	}{
		{name: "empty selection", cwd: dir},
		{name: "relative selection", paths: []string{"relative.txt"}, cwd: dir},
		{
			name: "unclean selection",
			paths: []string{
				dir + string(filepath.Separator) + "child" + string(filepath.Separator) + ".." +
					string(filepath.Separator) + "file.txt",
			},
			cwd: dir,
		},
		{name: "relative cwd", paths: []string{absoluteSelection(t, ".txt")}, cwd: "."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := registry.Execute(
				context.Background(), "plugin", tt.paths, tt.cwd,
			)
			var target *SelectionError
			if !errors.As(err, &target) {
				t.Fatalf("error = %T %v, want *SelectionError", err, err)
			}
		})
	}

	_, err := registry.Execute(
		context.Background(), "plugin", []string{absoluteSelection(t, ".png")}, dir,
	)
	var extensionErr *ExtensionError
	if !errors.As(err, &extensionErr) {
		t.Fatalf("error = %T %v, want *ExtensionError", err, err)
	}
}

func TestExecuteReturnsTypedLifecycleErrorsAndBoundedOutput(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	t.Run("timeout", func(t *testing.T) {
		m := validManifest("slow", "Slow")
		m.Executable = executable
		m.TimeoutMS = 25
		m.Args = []string{"-test.run=^TestHelperProcess$", "--", "sleep", "2s"}
		registry := registryWithManifest(t, m)
		_, err := registry.Execute(
			context.Background(), "slow", []string{absoluteSelection(t, ".txt")}, t.TempDir(),
		)
		var target *TimeoutError
		if !errors.As(err, &target) || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("error = %T %v, want *TimeoutError", err, err)
		}
	})

	t.Run("caller cancellation", func(t *testing.T) {
		m := validManifest("cancel", "Cancel")
		m.Executable = executable
		m.Args = []string{"-test.run=^TestHelperProcess$", "--", "sleep", "2s"}
		registry := registryWithManifest(t, m)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := registry.Execute(
			ctx, "cancel", []string{absoluteSelection(t, ".txt")}, t.TempDir(),
		)
		var target *CanceledError
		if !errors.As(err, &target) || !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %T %v, want canceled error", err, err)
		}
	})

	t.Run("output limit", func(t *testing.T) {
		m := validManifest("noisy", "Noisy")
		m.Executable = executable
		m.MaxOutputBytes = 32
		m.Args = []string{"-test.run=^TestHelperProcess$", "--", "output"}
		registry := registryWithManifest(t, m)
		result, err := registry.Execute(
			context.Background(), "noisy", []string{absoluteSelection(t, ".txt")}, t.TempDir(),
		)
		var target *OutputLimitError
		if !errors.As(err, &target) {
			t.Fatalf("error = %T %v, want *OutputLimitError", err, err)
		}
		if len(result.Output) != 32 {
			t.Fatalf("len(output) = %d, want 32", len(result.Output))
		}
	})

	t.Run("non-zero exit", func(t *testing.T) {
		m := validManifest("failure", "Failure")
		m.Executable = executable
		m.Args = []string{"-test.run=^TestHelperProcess$", "--", "exit"}
		registry := registryWithManifest(t, m)
		result, err := registry.Execute(
			context.Background(), "failure", []string{absoluteSelection(t, ".txt")}, t.TempDir(),
		)
		var target *ExitError
		if !errors.As(err, &target) || result.ExitCode != 7 ||
			!strings.Contains(string(result.Output), "helper failed") {
			t.Fatalf("result = %+v, error = %T %v", result, err, err)
		}
	})

	t.Run("unknown plugin", func(t *testing.T) {
		registry := registryWithManifest(t, validManifest("known", "Known"))
		_, err := registry.Execute(
			context.Background(), "missing", []string{absoluteSelection(t, ".txt")}, t.TempDir(),
		)
		var target *NotFoundError
		if !errors.As(err, &target) {
			t.Fatalf("error = %T %v, want *NotFoundError", err, err)
		}
	})
}

func TestExecuteBoundsWaitForInheritedOutputPipes(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	manifest := validManifest("orphan", "Orphan pipe")
	manifest.Executable = executable
	manifest.TimeoutMS = 100
	manifest.Args = []string{"-test.run=^TestHelperProcess$", "--", "spawn-pipe-holder"}
	registry := registryWithManifest(t, manifest)

	started := time.Now()
	result, err := registry.Execute(
		context.Background(), "orphan", []string{absoluteSelection(t, ".txt")}, t.TempDir(),
	)
	elapsed := time.Since(started)
	if runtime.GOOS == "windows" {
		pid, parseErr := strconv.Atoi(strings.TrimSpace(string(result.Output)))
		if parseErr != nil {
			t.Fatalf("pipe-holder PID missing: %q: %v", result.Output, parseErr)
		}
		process, findErr := os.FindProcess(pid)
		if findErr != nil {
			t.Fatal(findErr)
		}
		// Outside the measured Execute interval: Windows cannot remove the Go
		// test executable while this deliberately orphaned helper still runs.
		t.Cleanup(func() {
			if _, waitErr := process.Wait(); waitErr != nil {
				t.Errorf("wait for pipe-holder cleanup: %v", waitErr)
			}
		})
	}
	var timeoutErr *TimeoutError
	if !errors.As(err, &timeoutErr) {
		t.Fatalf("error = %T %v, want *TimeoutError", err, err)
	}
	if elapsed >= time.Second {
		t.Fatalf("Execute waited %s for a descendant-held pipe", elapsed)
	}
}

func validManifest(id, name string) Manifest {
	return Manifest{
		Version:        ManifestVersion,
		ID:             id,
		Name:           name,
		Executable:     "safe-tool",
		Args:           []string{"{paths}"},
		Extensions:     []string{".txt"},
		TimeoutMS:      2_000,
		MaxOutputBytes: 64 * 1024,
		EnvAllowlist:   []string{"LANG", "GOCOVERDIR"},
	}
}

func writeManifest(t *testing.T, dir, name string, manifest Manifest) {
	t.Helper()
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func mustDiscover(t *testing.T, dir string) *Registry {
	t.Helper()
	registry, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	return registry
}

func registryWithManifest(t *testing.T, manifest Manifest) *Registry {
	t.Helper()
	dir := t.TempDir()
	writeManifest(t, dir, "plugin.json", manifest)
	return mustDiscover(t, dir)
}

func absoluteSelection(t *testing.T, extension string) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "selected"+extension)
}
