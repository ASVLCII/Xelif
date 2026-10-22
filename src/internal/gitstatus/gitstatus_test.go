package gitstatus

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRootAndStatus(t *testing.T) {
	repo := newTestRepo(t)
	writeFile(t, repo, "clean.txt", "clean\n")
	writeFile(t, repo, "dir/modified file.txt", "before\n")
	writeFile(t, repo, "deleted.txt", "delete me\n")
	writeFile(t, repo, "old name.txt", "rename me\n")
	writeFile(t, repo, ".gitignore", "*.ignored\n")
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-m", "initial")

	writeFile(t, repo, "dir/modified file.txt", "after\n")
	if err := os.Remove(filepath.Join(repo, "deleted.txt")); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "mv", "old name.txt", "renamed 世界.txt")
	writeFile(t, repo, "added.txt", "added\n")
	git(t, repo, "add", "added.txt")
	writeFile(t, repo, "untracked 世界.txt", "untracked\n")
	writeFile(t, repo, "private.ignored", "ignored\n")

	client := New(Options{})
	ctx := context.Background()
	root, err := client.Root(ctx, filepath.Join(repo, "dir", "modified file.txt"))
	if err != nil {
		t.Fatalf("Root: %v", err)
	}
	if root != filepath.Clean(repo) {
		t.Fatalf("Root = %q, want %q", root, filepath.Clean(repo))
	}

	snapshot, err := client.Status(ctx, filepath.Join(repo, "dir"))
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	assertState(t, snapshot, "clean.txt", Clean)
	assertState(t, snapshot, "dir/modified file.txt", Modified)
	assertState(t, snapshot, "deleted.txt", Deleted)
	assertState(t, snapshot, "renamed 世界.txt", Renamed)
	assertState(t, snapshot, "added.txt", Added)
	assertState(t, snapshot, "untracked 世界.txt", Untracked)
	assertState(t, snapshot, "private.ignored", Ignored)
	assertState(t, snapshot, "dir", Modified)

	var rename Entry
	for _, entry := range snapshot.Entries {
		if entry.Path == filepath.FromSlash("renamed 世界.txt") {
			rename = entry
			break
		}
	}
	if rename.OriginalPath != filepath.FromSlash("old name.txt") {
		t.Fatalf("rename original = %q", rename.OriginalPath)
	}
}

func TestConflictState(t *testing.T) {
	repo := newTestRepo(t)
	writeFile(t, repo, "conflict.txt", "base\n")
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-m", "base")
	git(t, repo, "checkout", "-b", "other")
	writeFile(t, repo, "conflict.txt", "other\n")
	git(t, repo, "commit", "-am", "other")
	git(t, repo, "checkout", "master")
	writeFile(t, repo, "conflict.txt", "master\n")
	git(t, repo, "commit", "-am", "master")
	gitAllowFailure(t, repo, "merge", "other")

	snapshot, err := New(Options{}).Status(context.Background(), repo)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	assertState(t, snapshot, "conflict.txt", Conflict)
}

func TestRootFindsRepositoryForMissingDescendant(t *testing.T) {
	repo := newTestRepo(t)
	root, err := New(Options{}).Root(context.Background(), filepath.Join(repo, "not", "created", "yet.txt"))
	if err != nil {
		t.Fatalf("Root: %v", err)
	}
	if root != filepath.Clean(repo) {
		t.Fatalf("Root = %q, want %q", root, filepath.Clean(repo))
	}
}

func TestDiffPreservesLiteralPaths(t *testing.T) {
	repo := newTestRepo(t)
	// This option-like name also contains Git pathspec and shell metacharacters.
	// It must remain one literal argv element throughout the client boundary.
	name := "--literal [abc] & 世界.txt"
	writeFile(t, repo, name, "before\n")
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-m", "initial")
	writeFile(t, repo, name, "unstaged marker\n")

	client := New(Options{})
	diff, err := client.Diff(context.Background(), repo, DiffOptions{Paths: []string{name}})
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if !strings.Contains(string(diff), "+unstaged marker") {
		t.Fatalf("unstaged diff missing marker:\n%s", diff)
	}

	git(t, repo, "add", "--", name)
	diff, err = client.Diff(context.Background(), repo, DiffOptions{Staged: true, Paths: []string{name}})
	if err != nil {
		t.Fatalf("staged Diff: %v", err)
	}
	if !strings.Contains(string(diff), "+unstaged marker") {
		t.Fatalf("staged diff missing marker:\n%s", diff)
	}

	_, err = client.Diff(context.Background(), repo, DiffOptions{Paths: []string{"../outside"}})
	assertErrorKind(t, err, InvalidPath)
}

