//go:build !windows

package globalsearch

import (
	"context"
	"errors"
)

func searchEverythingDirect(context.Context, string, int) ([]Result, error) {
	return nil, errors.New("Everything IPC is only available on Windows")
}
