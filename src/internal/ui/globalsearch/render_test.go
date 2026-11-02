package globalsearch

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDimensionsClampToMinimums(t *testing.T) {
	t.Parallel()

	m := New(func(context.Context, string, string, int) Response { return Response{} })
	m.SetWidth(1)
	m.SetMaxHeight(1)

	assert.Equal(t, MinWidth, m.GetWidth())
	assert.Equal(t, MinHeight, m.GetMaxHeight())
}

func TestRenderShowsQueryResultsProviderAndExplicitScope(t *testing.T) {
	t.Parallel()

	m := New(func(context.Context, string, string, int) Response { return Response{} })
	m.SetWidth(80)
	m.SetMaxHeight(14)
	m.Open(`C:\Work`)
	m.textInput.SetValue("report")
	m.provider = ProviderEverything
	m.results = []Result{
		{Path: `C:\Docs\report.txt`},
		{Path: `C:\Reports`, Directory: true},
	}
	m.cursor = 1

	output := m.Render()

	assert.Contains(t, output, "Search Everywhere")
	assert.Contains(t, output, ProviderEverything)
	assert.Contains(t, output, "indexed: system index")
	assert.Contains(t, output, "Ctrl+S: folder")
	assert.Contains(t, output, "report")
	assert.Contains(t, output, `C:\Docs\report.txt`)
	assert.Contains(t, output, `C:\Reports`)
}
