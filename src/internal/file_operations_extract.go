package internal

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"golift.io/xtractr"

	"github.com/yorukot/superfile/src/pkg/utils"

	"github.com/yorukot/superfile/src/internal/ui/processbar"
)

func extractCompressFile(src, dest string, processBar *processbar.Model) error {
	p, err := processBar.SendAddProcessMsg(filepath.Base(src), processbar.OpExtract, 1, true)
	if err != nil {
		return fmt.Errorf("cannot spawn process : %w", err)
	}

	x := &xtractr.XFile{
		Context:   processBar.ProcessContext(p.ID),
		FilePath:  src,
		OutputDir: dest,
		FileMode:  utils.ExtractedFileMode,
		DirMode:   utils.ExtractedDirMode,
	}
	p.Source, p.Destination = src, dest

	_, _, _, err = xtractr.ExtractFile(x)

	if err != nil {
		p.State = processbar.Failed
		p.ErrorMsg = err.Error()
		p.Partial = "Complete extracted entries are retained in " + dest
		if errors.Is(err, context.Canceled) {
			p.State = processbar.Cancelled
		}
		slog.Error("Error extracting", "path", src, "error", err)
	} else {
		p.State = processbar.Successful
		p.Done = 1
	}

	p.DoneTime = time.Now()
	pSendErr := processBar.SendUpdateProcessMsg(p, true)
	if pSendErr != nil {
		slog.Error("Error sending process update", "error", pSendErr)
	}

	return err
}
