package internal

import "testing"

func waitArchiveWithDiagnostics(t *testing.T, p *TeaProg, signal *testFileOperationSignal, operation string) bool {
	t.Helper()
	if signal.wait(t, operation) {
		return true
	}
	p.WithModel(func(m *model) {
		panel := m.getFocusedFilePanel()
		t.Logf(
			"archive timeout: pending=%d focus=%v fileFocused=%v firstUse=%v entries=%d selected=%d notify=%v title=%q content=%q tasks=%+v",
			p.owner.pending,
			m.focusPanel,
			panel.IsFocused,
			m.firstUse,
			panel.ElemCount(),
			panel.SelectedCount(),
			m.notifyModel.IsOpen(),
			m.notifyModel.GetTitle(),
			m.notifyModel.GetContent(),
			m.processBarModel.TaskSnapshot(),
		)
	})
	return false
}
