package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yorukot/superfile/src/internal/common"
)

func TestApplyPlainModeConfigUsesAccessibleRendering(t *testing.T) {
	original := common.Config
	t.Cleanup(func() { common.SetConfig(original) })
	common.Config.PlainMode = true
	common.Config.Nerdfont = true
	common.Config.ShowImagePreview = true
	common.Config.ShowSelectIcons = true

	applyPlainModeConfig()

	assert.False(t, common.Config.Nerdfont)
	assert.False(t, common.Config.ShowImagePreview)
	assert.False(t, common.Config.ShowSelectIcons)
	assert.Equal(t, "-", common.Config.BorderTop)
	assert.Equal(t, "|", common.Config.BorderLeft)
	assert.Equal(t, "+", common.Config.BorderTopLeft)
}
