package archivebrowser

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSupported(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"things.zip", "THINGS.ZIP", "folder/name.Zip"} {
		if !Supported(name) {
			t.Errorf("Supported(%q) = false, want true", name)
		}
	}
	for _, name := range []string{"things.tar", "zip", "things.zip.txt", ""} {
		if Supported(name) {
			t.Errorf("Supported(%q) = true, want false", name)
		}
	}
}

func TestOpenErrorsPreserveCause(t *testing.T) {
	t.Parallel()

	if _, err := Open(filepath.Join(t.TempDir(), "archive.tar")); !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("Open unsupported error = %v, want ErrUnsupportedFormat", err)
	}
	_, err := Open(filepath.Join(t.TempDir(), "missing.zip"))
	if !errors.Is(err, ErrInvalidArchive) || !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Open missing error = %v, want ErrInvalidArchive and fs.ErrNotExist", err)
	}
}

func TestOpenBuildsNormalizedTreeWithImplicitDirectories(t *testing.T) {
	t.Parallel()

	archivePath := writeZIP(t, []zipMember{
		{name: "root.txt", body: "root"},
		{name: "docs//./nested/read me.txt", body: "hello"},
		{name: `docs\日本語.txt`, body: "unicode"},
		{name: "empty/", dir: true},
	})
	archive := openArchive(t, archivePath)

	root, err := archive.List("")
	if err != nil {
		t.Fatalf("List root: %v", err)
	}
	if got, want := entryPaths(root), []string{"docs", "empty", "root.txt"}; !slices.Equal(got, want) {
		t.Fatalf("root paths = %q, want %q", got, want)
	}
	if !root[0].IsDir || !root[0].Implicit {
		t.Fatalf("docs metadata = %#v, want implicit directory", root[0])
	}
	if !root[1].IsDir || root[1].Implicit {
		t.Fatalf("empty metadata = %#v, want explicit directory", root[1])
	}

	docs, err := archive.List("docs/")
	if err != nil {
		t.Fatalf("List docs: %v", err)
	}
	if got, want := entryPaths(docs), []string{"docs/nested", "docs/日本語.txt"}; !slices.Equal(got, want) {
		t.Fatalf("docs paths = %q, want %q", got, want)
	}

	entry, err := archive.Stat(`docs\nested\read me.txt`)
	if err != nil {
		t.Fatalf("Stat normalized path: %v", err)
	}
	if entry.Name != "read me.txt" || entry.Path != "docs/nested/read me.txt" || entry.IsDir || entry.Size != 5 {
		t.Fatalf("entry = %#v", entry)
	}
}

func TestListAndStatErrorsAreTyped(t *testing.T) {
	t.Parallel()

	archive := openArchive(t, writeZIP(t, []zipMember{{name: "file.txt", body: "x"}}))
	if _, err := archive.List("file.txt"); !errors.Is(err, ErrNotDirectory) {
		t.Fatalf("List file error = %v, want ErrNotDirectory", err)
	}
	if _, err := archive.Stat("missing"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Stat missing error = %v, want fs.ErrNotExist", err)
	}
	root, err := archive.Stat("")
	if err != nil || !root.IsDir || root.Path != "" {
		t.Fatalf("Stat root = %#v, %v", root, err)
	}
	if _, err := archive.List("../outside"); !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("List unsafe path error = %v, want ErrUnsafePath", err)
	}
}

func TestOpenRejectsDuplicateNormalizedPaths(t *testing.T) {
	t.Parallel()

	archivePath := writeZIP(t, []zipMember{
		{name: "same//file.txt", body: "one"},
		{name: "same/file.txt", body: "two"},
	})
	_, err := Open(archivePath)
	if !errors.Is(err, ErrDuplicatePath) {
		t.Fatalf("Open duplicate error = %v, want ErrDuplicatePath", err)
	}
	var opErr *Error
	if !errors.As(err, &opErr) || opErr.Op != "index" || opErr.Member != "same/file.txt" {
		t.Fatalf("Open duplicate error = %#v, want actionable index error", err)
	}
}

func TestOpenRejectsFileDirectoryCollision(t *testing.T) {
	t.Parallel()

	archivePath := writeZIP(t, []zipMember{
		{name: "collision", body: "file"},
		{name: "collision/child.txt", body: "child"},
	})
	if _, err := Open(archivePath); !errors.Is(err, ErrDuplicatePath) {
		t.Fatalf("Open collision error = %v, want ErrDuplicatePath", err)
	}
}

