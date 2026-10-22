// Package plugincommands discovers and explicitly executes declarative command
// plugins. It never executes a plugin during discovery and never invokes a shell.
package plugincommands

import "time"

const (
	pathPlaceholder  = "{path}"
	pathsPlaceholder = "{paths}"
	dirPlaceholder   = "{dir}"

	maxManifestArguments = 128

	// ManifestVersion is the only manifest schema version understood by this package.
	ManifestVersion = 1
	// MaxManifestBytes bounds each JSON manifest read during discovery.
	MaxManifestBytes = 64 * 1024
	// MaxTimeoutMS is the largest command timeout accepted from a manifest.
	MaxTimeoutMS = 10 * 60 * 1000
	// MaxOutputBytes is the largest combined stdout/stderr capture accepted.
	MaxOutputBytes = 8 * 1024 * 1024
)

// Manifest is the version 1 declarative command-plugin schema.
//
// {path} expands to the first selected path, {paths} must occupy a complete
// argument and expands to every selected path, and {dir} expands to the parent
// directory of the first selected path.
type Manifest struct {
	Version        int      `json:"version"`
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Executable     string   `json:"executable"`
	Args           []string `json:"args"`
	Extensions     []string `json:"extensions,omitempty"`
	TimeoutMS      int      `json:"timeout_ms"`
	MaxOutputBytes int      `json:"max_output_bytes"`
	EnvAllowlist   []string `json:"env_allowlist"`
}

// Registry is an immutable collection of discovered plugins.
type Registry struct {
	plugins map[string]Manifest
	ordered []string
}

// Result describes one attempted execution. Output is combined stdout and
// stderr in arrival order and never exceeds the manifest's MaxOutputBytes.
type Result struct {
	PluginID   string
	Executable string
	Args       []string
	Output     []byte
	ExitCode   int
	Duration   time.Duration
}

// List returns validated manifests sorted by stable plugin ID. Returned values
// do not share mutable slices with the registry.
func (r *Registry) List() []Manifest {
	if r == nil {
		return nil
	}
	result := make([]Manifest, 0, len(r.ordered))
	for _, id := range r.ordered {
		result = append(result, cloneManifest(r.plugins[id]))
	}
	return result
}

func cloneManifest(manifest Manifest) Manifest {
	manifest.Args = append([]string(nil), manifest.Args...)
	manifest.Extensions = append([]string(nil), manifest.Extensions...)
	manifest.EnvAllowlist = append([]string(nil), manifest.EnvAllowlist...)
	return manifest
}
