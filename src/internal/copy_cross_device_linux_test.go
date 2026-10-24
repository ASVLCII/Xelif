package internal

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/yorukot/superfile/src/internal/ui/processbar"
)

func TestCopyRejectsFIFOWithoutBlocking(t *testing.T) {
	if operation := os.Getenv("XELIF_TEST_FIFO_COPY"); operation != "" {
		source := os.Getenv("XELIF_TEST_FIFO_SOURCE")
		destination := source + "-destination"
		var err error
		if operation == "copy-file" {
			info, statErr := os.Lstat(source)
			require.NoError(t, statErr)
			err = copyFile(source, destination, info)
		} else {
			err = copyElementNoReplace(source, destination)
		}
		require.ErrorIs(t, err, errors.ErrUnsupported)
		_, err = os.Lstat(destination)
		require.ErrorIs(t, err, os.ErrNotExist)
		return
	}
	for _, operation := range []string{"copy-file", "move-fallback"} {
		t.Run(operation, func(t *testing.T) {
			source := filepath.Join(t.TempDir(), "pipe")
			require.NoError(t, unix.Mkfifo(source, 0o600))
			// A regression would block in Open waiting for a FIFO writer. Bound
			// that wait in a subprocess so it cannot hang the complete suite.
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCopyRejectsFIFOWithoutBlocking$", "-test.v")
			command.Env = append(os.Environ(), "XELIF_TEST_FIFO_COPY="+operation, "XELIF_TEST_FIFO_SOURCE="+source)
			output, err := command.CombinedOutput()
			require.NoError(t, ctx.Err(), "copy blocked waiting for FIFO input: %s", output)
			require.NoError(t, err, "%s", output)
		})
	}
}

func TestCutAcrossFilesystems(t *testing.T) {
	//nolint:usetesting // The source must use /tmp's filesystem, independently of TMPDIR used by t.TempDir.
	sourceRoot, err := os.MkdirTemp("/tmp", "xelif-cross-device-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(sourceRoot) })
	destinationRoot := t.TempDir()
	var sourceStat, destinationStat unix.Stat_t
	require.NoError(t, unix.Stat(sourceRoot, &sourceStat))
	require.NoError(t, unix.Stat(destinationRoot, &destinationStat))
	if sourceStat.Dev == destinationStat.Dev {
		t.Skip("/tmp and the test destination are on the same filesystem")
	}
	for _, operation := range []string{"paste-helper", "clipboard-cut"} {
		for _, kind := range []string{"file", "directory", "symlink"} {
			t.Run(operation+"/"+kind, func(t *testing.T) {
				source := filepath.Join(sourceRoot, operation+"-"+kind)
				destination := filepath.Join(destinationRoot, filepath.Base(source))
				makeCrossDeviceSource(t, source, kind)
				if operation == "paste-helper" {
					bar := processbar.New()
					p := processbar.Process{}
					require.NoError(t, pasteDir(source, destination, &p, true, &bar))
				} else {
					var moves []pathMove
					require.NoError(t, movePasteItemContext(context.Background(), source, destinationRoot, &moves))
					require.Equal(t, []pathMove{{from: source, to: destination}}, moves)
				}
				_, statErr := os.Lstat(source)
				require.ErrorIs(t, statErr, os.ErrNotExist, "cut must remove its original")
				assertCrossDeviceDestination(t, destination, kind)
			})
		}
	}
}

func makeCrossDeviceSource(t *testing.T, source, kind string) {
	t.Helper()
	switch kind {
	case "directory":
		require.NoError(t, os.MkdirAll(filepath.Join(source, "nested"), 0o751))
		require.NoError(
			t,
			os.WriteFile(filepath.Join(source, "nested", "file.txt"), []byte("cross-device content"), 0o640),
		)
		require.NoError(t, os.Symlink("missing-target", filepath.Join(source, "link")))
	case "file":
		require.NoError(t, os.WriteFile(source, []byte("cross-device content"), 0o640))
	case "symlink":
		require.NoError(t, os.Symlink("missing-target", source))
	}
}

func assertCrossDeviceDestination(t *testing.T, destination, kind string) {
	t.Helper()
	if kind == "symlink" {
		target, err := os.Readlink(destination)
		require.NoError(t, err)
		require.Equal(t, "missing-target", target)
		return
	}
	filePath := destination
	if kind == "directory" {
		filePath = filepath.Join(destination, "nested", "file.txt")
		target, err := os.Readlink(filepath.Join(destination, "link"))
		require.NoError(t, err)
		require.Equal(t, "missing-target", target)
	}
	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	require.Equal(t, "cross-device content", string(content))
}
