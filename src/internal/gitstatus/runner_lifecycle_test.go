package gitstatus

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const inheritedPipeHelperPattern = "-test.run=^TestGitInheritedPipeHelper$"

func TestGitRunnerBoundsInheritedOutputPipes(t *testing.T) {
	root := t.TempDir()
	executable, err := os.Executable()
	require.NoError(t, err)
	client := New(Options{Timeout: 20 * time.Second})
	client.gitBinary = executable
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, runErr := client.run(ctx, "inherited pipe test", root, root, false,
			inheritedPipeHelperPattern, "--", "parent", root)
		result <- runErr
	}()
	t.Cleanup(func() {
		// Release the test-owned descendant even when an assertion fails.
		require.NoError(t, os.WriteFile(filepath.Join(root, "release"), nil, 0o600))
		cancel()
		require.Eventually(t, func() bool {
			_, statErr := os.Stat(filepath.Join(root, "child-exited"))
			return statErr == nil
		}, 5*time.Second, 10*time.Millisecond)
	})
	require.Eventually(t, func() bool {
		_, statErr := os.Stat(filepath.Join(root, "parent-ready"))
		return statErr == nil
	}, 10*time.Second, 10*time.Millisecond)
	select {
	case runErr := <-result:
		require.ErrorIs(t, runErr, exec.ErrWaitDelay, "truncated pipe output must not be reported as success")
		assertErrorKind(t, runErr, CommandFailed)
	case <-time.After(8 * commandPipeWaitDelay):
		t.Fatal("Git runner waited on output inherited by a descendant")
	}
}

// A shell-free helper creates the inherited-pipe condition on every native OS.
// It is inert during ordinary test discovery and has a bounded fallback lifetime.
func TestGitInheritedPipeHelper(t *testing.T) {
	if len(os.Args) < 3 || os.Args[len(os.Args)-3] != "--" {
		return
	}
	mode, root := os.Args[len(os.Args)-2], os.Args[len(os.Args)-1]
	if mode == "child" {
		require.NoError(t, os.WriteFile(filepath.Join(root, "child-ready"), nil, 0o600))
		waitForPipeHelperFile(t, filepath.Join(root, "release"))
		require.NoError(t, os.WriteFile(filepath.Join(root, "child-exited"), nil, 0o600))
		return
	}
	if mode != "parent" {
		t.Fatal("unknown inherited pipe helper mode")
	}
	executable, err := os.Executable()
	require.NoError(t, err)
	child := exec.Command(executable, inheritedPipeHelperPattern, "--", "child", root)
	child.Stdout, child.Stderr = os.Stdout, os.Stderr
	require.NoError(t, child.Start())
	require.NoError(t, child.Process.Release())
	waitForPipeHelperFile(t, filepath.Join(root, "child-ready"))
	require.NoError(t, os.WriteFile(filepath.Join(root, "parent-ready"), nil, 0o600))
}

func waitForPipeHelperFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		_, err := os.Stat(path)
		if err == nil {
			return
		}
		require.ErrorIs(t, err, os.ErrNotExist)
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("test pipe helper did not receive its bounded lifecycle signal")
}
