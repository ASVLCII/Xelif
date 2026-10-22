package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	variable "github.com/yorukot/superfile/src/config"
	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui/prompt"
	"github.com/yorukot/superfile/src/pkg/utils"
)

const promptCompletionModified = "2021-06-07T08:09:10Z"

func TestPromptBatchRenameAsyncCompletionIsVisible(t *testing.T) {
	for _, closeOnSuccess := range []bool{false, true} {
		for _, failure := range []bool{false, true} {
			t.Run(fmt.Sprintf("close=%t/failure=%t", closeOnSuccess, failure), func(t *testing.T) {
				m, source, _ := newPromptCompletionModel(t, closeOnSuccess)
				require.Nil(t, submitCompletionPrompt(t, m, "batch-rename prefix done-"))
				require.NotNil(t, m.pendingBatchRename)
				cmd := submitCompletionPrompt(t, m, "batch-rename apply")
				require.NotNil(t, cmd)
				assertCompletionSurface(t, m, "Applying batch rename")
				destination := filepath.Join(filepath.Dir(source), "done-"+filepath.Base(source))
				if failure {
					require.NoError(t, os.WriteFile(destination, []byte("external arrival"), 0o600))
				}

				msg, ok := cmd().(batchRenameOperationMsg)
				require.True(t, ok, "typed apply must return the asynchronous rename operation")
				if failure {
					require.Error(t, msg.err)
				} else {
					require.NoError(t, msg.err)
				}
				TeaUpdate(m, msg)
				if failure {
					assertCompletionSurface(t, m, "Batch rename failed")
					assertPromptCompletionContents(t, source, "original content")
					assertPromptCompletionContents(t, destination, "external arrival")
				} else {
					assertCompletionSurface(t, m, "Batch rename complete")
					assertPromptCompletionContents(t, destination, "original content")
					require.NoFileExists(t, source)
					assert.NotNil(t, m.lastUndo)
				}
			})
		}
	}
}

func TestPromptMetadataApplyAndRestoreAsyncCompletionsAreVisible(t *testing.T) {
	for _, closeOnSuccess := range []bool{false, true} {
		t.Run(fmt.Sprintf("close=%t", closeOnSuccess), func(t *testing.T) {
			journalDirectory := isolatePromptMetadataJournals(t)
			m, source, originalModified := newPromptCompletionModel(t, closeOnSuccess)
			require.Nil(t, submitCompletionPrompt(t, m, "metadata modified "+promptCompletionModified))
			require.NotNil(t, m.pendingMetadataPatch)
			modified, err := time.Parse(time.RFC3339, promptCompletionModified)
			require.NoError(t, err)
			for _, step := range []struct {
				command, progress, complete string
				wantModified                time.Time
			}{
				{"metadata apply", "Applying metadata", "Metadata updated", modified},
				{"metadata restore", "Restoring metadata", "Metadata restored", originalModified},
			} {
				cmd := submitCompletionPrompt(t, m, step.command)
				require.NotNil(t, cmd)
				assertCompletionSurface(t, m, step.progress)
				msg, ok := cmd().(metadataOperationMsg)
				require.True(t, ok, "typed metadata command must return its asynchronous operation")
				require.NoError(t, msg.err)
				assert.Equal(t, journalDirectory, filepath.Dir(msg.journal))
				TeaUpdate(m, msg)
				assertCompletionSurface(t, m, step.complete)
				info, err := os.Stat(source)
				require.NoError(t, err)
				assert.True(
					t,
					step.wantModified.Equal(info.ModTime()),
					"modification time must match the completed edit",
				)
				if step.command == "metadata apply" {
					require.FileExists(t, m.lastMetadataJournal)
				} else {
					assert.Empty(t, m.lastMetadataJournal)
				}
			}
		})
	}
}

