package internal

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/common"
)

func TestCommandHotkeysRetainNavigationAndActionBindings(t *testing.T) {
	m := &model{}
	for _, tc := range []struct {
		id   string
		want []string
	}{
		{commandIDParent, common.Hotkeys.ParentDirectory},
		{commandIDNewTab, common.Hotkeys.NewFileTab},
		{commandIDHistoryForward, common.Hotkeys.HistoryForward},
		{commandIDToggleSelection, common.Hotkeys.ChangePanelMode},
		{commandIDSelectDirectories, common.Hotkeys.FilePanelSelectDirectories},
		{commandIDSort, common.Hotkeys.OpenSortOptionsMenu},
		{commandIDPaste, common.Hotkeys.PasteItems},
		{commandIDUndo, common.Hotkeys.UndoLastFileOperation},
		{commandIDMetadataPrompt, common.Hotkeys.OpenSPFPrompt},
		{commandIDFocusMetadata, common.Hotkeys.FocusOnMetaData},
		{"unknown", nil},
	} {
		require.Equal(t, tc.want, m.commandHotkeys(tc.id), tc.id)
	}
}
