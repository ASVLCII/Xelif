package xtractr

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Unpackerr/iso9660"
	"github.com/cavaliergopher/cpio"
)

func TestExtractionRejectsSiblingPrefixEscape(t *testing.T) {
	for _, format := range []string{"zip", "tar"} {
		t.Run(format, func(t *testing.T) {
			base := t.TempDir()
			output := filepath.Join(base, "output")
			sibling := filepath.Join(base, "output-private")
			if err := os.MkdirAll(output, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(sibling, 0755); err != nil {
				t.Fatal(err)
			}
			victim := filepath.Join(sibling, "keep.txt")
			if err := os.WriteFile(victim, []byte("original"), 0644); err != nil {
				t.Fatal(err)
			}
			archive := filepath.Join(base, "attack."+format)
			file, err := os.Create(archive)
			if err != nil {
				t.Fatal(err)
			}
			if format == "zip" {
				writer := zip.NewWriter(file)
				entry, err := writer.Create("../output-private/keep.txt")
				if err != nil {
					t.Fatal(err)
				}
				if _, err := entry.Write([]byte("overwritten")); err != nil {
					t.Fatal(err)
				}
				if err := writer.Close(); err != nil {
					t.Fatal(err)
				}
			} else {
				writer := tar.NewWriter(file)
				if err := writer.WriteHeader(&tar.Header{Name: "../output-private/keep.txt", Mode: 0644, Size: 11}); err != nil {
					t.Fatal(err)
				}
				if _, err := writer.Write([]byte("overwritten")); err != nil {
					t.Fatal(err)
				}
				if err := writer.Close(); err != nil {
					t.Fatal(err)
				}
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
			_, _, _, extractErr := ExtractFile(&XFile{FilePath: archive, OutputDir: output, FileMode: 0644, DirMode: 0755})
			contents, err := os.ReadFile(victim)
			if err != nil {
				t.Fatal(err)
			}
			if string(contents) != "original" {
				t.Errorf("outside file overwritten: %q", contents)
			}
			if extractErr == nil {
				t.Error("extraction accepted sibling-prefix escape")
			}
		})
	}
}

func TestNormalRARPasswordRetries(t *testing.T) {
	size, files, _, err := ExtractFile(&XFile{FilePath: "test_data/archive.rar", OutputDir: t.TempDir(), FileMode: 0644, DirMode: 0755, Password: "testing", Passwords: []string{"testingmore", "some_password", "some_other"}})
	if err != nil {
		t.Fatal(err)
	}
	if size != 20770 || len(files) != 6 {
		t.Fatalf("size %d, files %d", size, len(files))
	}
}

func TestNormalISOExtraction(t *testing.T) {
	base := t.TempDir()
	writer, err := iso9660.NewWriter()
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Cleanup()
	if err := writer.AddFile(strings.NewReader("contents"), "nested/document.txt"); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(base, "document.iso")
	file, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteTo(file, "test"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(base, "output")
	size, files, _, err := ExtractFile(&XFile{FilePath: archive, OutputDir: output, FileMode: 0644, DirMode: 0755})
	if err != nil {
		t.Fatal(err)
	}
	if size != 8 || len(files) != 1 {
		t.Fatalf("size %d, files %d", size, len(files))
	}
	contents, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "contents" {
		t.Fatalf("contents %q", contents)
	}
}

func TestNormalExtractionFormats(t *testing.T) {
	for _, format := range []string{"zip", "tar", "tar.gz", "gz"} {
		t.Run(format, func(t *testing.T) {
			base := t.TempDir()
			archive := filepath.Join(base, "document."+format)
			file, err := os.Create(archive)
			if err != nil {
				t.Fatal(err)
			}
			var writer io.Writer = file
			var compressed *gzip.Writer
			if format == "gz" || format == "tar.gz" {
				compressed = gzip.NewWriter(file)
				writer = compressed
			}
			switch format {
			case "zip":
				z := zip.NewWriter(writer)
				if _, err := z.Create("nested/"); err != nil {
					t.Fatal(err)
				}
				entry, err := z.Create("nested/document.txt")
				if err != nil {
					t.Fatal(err)
				}
				if _, err := io.WriteString(entry, "contents"); err != nil {
					t.Fatal(err)
				}
				if err := z.Close(); err != nil {
					t.Fatal(err)
				}
			case "tar", "tar.gz":
				z := tar.NewWriter(writer)
				if err := z.WriteHeader(&tar.Header{Name: "nested/", Mode: 0755, Typeflag: tar.TypeDir}); err != nil {
					t.Fatal(err)
				}
				if err := z.WriteHeader(&tar.Header{Name: "nested/document.txt", Mode: 0644, Size: 8}); err != nil {
					t.Fatal(err)
				}
				if _, err := io.WriteString(z, "contents"); err != nil {
					t.Fatal(err)
				}
				if err := z.Close(); err != nil {
					t.Fatal(err)
				}
			default:
				if _, err := io.WriteString(writer, "contents"); err != nil {
					t.Fatal(err)
				}
			}
			if compressed != nil {
				if err := compressed.Close(); err != nil {
					t.Fatal(err)
				}
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
			output := filepath.Join(base, "output")
			_, _, _, err = ExtractFile(&XFile{FilePath: archive, OutputDir: output, FileMode: 0644, DirMode: 0755})
			if err != nil {
				t.Fatal(err)
			}
			member := "nested/document.txt"
			if format == "gz" {
				member = "document"
			}
			contents, err := os.ReadFile(filepath.Join(output, filepath.FromSlash(member)))
			if err != nil {
				t.Fatal(err)
			}
			if string(contents) != "contents" {
				t.Fatalf("contents = %q", contents)
			}
		})
	}
}

func TestDirectFormatExtractorCreatesDestination(t *testing.T) {
	for _, format := range []string{"zip", "tar"} {
		t.Run(format, func(t *testing.T) {
			base := t.TempDir()
			archive := filepath.Join(base, "document."+format)
			file, err := os.Create(archive)
			if err != nil {
				t.Fatal(err)
			}
			if format == "zip" {
				writer := zip.NewWriter(file)
				entry, err := writer.Create("document.txt")
				if err != nil {
					t.Fatal(err)
				}
				if _, err := io.WriteString(entry, "contents"); err != nil {
					t.Fatal(err)
				}
				if err := writer.Close(); err != nil {
					t.Fatal(err)
				}
			} else {
				writer := tar.NewWriter(file)
				if err := writer.WriteHeader(&tar.Header{Name: "document.txt", Mode: 0644, Size: 8}); err != nil {
					t.Fatal(err)
				}
				if _, err := io.WriteString(writer, "contents"); err != nil {
					t.Fatal(err)
				}
				if err := writer.Close(); err != nil {
					t.Fatal(err)
				}
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
			output := filepath.Join(base, "missing", "output")
			x := &XFile{FilePath: archive, OutputDir: output, FileMode: 0644, DirMode: 0755}
			if format == "zip" {
				_, _, err = ExtractZIP(x)
			} else {
				_, _, err = ExtractTar(x)
			}
			if err != nil {
				t.Fatal(err)
			}
			contents, err := os.ReadFile(filepath.Join(output, "document.txt"))
			if err != nil {
				t.Fatal(err)
			}
			if string(contents) != "contents" {
				t.Fatalf("contents %q", contents)
			}
		})
	}
}

func TestDirectoryCreationCannotEscape(t *testing.T) {
	base := t.TempDir()
	output := filepath.Join(base, "output")
	if err := os.Mkdir(output, 0755); err != nil {
		t.Fatal(err)
	}
	x := &XFile{OutputDir: output, DirMode: 0755}
	if err := x.mkDir(filepath.Join(base, "outside"), 0755, time.Now()); err == nil {
		t.Fatal("accepted escaping directory")
	}
	if _, err := os.Stat(filepath.Join(base, "outside")); !os.IsNotExist(err) {
		t.Fatalf("outside path created: %v", err)
	}
}

func TestOutputRootRejectsFinalSymlink(t *testing.T) {
	base := t.TempDir()
	outside := filepath.Join(base, "outside")
	if err := os.Mkdir(outside, 0755); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(base, "redirect")
	if err := os.Symlink(outside, destination); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	root, err := acquireOutputRoot(destination, 0755)
	if root != nil {
		_ = root.Close()
	}
	if !errors.Is(err, ErrInvalidPath) {
		t.Fatalf("error %v, want ErrInvalidPath", err)
	}
}

func TestOutputRootRejectsIdentityMismatch(t *testing.T) {
	initialPath := t.TempDir()
	initial, err := os.Lstat(initialPath)
	if err != nil {
		t.Fatal(err)
	}
	replacement, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer replacement.Close()
	if err := verifyOutputRoot(replacement, initial); !errors.Is(err, ErrInvalidPath) {
		t.Fatalf("error %v, want ErrInvalidPath", err)
	}
	unchanged, err := os.OpenRoot(initialPath)
	if err != nil {
		t.Fatal(err)
	}
	defer unchanged.Close()
	if err := verifyOutputRoot(unchanged, initial); err != nil {
		t.Fatalf("unchanged root rejected: %v", err)
	}
}

func TestWriteRejectsEscapingSymlink(t *testing.T) {
	base := t.TempDir()
	output, outside := filepath.Join(base, "output"), filepath.Join(base, "outside")
	for _, dir := range []string{output, outside} {
		if err := os.Mkdir(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(outside, filepath.Join(output, "redirect")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	x := &XFile{OutputDir: output, DirMode: 0755}
	if err := x.writeOutput(filepath.Join(output, "redirect", "new.txt"), []byte("escape"), 0644); err == nil {
		t.Fatal("followed escaping symlink")
	}
	if _, err := os.Stat(filepath.Join(outside, "new.txt")); !os.IsNotExist(err) {
		t.Fatalf("outside file created: %v", err)
	}
}

func TestCPIORejectsExternalLinkBeforeCreation(t *testing.T) {
	base := t.TempDir()
	archive := filepath.Join(base, "attack.cpio")
	file, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	writer := cpio.NewWriter(file)
	if err := writer.WriteHeader(&cpio.Header{Name: "redirect", Mode: cpio.TypeSymlink | 0777, Linkname: "../../outside"}); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(base, "output")
	_, _, _, err = ExtractFile(&XFile{FilePath: archive, OutputDir: output, FileMode: 0644, DirMode: 0755})
	if err == nil {
		t.Fatal("external link accepted")
	}
	if _, err := os.Lstat(filepath.Join(output, "redirect")); !os.IsNotExist(err) {
		t.Fatalf("unsafe link created: %v", err)
	}
}
