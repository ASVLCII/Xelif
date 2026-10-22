package remote

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

// Download explicitly transfers one regular Entry into localDir. It publishes
// a completed same-directory temporary file atomically and never overwrites an
// existing destination.
func (c *Client) Download(ctx context.Context, entry Entry, localDir string) (string, error) {
	remotePath, entryErr := validateDownloadEntry(entry)
	if entryErr != nil {
		return "", entryErr
	}
	directory, destination, destinationErr := validateDownloadDestination(remotePath, entry.Name, localDir)
	if destinationErr != nil {
		return "", destinationErr
	}
	ctx, cancel := c.operationContext(ctx)
	defer cancel()
	if configureErr := c.ensureConfigured(ctx, remotePath.Remote); configureErr != nil {
		return "", configureErr
	}

	temporaryPath, temporaryErr := createDownloadTemp(remotePath, directory)
	if temporaryErr != nil {
		return "", temporaryErr
	}
	defer os.Remove(temporaryPath)

	_, runErr := c.run(ctx, "download", remotePath.String(),
		"copyto", remotePath.String(), temporaryPath, "--inplace", "--ask-password=false")
	if runErr != nil {
		return "", runErr
	}
	if publishErr := publishDownload(remotePath, temporaryPath, destination); publishErr != nil {
		return "", publishErr
	}
	return destination, nil
}

func validateDownloadEntry(entry Entry) (RemotePath, error) {
	rawPath := RemotePath{Remote: entry.Remote, Path: entry.Path}
	remotePath, err := normalizeRemotePath(rawPath, false)
	if err == nil && (entry.IsDir || entry.Name == "" || path.Base(remotePath.Path) != entry.Name) {
		err = errors.New("entry is not a selected regular file")
	}
	if err != nil {
		return RemotePath{}, newError("download", rawPath.String(), InvalidPath, err, "")
	}
	return remotePath, nil
}

func validateDownloadDestination(remotePath RemotePath, name, localDir string) (string, string, error) {
	if localDir == "" || !validLocalName(name) {
		return "", "", newError("download", remotePath.String(), InvalidPath, errUnsafePath, "")
	}
	directory, err := filepath.Abs(filepath.Clean(localDir))
	if err != nil {
		return "", "", newError("download", remotePath.String(), InvalidPath, err, "")
	}
	info, err := os.Stat(directory)
	if err != nil {
		return "", "", newError("download", remotePath.String(), InvalidPath, err, "")
	}
	if !info.IsDir() {
		return "", "", newError(
			"download",
			remotePath.String(),
			InvalidPath,
			errors.New("local destination is not a directory"),
			"",
		)
	}
	destination := filepath.Join(directory, name)
	relative, err := filepath.Rel(directory, destination)
	if err != nil || relative != name {
		return "", "", newError("download", remotePath.String(), InvalidPath, errUnsafePath, "")
	}
	if _, statErr := os.Lstat(destination); statErr == nil {
		return "", "", newError("download", remotePath.String(), AlreadyExists, fs.ErrExist, "")
	} else if !errors.Is(statErr, fs.ErrNotExist) {
		return "", "", newError("download", remotePath.String(), InvalidPath, statErr, "")
	}
	return directory, destination, nil
}

func createDownloadTemp(remotePath RemotePath, directory string) (string, error) {
	temporary, err := os.CreateTemp(directory, ".superfile-rclone-*")
	if err != nil {
		return "", newError(
			"download",
			remotePath.String(),
			CommandFailed,
			fmt.Errorf("create temporary file: %w", err),
			"",
		)
	}
	temporaryPath := temporary.Name()
	if closeErr := temporary.Close(); closeErr != nil {
		_ = os.Remove(temporaryPath)
		return "", newError(
			"download",
			remotePath.String(),
			CommandFailed,
			fmt.Errorf("close temporary file: %w", closeErr),
			"",
		)
	}
	return temporaryPath, nil
}

func publishDownload(remotePath RemotePath, temporaryPath, destination string) error {
	if err := os.Link(temporaryPath, destination); err != nil {
		if _, existsErr := os.Lstat(destination); existsErr == nil || errors.Is(err, fs.ErrExist) {
			return newError("download", remotePath.String(), AlreadyExists, fs.ErrExist, "")
		}
		return newError(
			"download",
			remotePath.String(),
			CommandFailed,
			fmt.Errorf("publish temporary file: %w", err),
			"",
		)
	}
	return nil
}

func validLocalName(name string) bool {
	if filepath.Base(name) != name || filepath.IsAbs(name) || name == "." || name == ".." {
		return false
	}
	if runtime.GOOS != "windows" {
		return true
	}
	if strings.ContainsAny(name, `<>:"/\|?*`) || strings.HasSuffix(name, " ") || strings.HasSuffix(name, ".") {
		return false
	}
	base := strings.ToUpper(strings.TrimSuffix(name, filepath.Ext(name)))
	switch base {
	case "CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		return false
	}
	return true
}