func TestExplicitActionsArePathScoped(t *testing.T) {
	repo := newTestRepo(t)
	writeFile(t, repo, "tracked.txt", "before\n")
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-m", "initial")
	writeFile(t, repo, "tracked.txt", "after\n")
	writeFile(t, repo, "other.txt", "leave alone\n")

	client := New(Options{})
	ctx := context.Background()
	if err := client.Stage(ctx, repo, "tracked.txt"); err != nil {
		t.Fatalf("Stage: %v", err)
	}
	snapshot, _ := client.Status(ctx, repo)
	assertState(t, snapshot, "tracked.txt", Modified)
	assertState(t, snapshot, "other.txt", Untracked)

	if err := client.Unstage(ctx, repo, "tracked.txt"); err != nil {
		t.Fatalf("Unstage: %v", err)
	}
	if err := client.Discard(ctx, repo, "tracked.txt"); err != nil {
		t.Fatalf("Discard: %v", err)
	}
	contents, err := os.ReadFile(filepath.Join(repo, "tracked.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "before\n" {
		t.Fatalf("discarded contents = %q", contents)
	}
	if _, err := os.Stat(filepath.Join(repo, "other.txt")); err != nil {
		t.Fatalf("unrelated file changed: %v", err)
	}

	assertErrorKind(t, client.Stage(ctx, repo), InvalidPath)
	assertErrorKind(t, client.Unstage(ctx, repo, "."), InvalidPath)
	assertErrorKind(t, client.Discard(ctx, repo, "../outside"), InvalidPath)
}

func TestUnstageInRepositoryWithoutCommits(t *testing.T) {
	repo := newTestRepo(t)
	writeFile(t, repo, "new file 世界.txt", "contents\n")
	client := New(Options{})
	if err := client.Stage(context.Background(), repo, "new file 世界.txt"); err != nil {
		t.Fatalf("Stage: %v", err)
	}
	if err := client.Unstage(context.Background(), repo, "new file 世界.txt"); err != nil {
		t.Fatalf("Unstage: %v", err)
	}
	snapshot, err := client.Status(context.Background(), repo)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	assertState(t, snapshot, "new file 世界.txt", Untracked)
}

func TestDiscardDoesNotDeleteUntrackedFile(t *testing.T) {
	repo := newTestRepo(t)
	writeFile(t, repo, "untracked.txt", "preserve me\n")
	err := New(Options{}).Discard(context.Background(), repo, "untracked.txt")
	assertErrorKind(t, err, CommandFailed)
	contents, readErr := os.ReadFile(filepath.Join(repo, "untracked.txt"))
	if readErr != nil || string(contents) != "preserve me\n" {
		t.Fatalf("untracked file was changed: contents=%q err=%v", contents, readErr)
	}
}

func TestErrorsAreTypedAndBounded(t *testing.T) {
	client := New(Options{Timeout: time.Second, MaxOutput: 32})
	_, err := client.Root(context.Background(), t.TempDir())
	assertErrorKind(t, err, NotRepository)

	client.gitBinary = filepath.Join(t.TempDir(), "missing-git")
	_, err = client.Root(context.Background(), t.TempDir())
	assertErrorKind(t, err, GitUnavailable)

	repo := newTestRepo(t)
	writeFile(t, repo, "large-output-file.txt", "untracked\n")
	boundedClient := New(Options{MaxOutput: 16})
	_, err = boundedClient.Status(context.Background(), repo)
	assertErrorKind(t, err, OutputLimit)
	// Also exercise status directly: a long temporary root can itself exceed
	// this budget, so Status alone would only prove the root-command limit.
	_, err = boundedClient.run(context.Background(), "read status", repo, repo, true,
		"status", "--porcelain=v2", "-z", "--untracked-files=all")
	assertErrorKind(t, err, OutputLimit)

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = New(Options{}).Status(cancelled, repo)
	assertErrorKind(t, err, TimedOut)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error does not unwrap context cancellation: %v", err)
	}
	message := err.Error()
	if !strings.Contains(message, "repository root") ||
		!strings.Contains(message, string(TimedOut)) {
		t.Fatalf("error is not actionable: %q", message)
	}
}

