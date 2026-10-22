package session

import (
	"fmt"
	"os"
	"path/filepath"
)

const privateFileMode os.FileMode = 0o600

func atomicWriteFile(path string, data []byte) error {
	return atomicWriteFileWithReplace(path, data, os.Rename)
}

func atomicWriteFileWithReplace(path string, data []byte, replace func(string, string) error) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}

	temporary, err := os.CreateTemp(directory, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary state file: %w", err)
	}
	temporaryPath := temporary.Name()
	temporaryClosed := false
	defer func() {
		if !temporaryClosed {
			_ = temporary.Close()
		}
		if temporaryPath != "" {
			_ = os.Remove(temporaryPath)
		}
	}()

	if err := temporary.Chmod(privateFileMode); err != nil {
		return fmt.Errorf("set temporary state file permissions: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		return fmt.Errorf("write temporary state file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync temporary state file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary state file: %w", err)
	}
	temporaryClosed = true
	if err := replace(temporaryPath, path); err != nil {
		return fmt.Errorf("replace state file: %w", err)
	}
	temporaryPath = ""
	if err := syncDirectory(directory); err != nil {
		return fmt.Errorf("sync state directory: %w", err)
	}
	return nil
}
