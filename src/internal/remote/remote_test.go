package remote

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

type fakeResponse struct {
	stdout string
	stderr string
	err    error
	run    func(args []string) error
}

type fakeRunner struct {
	responses []fakeResponse
	binaries  []string
	calls     [][]string
}

func (r *fakeRunner) Run(_ context.Context, binary string, args []string, _, _ int64) commandResult {
	r.binaries = append(r.binaries, binary)
	r.calls = append(r.calls, slices.Clone(args))
	response := r.responses[0]
	r.responses = r.responses[1:]
	err := response.err
	if response.run != nil {
		err = response.run(args)
	}
	return commandResult{stdout: []byte(response.stdout), stderr: []byte(response.stderr), err: err}
}

func TestNewDiscoversRcloneWithoutRunningIt(t *testing.T) {
	runner := &fakeRunner{}
	client, err := newClient(func(name string) (string, error) {
		if name != "rclone" {
			t.Fatalf("LookPath name = %q", name)
		}
		return `C:\tools\rclone.exe`, nil
	}, runner, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if client.binary != `C:\tools\rclone.exe` {
		t.Fatalf("binary = %q", client.binary)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("New ran commands: %v", runner.calls)
	}
}

func TestNewReturnsTypedUnavailableError(t *testing.T) {
	_, err := newClient(func(string) (string, error) { return "", errors.New("missing") }, &fakeRunner{}, Options{})
	assertErrorKind(t, err, Unavailable)
}

func TestNewUsesProcessPathAndValidatesOptions(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := New()
	assertErrorKind(t, err, Unavailable)

	for _, options := range []Options{{Timeout: -1}, {MaxOutputBytes: -1}} {
		_, optionsErr := newClient(
			func(string) (string, error) { return "rclone", nil },
			&fakeRunner{},
			options,
		)
		if optionsErr == nil {
			t.Fatalf("newClient(%+v) succeeded", options)
		}
	}
}

func TestParsePathPreservesUnicodeAndSpaces(t *testing.T) {
	got, err := ParsePath("My Drive:資料/Quarter 1/report final.txt")
	if err != nil {
		t.Fatal(err)
	}
	want := RemotePath{Remote: "My Drive", Path: "資料/Quarter 1/report final.txt"}
	if got != want || got.String() != "My Drive:資料/Quarter 1/report final.txt" {
		t.Fatalf("ParsePath = %#v, %q", got, got.String())
	}
}

func TestParsePathRejectsTraversalAndShellSyntax(t *testing.T) {
	for _, input := range []string{
		"", "missing-colon", ":file", "remote:/absolute", "remote:../escape",
		`remote:dir\file`, "remote:file; remove", "remote:file | command", "remote:file\nnext",
	} {
		t.Run(input, func(t *testing.T) {
			_, err := ParsePath(input)
			assertErrorKind(t, err, InvalidPath)
		})
	}
}

func TestRemotesUsesBoundedJSONAndSortsDeterministically(t *testing.T) {
	runner := &fakeRunner{responses: []fakeResponse{{stdout: `[
		{"name":"zeta","type":"s3","source":"file","description":"Z"},
		{"name":"álpha drive","type":"drive","source":"environment","description":"A"}
	]`}}}
	client := testClient(runner)

	got, err := client.Remotes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []Remote{
		{Name: "zeta", Type: "s3", Source: "file", Description: "Z"},
		{Name: "álpha drive", Type: "drive", Source: "environment", Description: "A"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Remotes = %#v", got)
	}
	if runner.binaries[0] != "rclone-test" {
		t.Fatalf("binary = %q", runner.binaries[0])
	}
	assertArgs(t, runner.calls[0], "listremotes", "--json", "--ask-password=false")
}

func TestRemotesRejectsInvalidMetadata(t *testing.T) {
	for _, output := range []string{
		`null`,
		`[{"name":"","type":"drive","source":"file","description":""}]`,
		`[{"name":"cloud","type":"","source":"file","description":""}]`,
		`[{"name":"cloud","type":"drive"},{"name":"cloud","type":"s3"}]`,
	} {
		runner := &fakeRunner{responses: []fakeResponse{{stdout: output}}}
		_, err := testClient(runner).Remotes(context.Background())
		assertErrorKind(t, err, InvalidOutput)
	}
}

func TestListBuildsNormalizedSortedEntriesAndExactArgs(t *testing.T) {
	runner := &fakeRunner{responses: []fakeResponse{
		{stdout: `[{"name":"cloud","type":"drive","source":"file","description":""}]`},
		{stdout: `[
			{"Path":"zeta.txt","Name":"zeta.txt","Size":7,"ModTime":"2026-07-30T12:30:00+07:00","IsDir":false},
			{"Path":"資料","Name":"資料","Size":-1,"ModTime":"2026-07-29T00:00:00Z","IsDir":true},
			{"Path":"Alpha file.txt","Name":"Alpha file.txt","Size":3,"ModTime":"2026-07-30T00:00:00Z","IsDir":false}
		]`},
	}}
	client := testClient(runner)

	got, err := client.List(context.Background(), RemotePath{Remote: "cloud", Path: "folder one"})
	if err != nil {
		t.Fatal(err)
	}
	wantPaths := []string{"folder one/Alpha file.txt", "folder one/zeta.txt", "folder one/資料"}
	if len(got) != len(wantPaths) {
		t.Fatalf("entries = %#v", got)
	}
	for i, want := range wantPaths {
		if got[i].Remote != "cloud" || got[i].Path != want || got[i].Name == "" {
			t.Fatalf("entry %d = %#v", i, got[i])
		}
	}
	if got[2].Size != 0 || !got[2].IsDir {
		t.Fatalf("directory was not normalized: %#v", got[2])
	}
	if got[0].ModTime.Location() != time.UTC {
		t.Fatalf("ModTime location = %v", got[0].ModTime.Location())
	}
	assertArgs(t, runner.calls[1], "lsjson", "cloud:folder one", "--no-mimetype", "--ask-password=false")
}

func TestListRejectsUnconfiguredRemoteWithoutNetworkCommand(t *testing.T) {
	runner := &fakeRunner{responses: []fakeResponse{{stdout: `[]`}}}
	client := testClient(runner)

	_, err := client.List(context.Background(), RemotePath{Remote: "missing"})
	assertErrorKind(t, err, NotConfigured)
	if len(runner.calls) != 1 {
		t.Fatalf("calls = %v", runner.calls)
	}
}

func TestListClassifiesAuthAndInvalidOutput(t *testing.T) {
	for _, tc := range []struct {
		name     string
		response fakeResponse
		kind     ErrorKind
	}{
		{
			name: "auth",
			response: fakeResponse{
				stderr: "HTTP error 401: Unauthorized",
				err:    errors.New("exit 1"),
			},
			kind: Authentication,
		},
		{name: "invalid JSON", response: fakeResponse{stdout: `[{`}, kind: InvalidOutput},
		{
			name: "unsafe path",
			response: fakeResponse{
				stdout: `[{"Path":"../escape","Name":"escape","Size":1,"ModTime":"2026-01-01T00:00:00Z","IsDir":false}]`,
			},
			kind: InvalidOutput,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runner := &fakeRunner{responses: []fakeResponse{
				{stdout: `[{"name":"cloud","type":"drive","source":"file","description":""}]`}, tc.response,
			}}
			_, err := testClient(runner).List(context.Background(), RemotePath{Remote: "cloud"})
			assertErrorKind(t, err, tc.kind)
		})
	}
}

func TestListRejectsInvalidFileMetadata(t *testing.T) {
	runner := &fakeRunner{responses: []fakeResponse{
		{stdout: `[{"name":"cloud","type":"drive","source":"file","description":""}]`},
		{stdout: `[{"Path":"file","Name":"file","Size":-1,"ModTime":"2026-01-01T00:00:00Z","IsDir":false}]`},
	}}
	_, err := testClient(runner).List(context.Background(), RemotePath{Remote: "cloud"})
	assertErrorKind(t, err, InvalidOutput)
}

func TestDownloadPublishesOneRegularFileWithoutOverwrite(t *testing.T) {
	dir := t.TempDir()
	runner := &fakeRunner{responses: []fakeResponse{
		{stdout: `[{"name":"cloud","type":"drive","source":"file","description":""}]`},
		{run: func(args []string) error {
			if err := os.WriteFile(args[2], []byte("downloaded"), 0o600); err != nil {
				return err
			}
			return nil
		}},
	}}
	client := testClient(runner)
	entry := Entry{Remote: "cloud", Path: "資料/report final.txt", Name: "report final.txt", Size: 10}

	got, err := client.Download(context.Background(), entry, dir)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "report final.txt")
	if got != want {
		t.Fatalf("Download path = %q, want %q", got, want)
	}
	data, err := os.ReadFile(want)
	if err != nil || string(data) != "downloaded" {
		t.Fatalf("downloaded data = %q, %v", data, err)
	}
	assertArgsPrefix(t, runner.calls[1], "copyto", "cloud:資料/report final.txt")
	if filepath.Dir(runner.calls[1][2]) != dir || filepath.Base(runner.calls[1][2]) == entry.Name {
		t.Fatalf("copy destination is not a same-directory temp file: %q", runner.calls[1][2])
	}
	assertNoTempFiles(t, dir)

	beforeCalls := len(runner.calls)
	_, err = client.Download(context.Background(), entry, dir)
	assertErrorKind(t, err, AlreadyExists)
	if len(runner.calls) != beforeCalls {
		t.Fatalf("existing destination caused commands: %v", runner.calls[beforeCalls:])
	}
}

