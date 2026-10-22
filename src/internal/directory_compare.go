package internal

import (
	"context"
	"errors"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/yorukot/superfile/src/internal/dircompare"
)

const minDirectoryComparePanels = 2

type pendingDirectorySync struct {
	source      string
	destination string
	plan        dircompare.CopyResult
}

type directoryCompareMsg struct {
	reqID         int
	left          string
	right         string
	result        dircompare.Result
	copyPlan      *dircompare.CopyResult
	verifyContent bool
	err           error
}

type directorySyncMsg struct {
	reqID  int
	result dircompare.CopyResult
	err    error
}

func (m *model) comparisonPanels() (string, string, error) {
	if m.fileModel.PanelCount() < minDirectoryComparePanels {
		return "", "", errors.New("open at least two file panels")
	}
	left := m.fileModel.FocusedPanelIndex
	right := (left + 1) % m.fileModel.PanelCount()
	return m.fileModel.FilePanels[left].Location, m.fileModel.FilePanels[right].Location, nil
}

func (m *model) startDirectoryCompare(verifyContent bool) tea.Cmd {
	left, right, err := m.comparisonPanels()
	if err != nil {
		m.showUserError("Could not compare folders", err)
		return nil
	}
	ctx, reqID := m.newDirectoryCompareContext()
	m.textViewer.Open("Directory comparison", "Comparing the focused panel with the next panel in the background…")
	m.setTextViewerSize()
	return func() tea.Msg {
		result, compareErr := dircompare.Compare(ctx, left, right, dircompare.Options{VerifyContent: verifyContent})
		return directoryCompareMsg{
			reqID: reqID, left: left, right: right, result: result,
			verifyContent: verifyContent, err: compareErr,
		}
	}
}

func (m *model) previewDirectorySync() tea.Cmd {
	source, destination, err := m.comparisonPanels()
	if err != nil {
		m.showUserError("Could not preview folder sync", err)
		return nil
	}
	ctx, reqID := m.newDirectoryCompareContext()
	m.textViewer.Open("Copy-missing preview", "Planning a safe one-way copy from the focused panel to the next panel…")
	m.setTextViewerSize()
	return func() tea.Msg {
		plan, planErr := dircompare.CopyMissing(ctx, source, destination, dircompare.CopyOptions{DryRun: true})
		return directoryCompareMsg{
			reqID: reqID, left: source, right: destination, copyPlan: &plan, err: planErr,
		}
	}
}

func (m *model) applyPendingDirectorySync() tea.Cmd {
	if m.pendingSync == nil {
		m.textViewer.Open(
			"Could not copy missing items",
			"No copy-missing preview is pending; preview again before applying.",
		)
		m.setTextViewerSize()
		return nil
	}
	plan := *m.pendingSync
	source, destination, err := m.comparisonPanels()
	if err != nil || source != plan.source || destination != plan.destination {
		m.pendingSync = nil
		m.textViewer.Open("Could not copy missing items", "The compared panels changed; preview again before applying.")
		m.setTextViewerSize()
		return nil
	}
	ctx, reqID := m.newDirectoryCompareContext()
	m.textViewer.Open("Copying missing items", "Copying without overwriting or deleting destination items…")
	m.setTextViewerSize()
	return func() tea.Msg {
		result, err := dircompare.CopyMissing(
			ctx,
			plan.source,
			plan.destination,
			dircompare.CopyOptions{ApprovedPlan: &plan.plan},
		)
		return directorySyncMsg{reqID: reqID, result: result, err: err}
	}
}

func (m *model) newDirectoryCompareContext() (context.Context, int) {
	if m.dirCompareCancel != nil {
		m.dirCompareCancel()
	}
	m.pendingSync = nil
	ctx, cancel := context.WithCancel(context.Background())
	m.dirCompareCancel = cancel
	m.dirCompareReqID++
	return ctx, m.dirCompareReqID
}

