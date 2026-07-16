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
	"github.com/uplang/go-tsvsheet"
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

	streams := Streams{In: strings.NewReader(""), Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
	require.NoError(t, runTUI(streams, tuiConfig{source: sheetFile(t)}))
	assert.NotNil(t, gotModel)
}

func TestRunTUI_RequiresFile(t *testing.T) {
	t.Parallel()

	err := runTUI(Streams{In: strings.NewReader("")}, tuiConfig{source: "-"})
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrInvalidValue)
}

func TestRunTUI_ProgramError(t *testing.T) {
	withRunProgram(t, func(tea.Model, io.Reader, io.Writer) error {
		return errors.New("tea boom")
	})

	err := runTUI(Streams{In: strings.NewReader(""), Out: &bytes.Buffer{}}, tuiConfig{source: sheetFile(t)})
	require.Error(t, err)
}

func TestRunTUI_BadRefreshSpec(t *testing.T) {
	t.Parallel()

	// A malformed --refresh-interval fails before the program runs: runTUI
	// surfaces the buildRefresh error rather than starting the editor.
	err := runTUI(
		Streams{In: strings.NewReader(""), Out: &bytes.Buffer{}, Err: &bytes.Buffer{}},
		tuiConfig{source: sheetFile(t), refresh: "garbage!!!"},
	)
	require.Error(t, err)
}

func TestTUICommand_Integration(t *testing.T) {
	withRunProgram(t, func(tea.Model, io.Reader, io.Writer) error { return nil })

	cmd := tuiCommand()
	err := cmd.Run(context.Background(), []string{cmdTUI, string(sheetFile(t))})
	require.NoError(t, err)
}

func TestDefaultRunProgram_QuitsOnInput(t *testing.T) {
	// Drive the real (unswapped) runProgram headlessly: feed "q" so the model
	// quits and Run returns, exercising the default tea.Program path without a
	// TTY.
	streams := Streams{In: strings.NewReader("q"), Out: io.Discard, Err: io.Discard}
	require.NoError(t, runTUI(streams, tuiConfig{source: sheetFile(t)}))
}
