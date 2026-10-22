package common

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/pelletier/go-toml/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	variable "github.com/yorukot/superfile/src/config"
)

func TestInstallMissingThemePreservesCustomCopy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "themes", "monochrome.toml")
	require.NoError(t, installMissingTheme(path, "bundled"))
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "bundled", string(data))
	require.NoError(t, os.WriteFile(path, []byte("customized"), 0o600))
	require.NoError(t, installMissingTheme(path, "new bundled version"))
	data, err = os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "customized", string(data))
}

func TestBundledDefaultsUseCustomizableMonochrome(t *testing.T) {
	configData, err := os.ReadFile(filepath.Join("..", "..", "superfile_config", "config.toml"))
	require.NoError(t, err)
	var config ConfigType
	require.NoError(t, toml.Unmarshal(configData, &config))
	assert.Equal(t, "monochrome", config.Theme)
	themeData, err := os.ReadFile(filepath.Join("..", "..", "superfile_config", "theme", "monochrome.toml"))
	require.NoError(t, err)
	var theme ThemeType
	require.NoError(t, toml.Unmarshal(themeData, &theme))
	values := reflect.ValueOf(theme)
	for index := range values.NumField() {
		field := values.Field(index)
		if field.Kind() != reflect.String || values.Type().Field(index).Name == "CodeSyntaxHighlightTheme" {
			continue
		}
		color := field.String()
		require.Len(t, color, 7, values.Type().Field(index).Name)
		assert.Equal(t, color[1:3], color[3:5], values.Type().Field(index).Name)
		assert.Equal(t, color[3:5], color[5:7], values.Type().Field(index).Name)
	}
}

func TestBundledThemeInstallationDoesNotOverwriteCustomizations(t *testing.T) {
	original := variable.ThemeFolder
	variable.ThemeFolder = t.TempDir() //nolint:reassign // Isolate the theme installation fixture.
	t.Cleanup(func() {
		variable.ThemeFolder = original //nolint:reassign // Restore the process-wide fixture.
	})
	customPath := filepath.Join(variable.ThemeFolder, "monochrome.toml")
	require.NoError(t, os.WriteFile(customPath, []byte("customized theme"), 0o600))
	content := fstest.MapFS{
		variable.EmbedThemeDir + "/monochrome.toml": {Data: []byte("bundled monochrome")},
		variable.EmbedThemeDir + "/other.toml":      {Data: []byte("other theme")},
	}
	require.NoError(t, WriteThemeFiles(content))
	data, err := os.ReadFile(customPath)
	require.NoError(t, err)
	assert.Equal(t, "customized theme", string(data))
	_, err = os.Stat(filepath.Join(variable.ThemeFolder, "other.toml"))
	require.NoError(t, err, "other bundled themes must remain available")
}
