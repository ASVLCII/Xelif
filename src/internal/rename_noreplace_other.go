//go:build !windows && !linux && !darwin

package internal

func renameNoReplace(_, _ string) (bool, error) {
	return false, nil
}

func isCrossDeviceRenameError(_ error) bool {
	return false
}