func TestPromptMetadataAsyncFailuresAreVisible(t *testing.T) {
	for _, closeOnSuccess := range []bool{false, true} {
		for _, restore := range []bool{false, true} {
			t.Run(fmt.Sprintf("close=%t/restore=%t", closeOnSuccess, restore), func(t *testing.T) {
				journalDirectory := isolatePromptMetadataJournals(t)
				m, source, wantModified := newPromptCompletionModel(t, closeOnSuccess)
				require.Nil(t, submitCompletionPrompt(t, m, "metadata modified "+promptCompletionModified))
				command, progress := "metadata apply", "Applying metadata"
				if restore {
					apply := submitCompletionPrompt(t, m, "metadata apply")
					require.NotNil(t, apply)
					msg, ok := apply().(metadataOperationMsg)
					require.True(t, ok)
					require.NoError(t, msg.err)
					TeaUpdate(m, msg)
					command, progress = "metadata restore", "Restoring metadata"
					var err error
					wantModified, err = time.Parse(time.RFC3339, promptCompletionModified)
					require.NoError(t, err)
				} else {
					blocker := filepath.Join(journalDirectory, "not-a-directory")
					require.NoError(t, os.WriteFile(blocker, []byte("keep journal blocker"), 0o600))
					//nolint:reassign // This isolated path makes journal creation fail before metadata mutation.
					variable.MetadataJournalDir = blocker
				}
				cmd := submitCompletionPrompt(t, m, command)
				require.NotNil(t, cmd)
				assertCompletionSurface(t, m, progress)
				if restore {
					require.NoError(t, os.WriteFile(m.lastMetadataJournal, []byte("invalid test journal"), 0o600))
				}

				msg, ok := cmd().(metadataOperationMsg)
				require.True(t, ok)
				require.Error(t, msg.err)
				TeaUpdate(m, msg)
				assertCompletionSurface(t, m, "Metadata operation failed")
				info, err := os.Stat(source)
				require.NoError(t, err)
				assert.True(t, wantModified.Equal(info.ModTime()), "failed work must preserve the previous timestamp")
				assertPromptCompletionContents(t, source, "original content")
			})
		}
	}
}

func newPromptCompletionModel(t *testing.T, closeOnSuccess bool) (*model, string, time.Time) {
	t.Helper()
	root := t.TempDir()
	source := filepath.Join(root, "original.txt")
	require.NoError(t, os.WriteFile(source, []byte("original content"), 0o600))
	originalModified := time.Date(2019, time.January, 2, 3, 4, 5, 0, time.UTC)
	require.NoError(t, os.Chtimes(source, originalModified, originalModified))
	m := defaultTestModel(root)
	// Avoid unrelated subprocess commands when inspecting the typed action's
	// returned command. Default test models already disable preview and metadata.
	m.gitClient = nil
	// Rename completion must not persist path updates into the user's catalog.
	m.catalogModel = nil
	m.promptModal = prompt.GenerateModel(common.Hotkeys.OpenSPFPrompt[0], common.Hotkeys.OpenCommandLine[0],
		closeOnSuccess, DefaultTestModelHeight/2, DefaultTestModelWidth-common.BorderPadding)
	return m, source, originalModified
}

func submitCompletionPrompt(t *testing.T, m *model, command string) tea.Cmd {
	t.Helper()
	if m.textViewer.IsOpen() {
		TeaUpdate(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	}
	require.False(t, m.textViewer.IsOpen())
	TeaUpdate(m, utils.TeaRuneKeyMsg(common.Hotkeys.OpenSPFPrompt[0]))
	require.True(t, m.promptModal.IsOpen())
	TeaUpdate(m, utils.TeaRuneKeyMsg(command))
	return TeaUpdate(m, tea.KeyPressMsg{Code: tea.KeyEnter})
}

func assertCompletionSurface(t *testing.T, m *model, title string) {
	t.Helper()
	assert.False(t, m.promptModal.IsOpen(), "the command prompt must hand off focus")
	assert.True(t, m.textViewer.IsOpen())
	assert.False(t, m.notifyModel.IsOpen(), "completion failures must not create hidden notifications")
	displayed := ansi.Strip(m.updateRenderForOverlay(m.mainComponentsRender()))
	assert.Contains(t, displayed, title)
}

func assertPromptCompletionContents(t *testing.T, path, expected string) {
	t.Helper()
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, expected, string(content))
}

func isolatePromptMetadataJournals(t *testing.T) string {
	t.Helper()
	original := variable.MetadataJournalDir
	//nolint:reassign // Never use the user's real metadata journal directory in regression tests.
	variable.MetadataJournalDir = t.TempDir()
	t.Cleanup(func() {
		//nolint:reassign // Restore repository-global configuration after this sequential test.
		variable.MetadataJournalDir = original
	})
	return variable.MetadataJournalDir
}
