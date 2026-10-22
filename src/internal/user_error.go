package internal

import (
	"log/slog"

	"github.com/yorukot/superfile/src/internal/ui/notify"
)

func (m *model) showUserError(title string, err error) {
	if err == nil {
		return
	}

	slog.Error(title, "error", err)
	m.notifyModel = notify.New(true, title, err.Error(), notify.NoAction)
}
