package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uplang/tsvsheet.go/internal/constants"
)

// withRunProgram swaps the tea program runner for a test double.
func withRunProgram(t *testing.T, fn func(tea.Model, io.Reader, io.Writer) error) {
	t.Helper()
	prev := runProgram
	runProgram = fn
	t.Cleanup(func() { runProgram = prev })
}

func TestRunTUI_LoadsAndRuns(t *testing.T) {
	var gotModel tea.Model
	withRunProgram(t, func(m tea.Model, _ io.Reader, _ io.Writer) error {
		gotModel = m
		return nil
	})

	tmpl, data := worksheetFiles(t)
	streams := Streams{In: strings.NewReader(""), Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
	require.NoError(t, runTUI(streams, tuiConfig{template: tmpl, data: data}))
	assert.NotNil(t, gotModel)
}

func TestRunTUI_RequiresFiles(t *testing.T) {
	t.Parallel()

	err := runTUI(Streams{In: strings.NewReader("")}, tuiConfig{template: "-", data: "-"})
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrInvalidValue)
}

func TestRunTUI_ProgramError(t *testing.T) {
	withRunProgram(t, func(tea.Model, io.Reader, io.Writer) error {
		return errors.New("tea boom")
	})

	tmpl, data := worksheetFiles(t)
	err := runTUI(Streams{In: strings.NewReader(""), Out: &bytes.Buffer{}}, tuiConfig{template: tmpl, data: data})
	require.Error(t, err)
}

func TestTUICommand_Integration(t *testing.T) {
	withRunProgram(t, func(tea.Model, io.Reader, io.Writer) error { return nil })

	tmpl, data := worksheetFiles(t)
	cmd := tuiCommand()
	err := cmd.Run(context.Background(), []string{cmdTUI, string(tmpl), string(data)})
	require.NoError(t, err)
}

func TestDefaultRunProgram_QuitsOnInput(t *testing.T) {
	// Drive the real (unswapped) runProgram headlessly: feed "q" so the model
	// quits and Run returns, exercising the default tea.Program path without a
	// TTY.
	tmpl, data := worksheetFiles(t)
	streams := Streams{In: strings.NewReader("q"), Out: io.Discard, Err: io.Discard}
	require.NoError(t, runTUI(streams, tuiConfig{template: tmpl, data: data}))
}
