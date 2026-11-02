package profile

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const maxProfileFileBytes = 8 << 20

// Refuse symlinks both at the leaf and in existing parent directories. Import
// must not write through a profile junction into a source or unrelated folder.
func safePath(path string) error {
	path = canonicalProfilePath(path)
	for current := filepath.Clean(path); ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return errors.New("profile symlinks are not imported or replaced")
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return nil
		}
	}
}

func readRegularFile(path string) ([]byte, error) {
	if err := safePath(path); err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("profile data must be a regular file")
	}
	if info.Size() > maxProfileFileBytes {
		return nil, errors.New("profile file exceeds the 8 MiB import limit")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxProfileFileBytes+1))
	if err == nil && len(data) > maxProfileFileBytes {
		return nil, errors.New("profile file exceeds the 8 MiB import limit")
	}
	return data, err
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func atomicWrite(path string, data []byte) error {
	if err := safePath(path); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, privateDirMode); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, ".xelif-profile-*")
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
		_ = os.Remove(file.Name())
	}()
	if err := file.Chmod(privateFileMode); err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return fmt.Errorf("replace profile file: %w", err)
	}
	return nil
}