func TestDirectoryStateUsesStablePriority(t *testing.T) {
	states := []State{Clean, Ignored, Untracked, Modified, Added, Renamed, Deleted, Conflict}
	for index, want := range states {
		entries := make([]Entry, 0, index+1)
		for child, state := range states[:index+1] {
			entries = append(entries, Entry{Path: filepath.Join("dir", string(rune('a'+child))), State: state})
		}
		snapshot := Snapshot{Root: t.TempDir(), Entries: entries}
		got, err := snapshot.StateFor("dir")
		if err != nil {
			t.Fatalf("StateFor: %v", err)
		}
		if got != want {
			t.Fatalf("with states %v, got %q, want %q", states[:index+1], got, want)
		}
	}
}

func TestParsePorcelainRejectsMalformedRecords(t *testing.T) {
	cases := [][]byte{
		[]byte("1 bad\x00"),
		[]byte("2 bad\x00"),
		[]byte("u bad\x00"),
		[]byte("?bad\x00"),
		[]byte("!bad\x00"),
		[]byte("x unknown\x00"),
	}
	for _, input := range cases {
		if _, err := parsePorcelainV2(input); err == nil {
			t.Fatalf("parsePorcelainV2(%q) unexpectedly succeeded", input)
		}
	}
}

func TestLimitedBufferDrainsAfterReachingLimit(t *testing.T) {
	buffer := &limitedBuffer{max: 4}
	written, err := buffer.Write([]byte("12345678"))
	if err != nil {
		t.Fatalf("Write returned an error that could stop pipe draining: %v", err)
	}
	if written != 8 {
		t.Fatalf("Write reported %d bytes, want 8", written)
	}
	if string(buffer.data) != "1234" || !buffer.exceeded {
		t.Fatalf("buffer = %q, exceeded = %v", buffer.data, buffer.exceeded)
	}
	written, err = buffer.Write([]byte("more"))
	if err != nil || written != 4 || string(buffer.data) != "1234" {
		t.Fatalf("post-limit Write = (%d, %v), buffer = %q", written, err, buffer.data)
	}
}

func assertState(t *testing.T, snapshot Snapshot, path string, want State) {
	t.Helper()
	got, err := snapshot.StateFor(path)
	if err != nil {
		t.Fatalf("StateFor(%q): %v", path, err)
	}
	if got != want {
		t.Fatalf("StateFor(%q) = %q, want %q; entries: %#v", path, got, want, snapshot.Entries)
	}
}

func assertErrorKind(t *testing.T, err error, want ErrorKind) {
	t.Helper()
	var gitErr *Error
	if !errors.As(err, &gitErr) {
		t.Fatalf("error %v is not *Error", err)
	}
	if gitErr.Kind != want {
		t.Fatalf("error kind = %q, want %q: %v", gitErr.Kind, want, err)
	}
}

func newTestRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}
	// Git reports a physical root, not a symlink alias such as macOS /var.
	repo, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	git(t, repo, "init", "--initial-branch=master")
	git(t, repo, "config", "user.email", "gitstatus@example.test")
	git(t, repo, "config", "user.name", "Git Status Tests")
	git(t, repo, "config", "core.autocrlf", "false")
	return repo
}

func writeFile(t *testing.T, root, name, contents string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func gitAllowFailure(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	if err := cmd.Run(); err == nil {
		t.Fatalf("git %v unexpectedly succeeded", args)
	}
}
