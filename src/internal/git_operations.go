package internal

import (
	"context"
	"errors"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/yorukot/superfile/src/internal/gitstatus"
	"github.com/yorukot/superfile/src/internal/ui/filepanel"
	"github.com/yorukot/superfile/src/internal/ui/notify"
)

type gitMutation uint8

const (
	gitMutationStage gitMutation = iota + 1
	gitMutationUnstage
	gitMutationDiscard
)

type gitOperationMsg struct {
	title   string
	content string
	err     error
	mutated bool
}

func (m *model) gitOperationPaths() []string {
	panel := m.getFocusedFilePanel()
	if panel.PanelMode == filepanel.SelectMode && panel.SelectedCount() > 0 {
		return panel.GetSelectedLocationsSortedAsVisible()
	}
	if panel.Empty() {
		return nil
	}
	return []string{panel.GetFocusedItem().Location}
}

func (m *model) gitDiffCmd(staged bool, paths []string) tea.Cmd {
	if m.gitClient == nil {
		return func() tea.Msg { return gitOperationMsg{err: errors.New("git integration is unavailable")} }
	}
	location := m.getFocusedFilePanel().Location
	client := m.gitClient
	paths = append([]string(nil), paths...)
	return func() tea.Msg {
		content, err := client.Diff(context.Background(), location, gitstatus.DiffOptions{Staged: staged, Paths: paths})
		if err != nil {
			return gitOperationMsg{err: err}
		}
		title := "Git diff"
		if staged {
			title = "Git staged diff"
		}
		if len(content) == 0 {
			content = []byte("No matching changes.")
		}
		return gitOperationMsg{title: title, content: string(content)}
	}
}

func (m *model) gitMutationCmd(mutation gitMutation, paths []string) tea.Cmd {
	if len(paths) == 0 {
		return nil
	}
	if m.gitClient == nil {
		return func() tea.Msg { return gitOperationMsg{err: errors.New("git integration is unavailable")} }
	}
	location := m.getFocusedFilePanel().Location
	client := m.gitClient
	paths = append([]string(nil), paths...)
	return func() tea.Msg {
		var action string
		var err error
		switch mutation {
		case gitMutationStage:
			action = "Staged"
			err = client.Stage(context.Background(), location, paths...)
		case gitMutationUnstage:
			action = "Unstaged"
			err = client.Unstage(context.Background(), location, paths...)
		case gitMutationDiscard:
			action = "Discarded worktree changes for"
			err = client.Discard(context.Background(), location, paths...)
		default:
			err = errors.New("unknown Git mutation")
		}
		if err != nil {
			return gitOperationMsg{err: err}
		}
		return gitOperationMsg{
			title:   "Git",
			content: fmt.Sprintf("%s %d item(s):\n%s", action, len(paths), strings.Join(paths, "\n")),
			mutated: true,
		}
	}
}

func (m *model) confirmGitDiscard(paths []string) tea.Cmd {
	if len(paths) == 0 {
		return nil
	}
	m.pendingGitDiscard = append([]string(nil), paths...)
	count := len(paths)
	reqID := m.nextIoReqCnt()
	return func() tea.Msg {
		return NewNotifyModalMsg(notify.New(true, "Discard Git changes?",
			fmt.Sprintf("Restore %d tracked item(s) from the index? Untracked files are never removed.", count),
			notify.GitDiscardAction), reqID)
	}
}

func (m *model) applyGitOperation(msg gitOperationMsg) tea.Cmd {
	if msg.err != nil {
		m.showUserError("Git operation failed", msg.err)
		return nil
	}
	if msg.content != "" {
		m.textViewer.Open(msg.title, msg.content)
		m.setTextViewerSize()
	}
	if msg.mutated {
		return m.refreshGitStatusCmd(true)
	}
	return nil
}
