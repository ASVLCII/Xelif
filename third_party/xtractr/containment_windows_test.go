package xtractr

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Junctions do not need Windows' symlink privilege, and must obey the same
// extraction boundary as Unix symlinks and privileged Windows symlinks.
func TestWindowsExtractionRejectsEscapingJunction(t *testing.T) {
	base := t.TempDir()
	output, outside := filepath.Join(base, "output"), filepath.Join(base, "outside")
	for _, dir := range []string{output, outside} {
		if err := os.Mkdir(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	junction := filepath.Join(output, "redirect")
	if result, err := exec.Command("cmd", "/c", "mklink", "/J", junction, outside).CombinedOutput(); err != nil {
		t.Fatalf("create junction: %v: %s", err, result)
	}
	root, err := acquireOutputRoot(junction, 0755)
	if root != nil {
		_ = root.Close()
	}
	if !errors.Is(err, ErrInvalidPath) {
		t.Fatalf("junction root error %v, want ErrInvalidPath", err)
	}
	victim := filepath.Join(outside, "keep.txt")
	if err := os.WriteFile(victim, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	x := &XFile{OutputDir: output, DirMode: 0755}
	if err := x.writeOutput(filepath.Join(junction, "keep.txt"), []byte("overwritten"), 0644); err == nil {
		t.Fatal("write followed escaping junction")
	}
	content, err := os.ReadFile(victim)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "original" {
		t.Fatalf("outside contents changed: %q", content)
	}
}
