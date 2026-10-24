package internal

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/yorukot/superfile/src/internal/ui/notify"
	"github.com/yorukot/superfile/src/internal/ui/processbar"
)

func validateCompressOperation(sources []string) (int, error) {
	totalFiles := 0
	for _, src := range sources {
		stat, err := os.Stat(src)
		if os.IsNotExist(err) {
			return 0, fmt.Errorf("source path does not exist: %s", src)
		}
		if err != nil {
			return 0, fmt.Errorf("cannot access source path %s: %w", src, err)
		}
		if !stat.IsDir() {
			if err = checkFileReadable(src); err != nil {
				slog.Error("the file is not readable", "error", err)
				return 0, fmt.Errorf("the file is not readable: %s", err.Error())
			}
		}
		count, err := countReadableFiles(src)
		if err != nil {
			slog.Error("Error while zip file count files ", "error", err)
			return 0, fmt.Errorf("error while counting files for %s: %s", src, err.Error())
		}
		totalFiles += count
	}
	return totalFiles, nil
}

func (m *model) getCompressSelectedFilesCmd() tea.Cmd {
	panel := m.getFocusedFilePanel()

	if panel.Empty() {
		return nil
	}
	var filesToCompress []string
	var firstFile string

	if panel.SelectedCount() == 0 {
		firstFile = panel.GetFocusedItem().Location
		filesToCompress = append(filesToCompress, firstFile)
	} else {
		firstFile = panel.GetFirstSelectedLocation()
		filesToCompress = panel.GetSelectedLocationsSortedAsVisible()
	}

	reqID := m.nextIoReqCnt()
	panelLocation := panel.Location

	return m.processBarModel.Run(func() tea.Msg {
		zipName, err := getZipArchiveName(filepath.Base(firstFile))
		if err != nil {
			slog.Error("Error in getZipArchiveName", "error", err)
			return NewNotifyModalMsg(notify.New(true, "Invalid zip target name", err.Error(), notify.NoAction),
				reqID)
		}
		zipPath, err := renameIfDuplicate(filepath.Join(panelLocation, zipName))
		if err != nil {
			return NewNotifyModalMsg(notify.New(true, "Invalid file/dir to compress", err.Error(), notify.NoAction),
				reqID)
		}
		if err := zipSources(filesToCompress, 0, zipPath, &m.processBarModel); err != nil {
			slog.Error("Error in zipping files", "error", err)
			return NewCompressOperationMsg(processbar.Failed, reqID)
		}
		return NewCompressOperationMsg(processbar.Successful, reqID)
	})
}

func zipSources(sources []string, totalFiles int, target string, processBar *processbar.Model) error {
	p, err := processBar.SendAddProcessMsg(filepath.Base(target), processbar.OpCompress, totalFiles, true)
	if err != nil {
		return fmt.Errorf("cannot spawn process: %w", err)
	}
	p.Source, p.Destination = strings.Join(sources, "; "), target
	err = createZipArchive(processBar.ProcessContext(p.ID), sources, target, processBar, &p)
	processBar.FinishTask(p, err)
	return err
}

func createZipArchive(
	ctx context.Context,
	sources []string,
	target string,
	processBar *processbar.Model,
	p *processbar.Process,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := os.Lstat(target); err == nil {
		return errors.New("file already exists")
	} else if !os.IsNotExist(err) {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(target), ".xelif-archive-*")
	if err != nil {
		return err
	}
	temporary := f.Name()
	identity, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return err
	}
	defer func() {
		_ = f.Close()
		if info, statErr := os.Lstat(temporary); statErr == nil && os.SameFile(identity, info) {
			_ = os.Remove(temporary)
		}
	}()
	archiveErr := writeZipContents(ctx, sources, processBar, p, f)
	fileErr := f.Close()
	if err := errors.Join(archiveErr, fileErr, ctx.Err()); err != nil {
		return err
	}
	if supported, err := renameNoReplace(temporary, target); supported {
		return err
	}
	return os.Link(temporary, target)
}

func writeZipContents(
	ctx context.Context,
	sources []string,
	processes *processbar.Model,
	p *processbar.Process,
	output io.Writer,
) error {
	writer := zip.NewWriter(output)
	walkErr := zipSourcesContext(ctx, sources, processes, p, writer)
	// Closing writes the ZIP central directory and can fail after every member
	// was copied successfully. This error must reach the publication boundary.
	return errors.Join(walkErr, writer.Close(), ctx.Err())
}

func zipSourcesContext(
	ctx context.Context,
	sources []string,
	processBar *processbar.Model,
	p *processbar.Process,
	writer *zip.Writer,
) error {
	for _, src := range sources {
		parent := filepath.Dir(src)
		err := filepath.Walk(src, func(path string, info os.FileInfo, walkErr error) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if walkErr != nil {
				return walkErr
			}
			p.CurrentFile = filepath.Base(path)
			relative, err := filepath.Rel(parent, path)
			if err != nil {
				return err
			}
			if err := writeZipFileContext(ctx, path, relative, info, writer); err != nil {
				return err
			}
			p.Done++
			processBar.TrySendingUpdateProcessMsg(*p)
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func writeZipFileContext(ctx context.Context, path, relPath string, info os.FileInfo, writer *zip.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Method, header.Name = zip.Deflate, filepath.ToSlash(relPath)
	if info.IsDir() {
		header.Name += "/"
	}
	output, err := writer.CreateHeader(header)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return nil
	}
	input, err := os.Open(path)
	if err != nil {
		return err
	}
	defer input.Close()
	_, err = io.Copy(output, cancellationReader{ctx: ctx, reader: input})
	return err
}

func getZipArchiveName(base string) (string, error) {
	if len(base) == 0 {
		return "", errors.New("empty filename to compress")
	}
	zipName := strings.TrimSuffix(base, filepath.Ext(base)) + ".zip"
	runes := []rune(base)
	if runes[0] == '.' && strings.Count(base, ".") == 1 {
		zipName = base + ".zip"
	}
	zipName, err := renameIfDuplicate(zipName)
	return zipName, err
}