func TestOpenRejectsTraversalAndAbsolutePaths(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"../escape.txt", "safe/../../escape.txt", "/absolute.txt", `C:\escape.txt`} {
		t.Run(strings.ReplaceAll(name, "/", "_"), func(t *testing.T) {
			t.Parallel()
			_, err := Open(writeZIP(t, []zipMember{{name: name, body: "bad"}}))
			if !errors.Is(err, ErrUnsafePath) {
				t.Fatalf("Open %q error = %v, want ErrUnsafePath", name, err)
			}
		})
	}
}

func TestReadEnforcesLimitAndCancellation(t *testing.T) {
	t.Parallel()

	archive := openArchive(t, writeZIP(t, []zipMember{
		{name: "large.txt", body: "123456"},
		{name: "empty.txt", body: ""},
	}))

	data, err := archive.Read(context.Background(), "large.txt", 6)
	if err != nil || string(data) != "123456" {
		t.Fatalf("Read exact limit = %q, %v", data, err)
	}
	if _, err := archive.Read(context.Background(), "large.txt", 5); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("Read over limit error = %v, want ErrLimitExceeded", err)
	}
	if data, err := archive.Read(context.Background(), "empty.txt", 0); err != nil || len(data) != 0 {
		t.Fatalf("Read empty at zero limit = %q, %v", data, err)
	}
	if _, err := archive.Read(context.Background(), "large.txt", -1); !errors.Is(err, ErrInvalidLimit) {
		t.Fatalf("Read negative limit error = %v, want ErrInvalidLimit", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := archive.Read(ctx, "large.txt", 10); !errors.Is(err, context.Canceled) {
		t.Fatalf("Read canceled error = %v, want context.Canceled", err)
	}
}

func TestReadCancellationDuringMember(t *testing.T) {
	t.Parallel()

	archive := openArchive(t, writeZIP(t, []zipMember{
		{name: "large.bin", body: strings.Repeat("x", 64*1024), store: true},
	}))
	ctx := &cancelAfterContext{after: 2}
	if _, err := archive.Read(ctx, "large.bin", 64*1024); !errors.Is(err, context.Canceled) {
		t.Fatalf("Read mid-stream cancellation error = %v, want context.Canceled", err)
	}
}

func TestReadDetectsCorruptMember(t *testing.T) {
	t.Parallel()

	archivePath := writeZIP(t, []zipMember{{name: "file.txt", body: "unique-payload", store: true}})
	data, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("read archive for corruption: %v", err)
	}
	offset := bytes.Index(data, []byte("unique-payload"))
	if offset < 0 {
		t.Fatal("stored member payload not found")
	}
	data[offset] ^= 0xff
	if err := os.WriteFile(archivePath, data, 0o600); err != nil {
		t.Fatalf("corrupt archive member: %v", err)
	}
	archive := openArchive(t, archivePath)
	if _, err := archive.Read(context.Background(), "file.txt", 64); !errors.Is(err, ErrInvalidArchive) {
		t.Fatalf("Read corrupt member error = %v, want ErrInvalidArchive", err)
	}
}

func TestReadRejectsDirectory(t *testing.T) {
	t.Parallel()

	archive := openArchive(t, writeZIP(t, []zipMember{{name: "dir/file", body: "x"}}))
	if _, err := archive.Read(context.Background(), "dir", 10); !errors.Is(err, ErrIsDirectory) {
		t.Fatalf("Read directory error = %v, want ErrIsDirectory", err)
	}
	// This call intentionally verifies that the API rejects a nil context.
	_, err := archive.Read(nil, "dir/file", 10) //nolint:staticcheck // Intentionally verifies nil-context rejection.
	if !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("Read nil context error = %v, want ErrInvalidArgument", err)
	}
	if _, missingErr := archive.Read(context.Background(), "missing", 10); !errors.Is(missingErr, fs.ErrNotExist) {
		t.Fatalf("Read missing error = %v, want fs.ErrNotExist", missingErr)
	}
}