func TestDownloadRejectsDirectoryAndUnsafeEntry(t *testing.T) {
	client := testClient(&fakeRunner{})
	dir := t.TempDir()
	for _, entry := range []Entry{
		{Remote: "cloud", Path: "folder", Name: "folder", IsDir: true},
		{Remote: "cloud", Path: "../escape", Name: "escape"},
		{Remote: "cloud", Path: "safe/file.txt", Name: "../escape"},
	} {
		_, err := client.Download(context.Background(), entry, dir)
		assertErrorKind(t, err, InvalidPath)
	}
}

func TestDownloadCleansTempOnFailure(t *testing.T) {
	dir := t.TempDir()
	runner := &fakeRunner{responses: []fakeResponse{
		{stdout: `[{"name":"cloud","type":"drive","source":"file","description":""}]`},
		{run: func(args []string) error {
			if err := os.WriteFile(args[2], []byte("partial"), 0o600); err != nil {
				return err
			}
			return errors.New("transfer failed")
		}},
	}}
	_, err := testClient(runner).Download(context.Background(), Entry{
		Remote: "cloud", Path: "file.txt", Name: "file.txt",
	}, dir)
	assertErrorKind(t, err, CommandFailed)
	assertNoTempFiles(t, dir)
	if _, statErr := os.Stat(filepath.Join(dir, "file.txt")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("destination exists after failure: %v", statErr)
	}
}

