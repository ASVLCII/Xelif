package internal

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/yorukot/superfile/src/internal/ui/processbar"
)

var suffixRegexp = regexp.MustCompile(`^(.*)\((\d+)\)$`)

func checkFileNameValidity(name string) error {
	switch {
	case name == ".", name == "..":
		return errors.New("file name cannot be '.' or '..'")
	case strings.HasSuffix(name, fmt.Sprintf("%c.", filepath.Separator)),
		strings.HasSuffix(name, fmt.Sprintf("%c..", filepath.Separator)):
		return fmt.Errorf("file name cannot end with '%c.' or '%c..'", filepath.Separator, filepath.Separator)
	default:
		return nil
	}
}

func renameIfDuplicate(destination string) (string, error) {
	if _, err := os.Stat(destination); os.IsNotExist(err) {
		return destination, nil
	} else if err != nil {
		return "", err
	}

	dir := filepath.Dir(destination)
	base := filepath.Base(destination)
	ext := filepath.Ext(base)
	name := base[:len(base)-len(ext)]

	// Extract base name without existing suffix
	counter := 1
	//nolint:mnd // 3 = full match + 2 capture groups
	if match := suffixRegexp.FindStringSubmatch(name); len(match) == 3 {
		name = match[1] // base name without (N)
		if num, err := strconv.Atoi(match[2]); err == nil {
			counter = num + 1 // start from next number
		}
	}

	// Find first available name
	for i := counter; i < 10_000; i++ {
		newName := fmt.Sprintf("%s(%d)%s", name, i, ext)
		newPath := filepath.Join(dir, newName)
		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			return newPath, nil
		}
	}

	return "", fmt.Errorf("could not find free name for %s after many attempts", destination)
}

// Count how many file in the directory
func countFiles(dirPath string) (int, error) {
	count := 0

	err := filepath.Walk(dirPath, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			count++
		}
		return nil
	})

	return count, err
}

func countReadableFiles(dirPath string) (int, error) {
	count := 0

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			if err = checkFileReadable(path); err != nil {
				slog.Error("the file is not readable", "error", err)
				return fmt.Errorf("the file is not readable: %s", err.Error())
			}
			count++
		}
		return nil
	})

	return count, err
}

func processCmdToTeaCmd(cmd processbar.Cmd) tea.Cmd {
	if cmd == nil {
		// To prevent us from running cmd() on nil cmd
		return nil
	}
	return func() tea.Msg {
		updateMsg := cmd()
		return ProcessBarUpdateMsg{
			pMsg: updateMsg,
			BaseMessage: BaseMessage{
				reqID: updateMsg.GetReqID(),
			},
		}
	}
}

func getCopyOrCutOperationName(cut bool) string {
	if cut {
		return "cut"
	}
	return "copy"
}