func TestExtractSelectedDirectoryRecursively(t *testing.T) {
	t.Parallel()

	archive := openArchive(t, writeZIP(t, []zipMember{
		{name: "chosen/read me.txt", body: "hello"},
		{name: "chosen/日本語.txt", body: "世界"},
		{name: "ignored.txt", body: "no"},
	}))
	destination := t.TempDir()
	if err := archive.Extract(context.Background(), []string{"chosen"}, destination); err != nil {
		t.Fatalf("Extract: %v", err)
	}
	assertFileContent(t, filepath.Join(destination, "chosen", "read me.txt"), "hello")
	assertFileContent(t, filepath.Join(destination, "chosen", "日本語.txt"), "世界")
	if _, err := os.Stat(filepath.Join(destination, "ignored.txt")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("ignored file Stat error = %v, want fs.ErrNotExist", err)
	}
}

func TestExtractDoesNotOverwriteAndRollsBackNewFiles(t *testing.T) {
	t.Parallel()

	archive := openArchive(t, writeZIP(t, []zipMember{
		{name: "a-new.txt", body: "new"},
		{name: "z-existing.txt", body: "archive"},
	}))
	destination := t.TempDir()
	existing := filepath.Join(destination, "z-existing.txt")
	if err := os.WriteFile(existing, []byte("keep"), 0o600); err != nil {
		t.Fatalf("seed existing file: %v", err)
	}

	err := archive.Extract(context.Background(), []string{"a-new.txt", "z-existing.txt"}, destination)
	if !errors.Is(err, fs.ErrExist) {
		t.Fatalf("Extract error = %v, want fs.ErrExist", err)
	}
	assertFileContent(t, existing, "keep")
	if _, err := os.Stat(filepath.Join(destination, "a-new.txt")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("new file remains after failed extraction: %v", err)
	}
}

func TestExtractCanExplicitlyOverwriteRegularFile(t *testing.T) {
	t.Parallel()

	archive := openArchive(t, writeZIP(t, []zipMember{{name: "file.txt", body: "replacement"}}))
	destination := t.TempDir()
	target := filepath.Join(destination, "file.txt")
	if err := os.WriteFile(target, []byte("old"), 0o600); err != nil {
		t.Fatalf("seed existing file: %v", err)
	}
	if err := archive.Extract(
		context.Background(),
		[]string{"file.txt"},
		destination,
		ExtractOptions{Overwrite: true},
	); err != nil {
		t.Fatalf("Extract overwrite: %v", err)
	}
	assertFileContent(t, target, "replacement")
}

func TestExtractRejectsArchiveSymlink(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"link", "link-with-slash/"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			archive := openArchive(t, writeZIP(t, []zipMember{
				{name: name, body: "target", mode: fs.ModeSymlink | 0o777},
			}))
			err := archive.Extract(context.Background(), []string{name}, t.TempDir())
			if !errors.Is(err, ErrUnsafeType) {
				t.Fatalf("Extract symlink error = %v, want ErrUnsafeType", err)
			}
		})
	}
}

func TestExtractRejectsDestinationSymlink(t *testing.T) {
	archive := openArchive(t, writeZIP(t, []zipMember{{name: "linked/escape.txt", body: "bad"}}))
	destination := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(destination, "linked")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	err := archive.Extract(context.Background(), []string{"linked/escape.txt"}, destination)
	if !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("Extract through symlink error = %v, want ErrUnsafePath", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "escape.txt")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("outside file Stat error = %v, want fs.ErrNotExist", err)
	}
}

func TestExtractHonorsCancellation(t *testing.T) {
	t.Parallel()

	archive := openArchive(t, writeZIP(t, []zipMember{{name: "file.txt", body: "content"}}))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	destination := t.TempDir()
	if err := archive.Extract(ctx, []string{"file.txt"}, destination); !errors.Is(err, context.Canceled) {
		t.Fatalf("Extract canceled error = %v, want context.Canceled", err)
	}
	if _, err := os.Stat(filepath.Join(destination, "file.txt")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("file exists after canceled extraction: %v", err)
	}
}

