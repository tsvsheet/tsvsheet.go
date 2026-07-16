package cli

import (
	"io"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/uplang/tsvsheet.go/internal/constants"
	"github.com/uplang/tsvsheet.go/internal/loader"
	"github.com/uplang/tsvsheet.go/internal/refresh"
	"github.com/uplang/tsvsheet.go/internal/session"
	"github.com/uplang/tsvsheet.go/internal/sheet"
	"github.com/uplang/tsvsheet.go/internal/tui"
)

// runProgram runs a bubbletea model over the given streams. It is a package
// variable so tests substitute a headless runner in place of the real TTY
// program.
var runProgram = func(model tea.Model, in io.Reader, out io.Writer) error {
	_, err := tea.NewProgram(model, tea.WithInput(in), tea.WithOutput(out)).Run()
	return err
}

// runTUI loads the spreadsheet into a session and edits it in the terminal UI.
// The sheet must be a file — the TUI saves edits back to it.
func runTUI(streams Streams, cfg tuiConfig) error {
	sess, persist, err := loadEditable(cfg.source, cfg.isUnconfined, cfg.limits)
	if err != nil {
		return err
	}
	next, err := buildRefresh(refresh.Spec(cfg.refresh), sess)
	if err != nil {
		return err
	}
	return runProgram(tui.New(sess, tui.Saver(persist), next), streams.In, streams.Out)
}

// loadEditable reads a file-backed spreadsheet into a session and returns it
// with a persist function that writes edits back to that file. Shared by serve
// and tui, both of which require a file source so edits can be saved. isUnconfined
// selects the confined or unconfined sheet loader.
func loadEditable(
	source sourcePath,
	isUnconfined pathAccess,
	limits sheet.Limits,
) (*session.Session, func() error, error) {
	if source.isStdin() {
		const msg = "requires a spreadsheet file path (e.g. `tsvsheet serve sheet.tsvt`)"
		return nil, nil, constants.ErrInvalidValue.With(nil, "message", msg)
	}
	path := filepath.Clean(string(source))
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, constants.ErrOpenFile.With(err, path)
	}
	// Resolve SHEET(...) and "file"! references within the spreadsheet's own
	// directory (or any path with isUnconfined), with this file as the base.
	load := sheetLoader(loader.Dir(filepath.Dir(path)), isUnconfined)
	sess, err := session.NewEmbeddable(src, load, sheet.Path(filepath.Base(path)), limits)
	if err != nil {
		return nil, nil, err
	}
	return sess, saver(sess, source), nil
}