func TestDownloadDoesNotOverwriteDestinationCreatedDuringTransfer(t *testing.T) {
	dir := t.TempDir()
	destination := filepath.Join(dir, "file.txt")
	runner := &fakeRunner{responses: []fakeResponse{
		{stdout: `[{"name":"cloud","type":"drive","source":"file","description":""}]`},
		{run: func(args []string) error {
			if err := os.WriteFile(args[2], []byte("downloaded"), 0o600); err != nil {
				return err
			}
			return os.WriteFile(destination, []byte("winner"), 0o600)
		}},
	}}
	_, err := testClient(runner).Download(context.Background(), Entry{
		Remote: "cloud", Path: "file.txt", Name: "file.txt",
	}, dir)
	assertErrorKind(t, err, AlreadyExists)
	data, readErr := os.ReadFile(destination)
	if readErr != nil || string(data) != "winner" {
		t.Fatalf("destination was overwritten: data=%q err=%v", data, readErr)
	}
	assertNoTempFiles(t, dir)
}

func TestDownloadCleansTempOnCancellation(t *testing.T) {
	dir := t.TempDir()
	runner := &fakeRunner{responses: []fakeResponse{
		{stdout: `[{"name":"cloud","type":"drive","source":"file","description":""}]`},
		{run: func(args []string) error {
			if err := os.WriteFile(args[2], []byte("partial"), 0o600); err != nil {
				return err
			}
			return context.Canceled
		}},
	}}
	_, err := testClient(runner).Download(context.Background(), Entry{
		Remote: "cloud", Path: "file.txt", Name: "file.txt",
	}, dir)
	assertErrorKind(t, err, Cancelled)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("errors.Is(context.Canceled) = false: %v", err)
	}
	assertNoTempFiles(t, dir)
}

func TestDownloadRejectsInvalidLocalDirectory(t *testing.T) {
	file := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := testClient(&fakeRunner{}).Download(context.Background(), Entry{
		Remote: "cloud", Path: "file.txt", Name: "file.txt",
	}, file)
	assertErrorKind(t, err, InvalidPath)
}

func TestValidLocalName(t *testing.T) {
	if !validLocalName("資料 final.txt") {
		t.Fatal("valid Unicode filename rejected")
	}
	for _, name := range []string{"", ".", "..", "../escape", `dir\file`} {
		if validLocalName(name) {
			t.Fatalf("unsafe local name accepted: %q", name)
		}
	}
}

func TestTypedErrorFormatsWithoutDuplicatingStderr(t *testing.T) {
	cause := errors.New("exit status 1: unauthorized")
	err := newError("list", "cloud:", Authentication, cause, "unauthorized")
	message := err.Error()
	if !strings.Contains(message, `remote: list failed for "cloud:" (authentication)`) ||
		strings.Count(message, "unauthorized") != 1 {
		t.Fatalf("Error() = %q", message)
	}
}

func testClient(runner commandRunner) *Client {
	client, err := newClient(func(string) (string, error) { return "rclone-test", nil }, runner, Options{
		// These are filesystem/validation tests, not latency tests. Keep the
		// production budget so loaded Windows filesystems do not mask errors.
		Timeout:        defaultTimeout,
		MaxOutputBytes: 1024,
	})
	if err != nil {
		panic(err)
	}
	return client
}

func assertErrorKind(t *testing.T, err error, want ErrorKind) {
	t.Helper()
	var typed *Error
	if !errors.As(err, &typed) || typed.Kind != want {
		t.Fatalf("error = %v, want kind %q", err, want)
	}
}

func assertArgs(t *testing.T, got []string, want ...string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %#v, want %#v", got, want)
	}
}

func assertArgsPrefix(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) < len(want) || !reflect.DeepEqual(got[:len(want)], want) {
		t.Fatalf("args = %#v, want prefix %#v", got, want)
	}
}

func assertNoTempFiles(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".superfile-rclone-") {
			t.Fatalf("temporary file remains: %q", entry.Name())
		}
	}
}
