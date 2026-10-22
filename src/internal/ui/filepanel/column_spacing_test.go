package filepanel

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui/sortmodel"
)

func TestMetadataColumnsAlignHeadersAndValues(t *testing.T) {
	old := common.Config
	t.Cleanup(func() { common.SetConfig(old) })
	common.Config.PlainMode = false
	root := t.TempDir()
	path := filepath.Join(root, "empty.txt")
	require.NoError(t, os.WriteFile(path, nil, 0600))
	info, err := os.Stat(path)
	require.NoError(t, err)
	dirInfo, err := os.Stat(root)
	require.NoError(t, err)
	m := testModel(0, 0, 12, BrowserMode, []Element{
		{Name: "empty.txt", Location: path, Info: info},
		{Name: "folder", Location: root, Directory: true, Info: dirInfo},
	})
	m.width = 100
	columns := m.makeColumns(2, 50)
	require.Len(t, columns, 5)
	require.Equal(t, strings.Repeat(" ", FileSizeColumnWidth-len("Size"))+"Size", ansi.Strip(columns[2].RenderHeader()))
	require.True(t, strings.HasPrefix(ansi.Strip(columns[4].RenderHeader()), "Modified"))
	size := ansi.Strip(m.renderFileSize(0, FileSizeColumnWidth))
	require.True(t, strings.HasSuffix(size, "0 B"), "real empty files must remain distinct from unknown sizes")
	require.Equal(t, "—", strings.TrimSpace(ansi.Strip(m.renderFileSize(1, FileSizeColumnWidth))))
	modified := ansi.Strip(m.renderModifyTime(0, ModifyTimeSizeColumnWidth))
	require.Equal(t, info.ModTime().Format("2006-01-02 15:04"), modified)
	kind, sortable := columnSortKind(columns[4].Name)
	require.True(t, sortable)
	require.Equal(t, sortmodel.SortByDate, kind)
	common.Config.PlainMode = true
	require.Equal(t, "-", strings.TrimSpace(ansi.Strip(m.renderFileSize(1, FileSizeColumnWidth))))
}
