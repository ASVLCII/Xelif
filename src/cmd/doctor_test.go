package cmd

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/profile"
)

func TestDoctorDoesNotCreateProfileOrGuessFont(t *testing.T) {
	root := t.TempDir()
	p := profile.Paths{
		ConfigDir:  filepath.Join(root, "config"),
		DataDir:    filepath.Join(root, "data"),
		StateDir:   filepath.Join(root, "state"),
		ConfigFile: filepath.Join(root, "config", "config.toml"),
	}
	report := collectDoctor(p, func(name string) string {
		if name == "WT_SESSION" {
			return "not-a-secret-to-report"
		}
		return ""
	}, func(string) (string, error) { return "", os.ErrNotExist })
	require.Equal(t, "windows-terminal", report.Terminal.Program)
	require.Equal(t, "unknown", report.Terminal.Font)
	require.Equal(t, "unknown", report.Settings["nerdfont"].State)
	entries, err := os.ReadDir(root)
	require.NoError(t, err)
	require.Empty(t, entries)
	data, err := json.Marshal(report)
	require.NoError(t, err)
	require.NotContains(t, string(data), "not-a-secret-to-report")
}

func TestDoctorCommandDispatchAvoidsTUIInitialization(t *testing.T) {
	app := newApp(embed.FS{})
	var output bytes.Buffer
	app.Writer = &output
	require.NoError(t, app.Run(context.Background(), []string{"xelif", "doctor", "--json"}))
	var report doctorReport
	require.NoError(t, json.Unmarshal(output.Bytes(), &report))
	require.Equal(t, 1, report.SchemaVersion)
	require.NotEmpty(t, report.Executable)
	require.NotEmpty(t, report.SearchProviders)
}
