package internal

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui/filemodel"
	"github.com/yorukot/superfile/src/internal/ui/filepanel"
	"github.com/yorukot/superfile/src/internal/ui/sortmodel"
)

const navigationBenchmarkEntries = 2000

// Opt-in fixture preparation exclusively creates a new directory and files.
// It never changes or deletes an existing corpus or the application's runtime.
func TestPrepareNavigationSpeedCorpus(t *testing.T) {
	if os.Getenv("XELIF_PREPARE_SPEED_CORPUS") != "1" {
		t.Skip("opt-in retained benchmark fixture")
	}
	root := os.Getenv("XELIF_SPEED_CORPUS")
	if root == "" {
		t.Fatal("XELIF_SPEED_CORPUS must name a new directory")
	}
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	extensions := [...]string{"md", "png", "go", "zip", "mp3", "bin"}
	for index := range navigationBenchmarkEntries {
		name := fmt.Sprintf("item-%05d.%s", index, extensions[index%len(extensions)])
		if index%2 != 0 {
			name = fmt.Sprintf("文件-tài-liệu-%05d.%s", index, extensions[index%len(extensions)])
		}
		file, err := os.OpenFile(filepath.Join(root, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func BenchmarkExplorerNavigationFrame(b *testing.B) {
	m, panel := navigationSpeedModel(b)
	key := common.Hotkeys.ListDown[0]
	var frame string
	b.ReportAllocs()
	for b.Loop() {
		m.mainKey(key)
		frame = panel.Render(true)
	}
	if frame == "" {
		b.Fatal("empty frame")
	}
	runtime.KeepAlive(frame)
}

func TestNavigationSpeedFingerprint(t *testing.T) {
	m, panel := navigationSpeedModel(t)
	for step := range 41 {
		if step == 0 || step == 1 || step == 40 {
			t.Logf("FRAME %d %x", step, sha256.Sum256([]byte(panel.Render(true))))
		}
		m.mainKey(common.Hotkeys.ListDown[0])
	}
}

func navigationSpeedModel(b testing.TB) (*model, *filepanel.Model) {
	b.Helper()
	root := os.Getenv("XELIF_SPEED_CORPUS")
	if root == "" {
		b.Skip("set XELIF_SPEED_CORPUS to a prepared 2000-file corpus")
	}
	original := common.Config
	b.Cleanup(func() { common.SetConfig(original) })
	common.Config.FilePanelExtraColumns = 2
	common.Config.FilePanelNamePercent = 50
	common.Config.Nerdfont, common.Config.ColorFileIcons = true, true
	b.Setenv("NO_COLOR", "")
	if err := os.Unsetenv("NO_COLOR"); err != nil {
		b.Fatal(err)
	}
	m := &model{
		focusPanel: nonePanelFocus,
		fileModel: filemodel.Model{
			FilePanels: []filepanel.Model{filepanel.New(root, true, "", sortmodel.SortByName, false)},
		},
	}
	// Create bound column-renderer methods after the panel reaches its final
	// address, so the renderer and navigation observe the same cursor.
	panel := &m.fileModel.FilePanels[0]
	panel.UpdateDimensions(120, 40)
	panel.UpdateElementsIfNeeded(true, false)
	if panel.ElemCount() != navigationBenchmarkEntries {
		b.Fatalf("expected %d entries, got %d", navigationBenchmarkEntries, panel.ElemCount())
	}
	return m, panel
}
