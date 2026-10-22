package internal

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/adrg/xdg"

	variable "github.com/yorukot/superfile/src/config"
	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui/filemodel"
	"github.com/yorukot/superfile/src/internal/ui/filepanel"
	"github.com/yorukot/superfile/src/internal/ui/sidebar"
	"github.com/yorukot/superfile/src/internal/ui/sortmodel"
	"github.com/yorukot/superfile/src/pkg/utils"
)

// These benchmarks measure synchronous root Update, including sidebar refresh
// and the ordinary file-panel polling decision. They do not execute returned
// commands, start a Bubble Tea event loop, or measure terminal paint latency.
// Metadata, preview, git refresh and filesystem watching are deliberately off.
// Disk discovery is real and host-dependent, including in the recent/pinned-only
// case: the pre-optimization sidebar eagerly discovers every section.
func BenchmarkExplorerRootUpdate(b *testing.B) {
	benchmarkExplorerRootUpdate(b, false)
}

// This companion includes both visible panels' rendering after root Update.
// It does not include the root View compositor, terminal output, or the footer.
func BenchmarkExplorerRootUpdateAndPanels(b *testing.B) {
	benchmarkExplorerRootUpdate(b, true)
}

// Forced refresh measures the reload work itself, rather than the cached input
// path. Both versions explicitly reread the sidebar and complete file listing.
func BenchmarkExplorerForcedRefresh(b *testing.B) {
	for _, entries := range []int{50, 2000} {
		for _, allSections := range []bool{false, true} {
			b.Run(fmt.Sprintf("files=%d/allSections=%t", entries, allSections), func(b *testing.B) {
				m := explorerUpdateBenchmarkModel(b, entries, allSections)
				b.ReportAllocs()
				for b.Loop() {
					m.sidebarModel.UpdateDirectories()
					m.fileModel.UpdateFilePanelsIfNeeded(true)
				}
				if m.getFocusedFilePanel().ElemCount() != entries {
					b.Fatal("forced refresh changed the fixture file count")
				}
			})
		}
	}
}

func benchmarkExplorerRootUpdate(b *testing.B, renderPanels bool) {
	if os.Getenv("XELIF_REFRESH_CORPUS") == "" {
		root := b.TempDir()
		prepareExplorerUpdateCorpus(b, root)
		b.Setenv("XELIF_REFRESH_CORPUS", root)
	}
	for _, entries := range []int{50, 2000} {
		for _, allSections := range []bool{false, true} {
			sectionName := "recent-pinned"
			if allSections {
				sectionName = "all-sections-host-disks"
			}
			b.Run(fmt.Sprintf("files=%d/%s", entries, sectionName), func(b *testing.B) {
				runExplorerRootUpdateBenchmark(b, entries, allSections, renderPanels)
			})
		}
	}
}

func runExplorerRootUpdateBenchmark(b *testing.B, entries int, allSections, renderPanels bool) {
	m := explorerUpdateBenchmarkModel(b, entries, allSections)
	panel := m.getFocusedFilePanel()
	keys := [...]tea.KeyPressMsg{{Code: tea.KeyDown}, {Code: tea.KeyUp}}
	step := 0
	var panelFrame, sidebarFrame string
	b.ReportAllocs()
	for b.Loop() {
		m.Update(keys[step&1])
		if renderPanels {
			panelFrame = panel.Render(true)
			sidebarFrame = m.sidebarModel.Render(false, panel.Location)
		}
		step++
	}
	if panel.GetCursor() != step&1 || panel.ElemCount() != entries {
		b.Fatal("navigation or file count changed during benchmark")
	}
	if renderPanels && (panelFrame == "" || sidebarFrame == "") {
		b.Fatal("empty panel render")
	}
	runtime.KeepAlive(panelFrame)
	runtime.KeepAlive(sidebarFrame)
}

func TestExplorerRootUpdateFixture(t *testing.T) {
	if os.Getenv("XELIF_REFRESH_CORPUS") == "" {
		root := t.TempDir()
		prepareExplorerUpdateCorpus(t, root)
		t.Setenv("XELIF_REFRESH_CORPUS", root)
	}
	for _, entries := range []int{50, 2000} {
		t.Run(fmt.Sprintf("files=%d", entries), func(t *testing.T) {
			checkExplorerRootUpdateFixture(t, entries)
		})
	}
}

