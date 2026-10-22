package gitstatus

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestMissingWorkingDirectoryDoesNotMeanGitUnavailable(t *testing.T) {
	binary, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not installed")
	}
	client := New(Options{})
	client.gitBinary = binary
	_, err = client.run(context.Background(), "check executable", "", filepath.Join(t.TempDir(), "missing"),
		true, "--version")
	assertErrorKind(t, err, CommandFailed)
}
