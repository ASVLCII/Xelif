package common

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pelletier/go-toml/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/pkg/utils"
)

func TestValidateConfigAllowsRecentAndPinnedSidebarSections(t *testing.T) {
	config := validSidebarSectionTestConfig()
	config.SidebarSections = []string{utils.SidebarSectionRecent, utils.SidebarSectionPinned}

	require.NoError(t, ValidateConfig(&config))
}

func TestDefaultSidebarSectionsIncludeRecentAndPinned(t *testing.T) {
	configPath := filepath.Join("..", "..", "superfile_config", "config.toml")
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var config ConfigType
	require.NoError(t, toml.Unmarshal(data, &config))
	assert.Contains(t, config.SidebarSections, utils.SidebarSectionRecent)
	assert.Contains(t, config.SidebarSections, utils.SidebarSectionPinned)
}

func validSidebarSectionTestConfig() ConfigType {
	return ConfigType{
		SidebarWidth:         20,
		DefaultSortType:      0,
		FilePanelNamePercent: 50,
		BorderTop:            "-",
		BorderBottom:         "-",
		BorderLeft:           "|",
		BorderRight:          "|",
		BorderTopLeft:        "+",
		BorderTopRight:       "+",
		BorderBottomLeft:     "+",
		BorderBottomRight:    "+",
		BorderMiddleLeft:     "+",
		BorderMiddleRight:    "+",
	}
}
