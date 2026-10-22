package internal

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShowUserErrorOpensConsistentNotification(t *testing.T) {
	m := defaultTestModel(t.TempDir())

	m.showUserError("Could not open item", errors.New("application unavailable"))

	require.True(t, m.notifyModel.IsOpen())
	assert.Equal(t, "Could not open item", m.notifyModel.GetTitle())
	assert.Equal(t, "application unavailable", m.notifyModel.GetContent())
}

func TestEditorFailureIsShownToUser(t *testing.T) {
	m := defaultTestModel(t.TempDir())

	TeaUpdate(m, editorFinishedMsg{err: errors.New("editor exited with status 1")})

	require.True(t, m.notifyModel.IsOpen())
	assert.Equal(t, "Editor failed", m.notifyModel.GetTitle())
	assert.Equal(t, "editor exited with status 1", m.notifyModel.GetContent())
}
