package xtractr

import (
	"archive/zip"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type cancellingReader struct {
	cancel context.CancelFunc
	read   bool
}

func (r *cancellingReader) Read(buffer []byte) (int, error) {
	if r.read {
		return 0, io.EOF
	}
	r.read = true
	r.cancel()
	return copy(buffer, "incomplete data"), nil
}

func TestCancelledExtractionCreatesNoOutput(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	destination := filepath.Join(t.TempDir(), "not-created")
	_, _, _, err := ExtractFile(&XFile{Context: ctx, FilePath: "unused.zip", OutputDir: destination})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("unexpected result: %v", err)
	}
	if _, statErr := os.Lstat(destination); !os.IsNotExist(statErr) {
		t.Fatalf("cancelled request created output: %v", statErr)
	}
}

func TestCancelledWriteRemovesOnlyIncompleteMember(t *testing.T) {
	dir := t.TempDir()
	root, err := acquireOutputRoot(dir, 0o700)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	x := &XFile{Context: ctx, outputRoot: root, OutputDir: dir, FileMode: 0o600, DirMode: 0o700}
	x.newProgress(0, 0, 2)
	complete := filepath.Join(dir, "complete")
	if _, err := x.write(&file{Path: complete, Data: strings.NewReader("retained"), FileMode: 0o600}); err != nil {
		t.Fatal(err)
	}
	incomplete := filepath.Join(dir, "incomplete")
	_, err = x.write(&file{Path: incomplete, Data: &cancellingReader{cancel: cancel}, FileMode: 0o600})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("unexpected result: %v", err)
	}
	data, err := os.ReadFile(complete)
	if err != nil || string(data) != "retained" {
		t.Fatalf("complete member changed: %q %v", data, err)
	}
	if _, err := os.Lstat(incomplete); !os.IsNotExist(err) {
		t.Fatalf("incomplete member retained: %v", err)
	}
}

func TestExtractionPreservesDestinationCollision(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "collision.zip")
	output := filepath.Join(dir, "output")
	if err := os.Mkdir(output, 0o700); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	member, err := writer.Create("keep")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := member.Write([]byte("overwrite")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	victim := filepath.Join(output, "keep")
	if err := os.WriteFile(victim, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := ExtractFile(
		&XFile{FilePath: archive, OutputDir: output, FileMode: 0o600, DirMode: 0o700},
	); err == nil {
		t.Fatal("collision accepted")
	}
	data, err := os.ReadFile(victim)
	if err != nil || string(data) != "original" {
		t.Fatalf("destination changed: %q %v", data, err)
	}
}
