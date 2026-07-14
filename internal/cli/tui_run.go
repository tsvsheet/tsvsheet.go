package cli

import (
	"io"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/uplang/tsvsheet.go/internal/constants"
	"github.com/uplang/tsvsheet.go/internal/session"
	"github.com/uplang/tsvsheet.go/internal/tui"
)

// runProgram runs a bubbletea model over the given streams. It is a package
// variable so tests substitute a headless runner in place of the real TTY
// program.
var runProgram = func(model tea.Model, in io.Reader, out io.Writer) error {
	_, err := tea.NewProgram(model, tea.WithInput(in), tea.WithOutput(out)).Run()
	return err
}

// runTUI loads the worksheet into a session and edits it in the terminal UI.
// Both template and data must be files — the TUI saves edits back to them.
func runTUI(streams Streams, cfg tuiConfig) error {
	sess, persist, err := loadEditable(cfg.template, cfg.data)
	if err != nil {
		return err
	}
	return runProgram(tui.New(sess, tui.Saver(persist)), streams.In, streams.Out)
}

// loadEditable reads a file-backed worksheet into a session and returns it with
// a persist function that writes edits back to those files. Shared by serve and
// tui, both of which require file sources so edits can be saved.
func loadEditable(template, data sourcePath) (*session.Session, func() error, error) {
	if template.isStdin() || data.isStdin() {
		return nil, nil, constants.ErrInvalidValue.With(nil, "message", "requires file paths for --template and --data")
	}
	sess, err := loadSession(template, data)
	if err != nil {
		return nil, nil, err
	}
	return sess, saver(sess, template, data), nil
}