func checkExplorerRootUpdateFixture(t *testing.T, entries int) {
	t.Helper()
	m := explorerUpdateBenchmarkModel(t, entries, false)
	panel := m.getFocusedFilePanel()
	first := panel.GetFocusedItem().Location
	recentBefore := readExplorerUpdateFile(t, variable.RecentFile)
	pinnedBefore := readExplorerUpdateFile(t, variable.PinnedFile)
	sidebarBefore := m.sidebarModel.Render(false, panel.Location)
	if m.sidebarModel.Disabled() || m.sidebarModel.NoActualDir() {
		t.Fatal("benchmark must exercise an enabled populated sidebar")
	}
	if !strings.Contains(sidebarBefore, "side-09") {
		t.Fatal("isolated recent fixture is not visible")
	}
	for step, key := range []tea.KeyPressMsg{{Code: tea.KeyDown}, {Code: tea.KeyUp}} {
		checkExplorerRootUpdateStep(t, m, key, 1-step)
	}
	if panel.GetCursor() != 0 || panel.GetFocusedItem().Location != first ||
		panel.ElemCount() != entries || panel.ValidateCursorAndRenderIndex() != nil {
		t.Fatal("root Update did not preserve valid navigation state")
	}
	if !m.firstLoadingComplete || m.fileModel.FilePreview.IsOpen() || !m.disableMetadata {
		t.Fatal("unexpected loading, preview, or metadata state")
	}
	if got := m.sidebarModel.Render(false, panel.Location); got != sidebarBefore {
		t.Fatal("navigation changed sidebar contents")
	}
	if string(readExplorerUpdateFile(t, variable.RecentFile)) != string(recentBefore) ||
		string(readExplorerUpdateFile(t, variable.PinnedFile)) != string(pinnedBefore) {
		t.Fatal("navigation rewrote isolated recent or pinned state")
	}
}

func checkExplorerRootUpdateStep(t *testing.T, m *model, key tea.KeyPressMsg, wantCursor int) {
	t.Helper()
	next, cmd := m.Update(key)
	if next != m || cmd != nil {
		t.Fatal("fixture must update in place without asynchronous commands")
	}
	if got := m.getFocusedFilePanel().GetCursor(); got != wantCursor {
		t.Fatalf("navigation cursor = %d, want %d", got, wantCursor)
	}
}

// Use the retained corpus so separately compiled baseline and candidate
// binaries can prove identical displayed state on exactly the same paths.
func TestExplorerRootUpdateFingerprint(t *testing.T) {
	if os.Getenv("XELIF_REFRESH_CORPUS") == "" {
		t.Skip("set XELIF_REFRESH_CORPUS to a retained root Update corpus")
	}
	for _, entries := range []int{50, 2000} {
		for _, allSections := range []bool{false, true} {
			m := explorerUpdateBenchmarkModel(t, entries, allSections)
			panel := m.getFocusedFilePanel()
			keys := [...]tea.KeyPressMsg{{Code: tea.KeyDown}, {Code: tea.KeyUp}}
			for step := range 3 {
				if step != 0 {
					m.Update(keys[step-1])
				}
				t.Logf("ROOTFRAME files=%d allSections=%t step=%d panel=%x sidebar=%x", entries, allSections, step,
					sha256.Sum256([]byte(panel.Render(true))),
					sha256.Sum256([]byte(m.sidebarModel.Render(false, panel.Location))))
			}
			m.sidebarModel.UpdateDirectories()
			m.fileModel.UpdateFilePanelsIfNeeded(true)
			t.Logf("ROOTFRAME files=%d allSections=%t step=forced panel=%x sidebar=%x", entries, allSections,
				sha256.Sum256([]byte(panel.Render(true))),
				sha256.Sum256([]byte(m.sidebarModel.Render(false, panel.Location))))
		}
	}
}

// Retain one corpus for paired old/new test binaries. This opt-in test creates
// only a new directory; an existing path is rejected without modifying it.
func TestPrepareExplorerRefreshCorpus(t *testing.T) {
	if os.Getenv("XELIF_PREPARE_REFRESH_CORPUS") != "1" {
		t.Skip("opt-in retained root Update benchmark fixture")
	}
	root := os.Getenv("XELIF_REFRESH_CORPUS")
	if root == "" {
		t.Fatal("XELIF_REFRESH_CORPUS must name a new directory")
	}
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	prepareExplorerUpdateCorpus(t, root)
}

