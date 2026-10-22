package internal

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/yorukot/superfile/src/internal/gitstatus"
)

const gitStatusRefreshInterval = 2 * time.Second

type gitStatusTarget struct {
	location string
	paths    []string
}

type gitStatusUpdateMsg struct {
	reqID  int
	states map[string]map[string]gitstatus.State
	errors []error
}

func (m *model) refreshGitStatusCmd(force bool) tea.Cmd {
	if m.gitClient == nil {
		return nil
	}
	targets, locations := m.currentGitStatusTargets()
	if !force && slices.Equal(locations, m.gitStatusPaths) &&
		time.Since(m.gitStatusLast) < gitStatusRefreshInterval {
		return nil
	}
	m.gitStatusPaths = slices.Clone(locations)
	m.gitStatusLast = time.Now()
	m.gitStatusReqID++
	reqID := m.gitStatusReqID
	client := m.gitClient
	return func() tea.Msg {
		return readGitStatusTargets(client, targets, reqID)
	}
}

func readGitStatusTargets(client *gitstatus.Client, targets []gitStatusTarget, reqID int) gitStatusUpdateMsg {
	states := make(map[string]map[string]gitstatus.State, len(targets))
	var operationErrors []error
	for _, target := range targets {
		pathStates := make(map[string]gitstatus.State, len(target.paths))
		states[target.location] = pathStates
		snapshot, err := client.Status(context.Background(), target.location)
		if err != nil {
			var gitErr *gitstatus.Error
			if !errors.As(err, &gitErr) || gitErr.Kind != gitstatus.NotRepository {
				operationErrors = append(operationErrors, err)
			}
			continue
		}
		for _, path := range target.paths {
			state, stateErr := snapshot.StateFor(path)
			if stateErr != nil {
				operationErrors = append(operationErrors, stateErr)
				continue
			}
			if state != gitstatus.Clean {
				pathStates[path] = state
			}
		}
	}
	return gitStatusUpdateMsg{reqID: reqID, states: states, errors: operationErrors}
}

func (m *model) currentGitStatusTargets() ([]gitStatusTarget, []string) {
	byLocation := make(map[string]*gitStatusTarget, len(m.fileModel.FilePanels))
	for panelIndex := range m.fileModel.FilePanels {
		panel := &m.fileModel.FilePanels[panelIndex]
		key := folderWatchPathKey(panel.Location)
		target, exists := byLocation[key]
		if !exists {
			target = &gitStatusTarget{location: panel.Location}
			byLocation[key] = target
		}
		for elementIndex := range panel.ElemCount() {
			target.paths = append(target.paths, panel.GetElementAtIdx(elementIndex).Location)
		}
	}
	locations := make([]string, 0, len(byLocation))
	for _, target := range byLocation {
		slices.Sort(target.paths)
		locations = append(locations, target.location)
	}
	slices.Sort(locations)
	targets := make([]gitStatusTarget, 0, len(locations))
	for _, location := range locations {
		targets = append(targets, *byLocation[folderWatchPathKey(location)])
	}
	return targets, locations
}

func (m *model) applyGitStatusUpdate(msg gitStatusUpdateMsg) {
	if msg.reqID != m.gitStatusReqID {
		return
	}
	for _, err := range msg.errors {
		slog.Warn("Could not refresh Git status", "error", err)
	}
	for panelIndex := range m.fileModel.FilePanels {
		panel := &m.fileModel.FilePanels[panelIndex]
		states := msg.states[panel.Location]
		if states == nil {
			states = map[string]gitstatus.State{}
		}
		panel.SetGitStates(states)
	}
}
