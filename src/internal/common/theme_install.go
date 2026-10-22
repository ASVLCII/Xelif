package common

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/yorukot/superfile/src/pkg/utils"
)

func installMissingTheme(path, data string) error {
	if err := os.MkdirAll(filepath.Dir(path), utils.ConfigDirPerm); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, utils.ConfigFilePerm)
	if errors.Is(err, os.ErrExist) {
		return nil
	}
	if err != nil {
		return err
	}
	_, writeErr := file.WriteString(data)
	return errors.Join(writeErr, file.Close())
}
