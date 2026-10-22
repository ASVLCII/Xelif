package plugincommands

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const outputDrainDelay = 100 * time.Millisecond

// Execute validates the selection, substitutes the documented placeholders,
// then invokes the configured executable directly with exec.CommandContext.
func (r *Registry) Execute(ctx context.Context, pluginID string, selectedPaths []string, cwd string) (Result, error) {
	manifest, exists := r.plugins[pluginID]
	if !exists {
		return Result{PluginID: pluginID, ExitCode: -1}, &NotFoundError{PluginID: pluginID}
	}
	if err := validateSelection(selectedPaths, cwd); err != nil {
		return Result{PluginID: pluginID, ExitCode: -1}, err
	}
	for _, path := range selectedPaths {
		if !acceptsExtension(manifest.Extensions, path) {
			return Result{PluginID: pluginID, ExitCode: -1}, &ExtensionError{PluginID: pluginID, Path: path}
		}
	}

	args := expandArguments(manifest.Args, selectedPaths)
	result := Result{
		PluginID:   pluginID,
		Executable: manifest.Executable,
		Args:       append([]string(nil), args...),
		ExitCode:   -1,
	}
	started := time.Now()
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(manifest.TimeoutMS)*time.Millisecond)
	defer cancel()

	output := newBoundedOutput(manifest.MaxOutputBytes, cancel)
	//nolint:gosec // Registry fields are private and populated by Discover validation; argv execution never invokes a shell.
	cmd := exec.CommandContext(runCtx, manifest.Executable, args...)
	cmd.WaitDelay = outputDrainDelay
	cmd.Dir = cwd
	cmd.Env = minimalEnvironment(manifest.EnvAllowlist)
	cmd.Stdout = output
	cmd.Stderr = output
	err := cmd.Run()
	result.Duration = time.Since(started)
	result.Output = output.Bytes()
	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}

	if output.Exceeded() {
		return result, &OutputLimitError{PluginID: pluginID, Limit: manifest.MaxOutputBytes}
	}
	if err == nil {
		return result, nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return result, &CanceledError{PluginID: pluginID, Err: ctxErr}
	}
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		return result, &TimeoutError{PluginID: pluginID, Timeout: time.Duration(manifest.TimeoutMS) * time.Millisecond}
	}
	if errors.Is(err, exec.ErrWaitDelay) {
		return result, &TimeoutError{PluginID: pluginID, Timeout: time.Duration(manifest.TimeoutMS) * time.Millisecond}
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return result, &ExitError{PluginID: pluginID, ExitCode: result.ExitCode, Err: err}
	}
	return result, &StartError{PluginID: pluginID, Err: err}
}

func validateSelection(paths []string, cwd string) error {
	if len(paths) == 0 {
		return &SelectionError{Reason: "at least one selected path is required"}
	}
	if !filepath.IsAbs(cwd) || filepath.Clean(cwd) != cwd {
		return &SelectionError{Reason: "working directory must be an absolute, clean path"}
	}
	for _, path := range paths {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path {
			return &SelectionError{Reason: "every selected path must be absolute and clean"}
		}
	}
	return nil
}

func acceptsExtension(extensions []string, path string) bool {
	if len(extensions) == 0 {
		return true
	}
	lowerPath := strings.ToLower(path)
	for _, extension := range extensions {
		if strings.HasSuffix(lowerPath, extension) {
			return true
		}
	}
	return false
}

func expandArguments(template, paths []string) []string {
	args := make([]string, 0, len(template)+len(paths))
	first := paths[0]
	dir := filepath.Dir(first)
	for _, arg := range template {
		if arg == pathsPlaceholder {
			args = append(args, paths...)
			continue
		}
		arg = strings.ReplaceAll(arg, pathPlaceholder, first)
		arg = strings.ReplaceAll(arg, dirPlaceholder, dir)
		args = append(args, arg)
	}
	return args
}

func minimalEnvironment(allowlist []string) []string {
	names := append([]string(nil), allowlist...)
	if runtime.GOOS == "windows" {
		names = append(names, "PATH", "SystemRoot")
	}
	seen := make(map[string]struct{}, len(names))
	environment := make([]string, 0, len(names))
	for _, name := range names {
		canonical := name
		if runtime.GOOS == "windows" {
			canonical = strings.ToUpper(name)
		}
		if _, exists := seen[canonical]; exists {
			continue
		}
		seen[canonical] = struct{}{}
		if value, exists := os.LookupEnv(name); exists {
			environment = append(environment, name+"="+value)
		}
	}
	return environment
}

type boundedOutput struct {
	mu       sync.Mutex
	buffer   bytes.Buffer
	limit    int
	exceeded bool
	cancel   context.CancelFunc
}

func newBoundedOutput(limit int, cancel context.CancelFunc) *boundedOutput {
	return &boundedOutput{limit: limit, cancel: cancel}
}

func (w *boundedOutput) Write(data []byte) (int, error) {
	w.mu.Lock()
	remaining := w.limit - w.buffer.Len()
	if remaining < len(data) {
		if remaining > 0 {
			_, _ = w.buffer.Write(data[:remaining])
		}
		w.exceeded = true
		w.mu.Unlock()
		w.cancel()
		return len(data), nil
	}
	_, _ = w.buffer.Write(data)
	w.mu.Unlock()
	return len(data), nil
}

func (w *boundedOutput) Bytes() []byte {
	w.mu.Lock()
	defer w.mu.Unlock()
	return bytes.Clone(w.buffer.Bytes())
}

func (w *boundedOutput) Exceeded() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.exceeded
}
