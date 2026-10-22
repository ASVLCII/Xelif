package i18n

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLanguageSelectionAndFallback(t *testing.T) {
	t.Cleanup(func() { _ = SetLanguage("en") })
	require.NoError(t, SetLanguage("zh-TW"))
	assert.Equal(t, TraditionalChinese, Current())
	assert.Equal(t, "全域搜尋", T("search_everywhere"))
	assert.Equal(t, "unknown_key", T("unknown_key"))

	err := SetLanguage("made-up")
	require.ErrorIs(t, err, ErrUnsupportedLanguage)
	assert.Equal(t, English, Current())
}

func TestAutoLanguageUsesTraditionalChineseLocale(t *testing.T) {
	t.Setenv("LC_ALL", "zh_TW.UTF-8")
	t.Cleanup(func() { _ = SetLanguage("en") })
	require.NoError(t, SetLanguage("auto"))
	assert.Equal(t, TraditionalChinese, Current())
	assert.Contains(t, Supported(), English)
}