func explorerUpdateBenchmarkModel(tb testing.TB, entries int, allSections bool) *model {
	tb.Helper()
	root := os.Getenv("XELIF_REFRESH_CORPUS")
	if root == "" {
		root = tb.TempDir()
		prepareExplorerUpdateCorpus(tb, root)
	}
	root, err := filepath.Abs(root)
	if err != nil {
		tb.Fatal(err)
	}
	originalConfig := common.Config
	originalRecent, originalPinned := variable.RecentFile, variable.PinnedFile
	originalHome, originalUserDirs := xdg.Home, xdg.UserDirs
	tb.Cleanup(func() {
		common.SetConfig(originalConfig)
		variable.RecentFile = originalRecent //nolint:reassign // Restore isolated fixture globals.
		variable.PinnedFile = originalPinned //nolint:reassign // Restore isolated fixture globals.
		xdg.Home = originalHome              //nolint:reassign // Restore isolated fixture globals.
		xdg.UserDirs = originalUserDirs      //nolint:reassign // Restore isolated fixture globals.
	})
	// Never load the user's recent or pinned state.
	variable.RecentFile = filepath.Join(root, "config", "recent.json") //nolint:reassign // Isolated fixture.
	variable.PinnedFile = filepath.Join(root, "config", "pinned.json") //nolint:reassign // Isolated fixture.
	xdg.Home = filepath.Join(root, "home")                             //nolint:reassign // Isolated fixture.
	xdg.UserDirs = xdg.UserDirectories{                                //nolint:reassign // Isolated fixture.
		Desktop: filepath.Join(root, "home", "Desktop"), Download: filepath.Join(root, "home", "Downloads"),
		Documents: filepath.Join(root, "home", "Documents"), Music: filepath.Join(root, "home", "Music"),
		Pictures: filepath.Join(root, "home", "Pictures"), Videos: filepath.Join(root, "home", "Videos"),
		Templates: filepath.Join(root, "home", "Templates"), PublicShare: filepath.Join(root, "home", "Public"),
	}
	config := originalConfig
	config.SidebarWidth = 20
	config.SidebarSections = []string{utils.SidebarSectionRecent, utils.SidebarSectionPinned}
	if allSections {
		config.SidebarSections = []string{utils.SidebarSectionHome, utils.SidebarSectionRecent,
			utils.SidebarSectionPinned, utils.SidebarSectionDisks}
	}
	config.FilePanelExtraColumns, config.FilePanelNamePercent = 2, 50
	config.Nerdfont, config.ColorFileIcons = true, true
	common.SetConfig(config)
	if !slices.Contains(common.Hotkeys.ListDown, "down") || !slices.Contains(common.Hotkeys.ListUp, "up") {
		tb.Fatal("benchmark requires embedded up/down navigation bindings")
	}
	m := &model{
		focusPanel:      nonePanelFocus,
		disableMetadata: true,
		sidebarModel:    sidebar.New(),
		fileModel: filemodel.Model{
			FilePanels: []filepanel.Model{filepanel.New(filepath.Join(root, fmt.Sprintf("files-%d", entries)),
				true, "", sortmodel.SortByName, false)},
			Width: 120, Height: 40,
		},
	}
	// Bind column renderers only after the panel reaches its final address.
	panel := m.getFocusedFilePanel()
	panel.UpdateDimensions(120, 40)
	panel.UpdateElementsIfNeeded(true, false)
	m.sidebarModel.SetHeight(40)
	if panel.ElemCount() != entries {
		tb.Fatalf("expected %d corpus entries, got %d", entries, panel.ElemCount())
	}
	return m
}

func prepareExplorerUpdateCorpus(tb testing.TB, root string) {
	tb.Helper()
	for _, entries := range []int{50, 2000} {
		folder := filepath.Join(root, fmt.Sprintf("files-%d", entries))
		makeExplorerUpdateDirectory(tb, folder)
		extensions := [...]string{"md", "png", "go", "zip", "mp3", "bin"}
		for index := range entries {
			name := fmt.Sprintf("item-%05d.%s", index, extensions[index%len(extensions)])
			if index%2 != 0 {
				name = fmt.Sprintf("文件-tài-liệu-%05d.%s", index, extensions[index%len(extensions)])
			}
			file, err := os.OpenFile(filepath.Join(folder, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
			if err != nil {
				tb.Fatal(err)
			}
			if err := file.Close(); err != nil {
				tb.Fatal(err)
			}
		}
	}
	makeExplorerUpdateDirectory(tb, filepath.Join(root, "home"))
	for _, name := range []string{"Desktop", "Downloads", "Documents", "Music", "Pictures", "Videos", "Templates", "Public"} {
		makeExplorerUpdateDirectory(tb, filepath.Join(root, "home", name))
	}
	type directoryEntry struct {
		Location string `json:"location"`
		Name     string `json:"name"`
	}
	recent := make([]directoryEntry, 0, 10)
	for index := range 10 {
		name := fmt.Sprintf("side-%02d", index)
		location := filepath.Join(root, name)
		makeExplorerUpdateDirectory(tb, location)
		recent = append(recent, directoryEntry{Location: location, Name: name})
	}
	makeExplorerUpdateDirectory(tb, filepath.Join(root, "config"))
	for name, value := range map[string][]directoryEntry{"recent.json": recent, "pinned.json": recent[:4]} {
		data, err := json.Marshal(value)
		if err != nil {
			tb.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "config", name), data, 0600); err != nil {
			tb.Fatal(err)
		}
	}
}

func makeExplorerUpdateDirectory(tb testing.TB, path string) {
	tb.Helper()
	if err := os.Mkdir(path, 0700); err != nil {
		tb.Fatal(err)
	}
}

func readExplorerUpdateFile(tb testing.TB, path string) []byte {
	tb.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		tb.Fatal(err)
	}
	return data
}