func (m *model) cancelDirectoryCompare() {
	if m.dirCompareCancel != nil {
		m.dirCompareCancel()
		m.dirCompareCancel = nil
	}
	m.pendingSync = nil
	// Cancellation also invalidates an already queued result or completion.
	m.dirCompareReqID++
}

func (m *model) applyDirectoryCompare(msg directoryCompareMsg) {
	if msg.reqID != m.dirCompareReqID {
		return
	}
	m.dirCompareCancel = nil
	if msg.err != nil {
		if !errors.Is(msg.err, context.Canceled) {
			m.showTextViewerError("Directory comparison failed", msg.err, "")
		}
		return
	}
	if msg.copyPlan != nil {
		m.pendingSync = &pendingDirectorySync{source: msg.left, destination: msg.right, plan: *msg.copyPlan}
		m.textViewer.Open("Copy-missing preview", renderDirectorySyncPlan(msg.left, msg.right, *msg.copyPlan))
	} else {
		m.textViewer.Open("Directory comparison", renderDirectoryComparison(msg))
	}
	m.setTextViewerSize()
}

func (m *model) applyDirectorySync(msg directorySyncMsg) {
	// Even a superseded/cancelled copy may have published complete entries.
	// Reconcile the visible filesystem without consuming a newer approval.
	if len(msg.result.Created) > 0 {
		m.fileModel.UpdateFilePanelsIfNeeded(true)
	}
	if msg.reqID != m.dirCompareReqID {
		return
	}
	m.dirCompareCancel = nil
	m.pendingSync = nil
	if msg.err != nil {
		if !errors.Is(msg.err, context.Canceled) {
			m.showTextViewerError("Copy-missing operation failed", msg.err, fmt.Sprintf(
				"Created %d item(s); completed entries are retained.", len(msg.result.Created)))
		}
		return
	}
	m.fileModel.UpdateFilePanelsIfNeeded(true)
	m.textViewer.Open("Copy-missing complete",
		fmt.Sprintf("Created %d missing item(s). Existing destination items were not overwritten; nothing was deleted.",
			len(msg.result.Created)))
	m.setTextViewerSize()
}

func renderDirectoryComparison(msg directoryCompareMsg) string {
	counts := make(map[dircompare.Classification]int)
	var differences strings.Builder
	for _, entry := range msg.result.Entries {
		counts[entry.Classification]++
		if entry.Classification != dircompare.Same {
			fmt.Fprintf(&differences, "%s  %s\n", entry.Classification, entry.Path)
		}
	}
	mode := "metadata fast path"
	if msg.verifyContent {
		mode = "SHA-256 content verification"
	}
	return fmt.Sprintf(
		"Left: %s\nRight: %s\nMode: %s\nSame: %d; only left: %d; only right: %d; different: %d; type mismatch: %d\nWarnings: %d\n\n%s",
		msg.left,
		msg.right,
		mode,
		counts[dircompare.Same],
		counts[dircompare.OnlyLeft],
		counts[dircompare.OnlyRight],
		counts[dircompare.Different],
		counts[dircompare.TypeMismatch],
		len(msg.result.Warnings),
		differences.String(),
	)
}

func renderDirectorySyncPlan(source, destination string, result dircompare.CopyResult) string {
	var builder strings.Builder
	fmt.Fprintf(
		&builder,
		"No files have been changed.\nSource (focused): %s\nDestination (next panel): %s\nChanged inputs require a fresh preview before applying.\n\n",
		source,
		destination,
	)
	for _, action := range result.Actions {
		fmt.Fprintf(&builder, "%s  %s\n", action.Kind, action.Path)
	}
	if len(result.Actions) == 0 {
		builder.WriteString("Nothing is missing at the destination.\n")
	} else {
		builder.WriteString("\nUse “Apply previewed copy-missing” in the command palette to confirm.\n")
	}
	return builder.String()
}