func TestExtractValidatesArguments(t *testing.T) {
	t.Parallel()

	archive := openArchive(t, writeZIP(t, []zipMember{{name: "file.txt", body: "content"}}))
	destination := t.TempDir()
	// This call intentionally verifies that the API rejects a nil context.
	err := archive.Extract(
		nil, //nolint:staticcheck // Deliberately tests nil-context rejection.
		[]string{"file.txt"},
		destination,
	)
	if !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("Extract nil context error = %v, want ErrInvalidArgument", err)
	}
	if optionsErr := archive.Extract(
		context.Background(),
		[]string{"file.txt"},
		destination,
		ExtractOptions{},
		ExtractOptions{},
	); !errors.Is(optionsErr, ErrInvalidArgument) {
		t.Fatalf("Extract multiple options error = %v, want ErrInvalidArgument", optionsErr)
	}
	missingErr := archive.Extract(context.Background(), []string{"missing"}, destination)
	if !errors.Is(missingErr, fs.ErrNotExist) {
		t.Fatalf("Extract missing member error = %v, want fs.ErrNotExist", missingErr)
	}
	unsafeErr := archive.Extract(context.Background(), []string{"../outside"}, destination)
	if !errors.Is(unsafeErr, ErrUnsafePath) {
		t.Fatalf("Extract unsafe member error = %v, want ErrUnsafePath", unsafeErr)
	}
	if destinationErr := archive.Extract(
		context.Background(),
		[]string{"file.txt"},
		filepath.Join(t.TempDir(), "missing"),
	); !errors.Is(destinationErr, fs.ErrNotExist) {
		t.Fatalf("Extract missing destination error = %v, want fs.ErrNotExist", destinationErr)
	}
	destinationFile := filepath.Join(t.TempDir(), "destination.txt")
	if writeErr := os.WriteFile(destinationFile, nil, 0o600); writeErr != nil {
		t.Fatalf("create destination file: %v", writeErr)
	}
	if destinationErr := archive.Extract(
		context.Background(),
		[]string{"file.txt"},
		destinationFile,
	); !errors.Is(destinationErr, ErrNotDirectory) {
		t.Fatalf("Extract file destination error = %v, want ErrNotDirectory", destinationErr)
	}
}

func TestOpenCorruptArchive(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "corrupt.zip")
	if err := os.WriteFile(path, []byte("not a zip"), 0o600); err != nil {
		t.Fatalf("write corrupt archive: %v", err)
	}
	_, err := Open(path)
	if !errors.Is(err, ErrInvalidArchive) {
		t.Fatalf("Open corrupt error = %v, want ErrInvalidArchive", err)
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	t.Parallel()

	archivePath := writeZIP(t, []zipMember{{name: "file", body: "x"}})
	archive, err := Open(archivePath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if _, err := archive.List(""); !errors.Is(err, ErrClosed) {
		t.Fatalf("List after close error = %v, want ErrClosed", err)
	}
}

type zipMember struct {
	name  string
	body  string
	dir   bool
	mode  fs.FileMode
	store bool
}

func writeZIP(t *testing.T, members []zipMember) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.zip")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	zw := zip.NewWriter(file)
	for _, member := range members {
		header := &zip.FileHeader{Name: member.name, Method: zip.Deflate}
		if member.dir || member.store {
			header.Method = zip.Store
		}
		if member.dir {
			header.SetMode(fs.ModeDir | 0o755)
		} else if member.mode != 0 {
			header.SetMode(member.mode)
		}
		writer, err := zw.CreateHeader(header)
		if err != nil {
			t.Fatalf("create ZIP member %q: %v", member.name, err)
		}
		if !strings.HasSuffix(member.name, "/") {
			if _, err := writer.Write([]byte(member.body)); err != nil {
				t.Fatalf("write ZIP member %q: %v", member.name, err)
			}
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close ZIP writer: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close archive: %v", err)
	}
	return path
}

type cancelAfterContext struct {
	calls atomic.Int32
	after int32
}

func (c *cancelAfterContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (c *cancelAfterContext) Done() <-chan struct{}       { return nil }
func (c *cancelAfterContext) Value(any) any               { return nil }

func (c *cancelAfterContext) Err() error {
	if c.calls.Add(1) > c.after {
		return context.Canceled
	}
	return nil
}

func openArchive(t *testing.T, path string) *Archive {
	t.Helper()
	archive, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := archive.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	return archive
}

func entryPaths(entries []Entry) []string {
	paths := make([]string, len(entries))
	for i, entry := range entries {
		paths[i] = entry.Path
	}
	return paths
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %q: %v", path, err)
	}
	if got := string(data); got != want {
		t.Fatalf("content of %q = %q, want %q", path, got, want)
	}
}
