package prompt

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yorukot/superfile/src/internal/common"
)

func TestModel_getPromptAction(t *testing.T) {
	// Notes of Things we tested
	// About Tokenization failure. Don't test all failures,
	// it will be in tokenize_test.go

	testdata := []struct {
		name           string
		text           string
		shellMode      bool
		expectecAction common.ModelAction
		expectedErr    bool
		expectedErrMsg string
	}{
		{
			name:           "No Action",
			text:           "",
			shellMode:      true,
			expectecAction: common.NoAction{},
			expectedErr:    false,
			expectedErrMsg: "",
		},
		{
			name:      "Shell command",
			text:      "abc xyz /def",
			shellMode: true,
			expectecAction: common.ShellCommandAction{
				Command: "abc xyz /def",
			},
			expectedErr:    false,
			expectedErrMsg: "",
		},
		{
			name:           "Tokenization failure",
			text:           "cd ${sdfdsf", // Missing "}"
			shellMode:      false,
			expectecAction: common.NoAction{},
			expectedErr:    true,
			expectedErrMsg: tokenizationError + " : " + curlyBracketMatchError().Error(),
		},
		{
			name:           "Split with extra arguments",
			text:           SplitCommand + " xyz",
			shellMode:      false,
			expectecAction: common.NoAction{},
			expectedErr:    true,
			expectedErrMsg: splitCommandArgError,
		},
		{
			name:           "cd with 0 arguments",
			text:           CdCommand,
			shellMode:      false,
			expectecAction: common.NoAction{},
			expectedErr:    true,
			expectedErrMsg: "cd command needs exactly one argument, received 0",
		},
		{
			name:           "Invalid command",
			text:           "abcd",
			shellMode:      false,
			expectecAction: common.NoAction{},
			expectedErr:    true,
			expectedErrMsg: "Invalid spf command : abcd",
		},
		{
			name:           "Correct split command",
			text:           SplitCommand,
			shellMode:      false,
			expectecAction: common.SplitPanelAction{},
			expectedErr:    false,
			expectedErrMsg: "",
		},
		{
			name:           "Correct cd command",
			text:           CdCommand + " /abc",
			shellMode:      false,
			expectecAction: common.CDCurrentPanelAction{Location: "/abc"},
			expectedErr:    false,
			expectedErrMsg: "",
		},
		{
			name:           "Correct open command",
			text:           OpenCommand + " /abc",
			shellMode:      false,
			expectecAction: common.OpenPanelAction{Location: "/abc"},
			expectedErr:    false,
			expectedErrMsg: "",
		},
		{
			name:           "open with three arguments",
			text:           OpenCommand + " /abc /xyz",
			shellMode:      false,
			expectecAction: common.NoAction{},
			expectedErr:    true,
			expectedErrMsg: "open command needs exactly one argument, received 2",
		},
		{
			name:           "save named workspace",
			text:           `workspace save "Project One"`,
			shellMode:      false,
			expectecAction: common.WorkspaceAction{Operation: common.WorkspaceSave, Name: "Project One"},
		},
		{
			name:           "list workspaces",
			text:           "workspace list",
			shellMode:      false,
			expectecAction: common.WorkspaceAction{Operation: common.WorkspaceList},
		},
		{
			name:           "workspace operation missing",
			text:           "workspace",
			shellMode:      false,
			expectecAction: common.NoAction{},
			expectedErr:    true,
			expectedErrMsg: "workspace needs an operation: save, load, delete, or list",
		},
		{
			name:      "save named search",
			text:      `search save Reports "type:file ext:pdf"`,
			shellMode: false,
			expectecAction: common.SearchAction{
				Operation: common.SearchSave, Name: "Reports", Query: "type:file ext:pdf",
			},
		},
		{
			name:           "run named search",
			text:           "search run Reports",
			shellMode:      false,
			expectecAction: common.SearchAction{Operation: common.SearchRun, Name: "Reports"},
		},
		{
			name:           "preview batch replacement",
			text:           `batch-rename replace " draft" ""`,
			shellMode:      false,
			expectecAction: common.BatchRenameAction{Operation: common.BatchRenameReplace, Find: " draft"},
		},
		{
			name:           "apply batch rename",
			text:           "batch-rename apply",
			shellMode:      false,
			expectecAction: common.BatchRenameAction{Operation: common.BatchRenameApply},
		},
		{
			name:           "add tag",
			text:           `tag add "Needs review"`,
			shellMode:      false,
			expectecAction: common.CatalogAction{Operation: common.TagAdd, Name: "Needs review"},
		},
		{
			name:           "save collection",
			text:           `collection save "Project files"`,
			shellMode:      false,
			expectecAction: common.CatalogAction{Operation: common.CollectionSave, Name: "Project files"},
		},
		{
			name:           "preview modified time",
			text:           "metadata modified 2026-07-31T12:00:00Z",
			shellMode:      false,
			expectecAction: common.MetadataAction{Operation: common.MetadataModified, Value: "2026-07-31T12:00:00Z"},
		},
	}

	for _, tt := range testdata {
		t.Run(tt.name, func(t *testing.T) {
			action, err := getPromptAction(tt.shellMode, tt.text, "/")
			if err != nil {
				assert.True(t, tt.expectedErr)
				//nolint: errorlint // We don't expect a wrapped error here, so using type assertion
				cmdErr, ok := err.(invalidCmdError)
				assert.True(t, ok)
				if tt.expectedErrMsg != "" {
					assert.Equal(t, tt.expectedErrMsg, cmdErr.uiMessage())
				}
			}

			assert.Equal(t, tt.expectecAction, action)
		})
	}
}

func Test_getFirstToken(t *testing.T) {
	t.Run("Basic test", func(t *testing.T) {
		assert.Equal(t, "abc", getFirstToken("abc"))
		assert.Equal(t, "abc", getFirstToken("abc a b c"))
		assert.Equal(t, "abc", getFirstToken("abc "))
		assert.Equal(t, "abc", getFirstToken("  abc "))
		assert.Equal(t, "abc\n\ta", getFirstToken("abc\n\ta"))
	})
}
