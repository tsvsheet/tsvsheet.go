package cli

import (
	"io"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tsvsheet/go-tsvsheet"

	"github.com/tsvsheet/tsvsheet.go/internal/constants"
	"github.com/tsvsheet/tsvsheet.go/internal/loader"
	"github.com/tsvsheet/tsvsheet.go/internal/refresh"
	"github.com/tsvsheet/tsvsheet.go/internal/session"
	"github.com/tsvsheet/tsvsheet.go/internal/tui"
)

// runProgram runs a bubbletea model over the given streams. It is a package
// variable so tests substitute a headless runner in place of the real TTY
// program.
var runProgram = func(model tea.Model, in io.Reader, out io.Writer) error {
	_, err := tea.NewProgram(model, tea.WithInput(in), tea.WithOutput(out)).Run()
	return err
}

// runTUI opens the spreadsheet at any size: within the resident cell budget
// it loads into an editing session exactly as before; over it, the census
// (one sequential scan, nothing materialized) routes to the view-only pager,
// which reads bounded viewport windows while the file stays on disk (spec
// 016). Raising --max-cells makes any size editable — budgets are policy.
func runTUI(streams Streams, cfg tuiConfig) error {
	if windowed, closeSource := openWindowed(cfg.source, cfg.limits); windowed != nil {
		defer func() { _ = closeSource() }()
		opts := tsvsheet.ComputeOptions{At: time.Now(), Limits: cfg.limits, Fetcher: cfg.fetcher}
		return runProgram(tui.NewPager(windowed, opts), streams.In, streams.Out)
	}
	sess, persist, err := loadEditable(cfg.source, cfg.isUnconfined, cfg.limits, cfg.fetcher)
	if err != nil {
		return err
	}
	wireRefresh(sess, cfg.cache)
	next, err := buildRefresh(refresh.Spec(cfg.refresh), sess)
	if err != nil {
		return err
	}
	return runProgram(tui.New(sess, tui.Saver(persist), next), streams.In, streams.Out)
}

// statSize reports the open file's size. It is a package variable so tests
// can force the stat failure branch — the sanctioned stdlib-fault seam.
var statSize = func(f *os.File) (int64, error) {
	info, err := f.Stat()
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// openWindowed opens the file through the engine's residency policy — one
// OpenSheet call with the caller's real limits, so policy lives in the engine
// and is never mirrored here. Only an over-budget document returns: the
// windowed capability with its block cache bounded by those same limits, plus
// a closer that keeps the file open for the pager's lifetime. EVERY other
// outcome — in budget (the resident sheet is discarded; a bounded double
// parse the budget itself caps), stdin, a missing file, a directory, a
// refused scan — returns nil and falls through to loadEditable, which stays
// the single voice for every load and every error, byte-identical to the
// pre-016 surface. The expensive case — a huge file — costs exactly one scan.
func openWindowed(source sourcePath, limits tsvsheet.Limits) (*tsvsheet.WindowedSheet, func() error) {
	if source.isStdin() {
		return nil, nil // stdin is refused by loadEditable's own message, even if a file named "-" exists
	}
	f, err := os.Open(filepath.Clean(string(source)))
	if err != nil {
		return nil, nil
	}
	size, err := statSize(f)
	if err != nil {
		_ = f.Close()
		return nil, nil
	}
	_, windowed, err := tsvsheet.OpenSheet(tsvsheet.ByteSource{ReadAt: f, Size: size}, limits)
	if err != nil || windowed == nil {
		_ = f.Close()
		return nil, nil
	}
	return windowed, f.Close
}

// loadEditable reads a file-backed spreadsheet into a session and returns it
// with a persist function that writes edits back to that file. Shared by serve
// and tui, both of which require a file source so edits can be saved. isUnconfined
// selects the confined or unconfined sheet loader.
func loadEditable(
	source sourcePath,
	isUnconfined pathAccess,
	limits tsvsheet.Limits,
	fetcher tsvsheet.Fetcher,
) (*session.Session, func() error, error) {
	if source.isStdin() {
		const msg = "requires a spreadsheet file path (e.g. `tsv serve sheet.tsvt`)"
		return nil, nil, tsvsheet.ErrInvalidValue.With(nil, "message", msg)
	}
	path := filepath.Clean(string(source))
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, constants.ErrOpenFile.With(err)
	}
	// Resolve SHEET(...) and "file"! references within the spreadsheet's own
	// directory (or any path with isUnconfined), with this file as the base;
	// content-typed IMPORT* cells fetch through the injected fetcher (nil off).
	load := sheetLoader(loader.Dir(filepath.Dir(path)), isUnconfined)
	sess, err := session.NewEmbeddable(src, load, tsvsheet.Path(filepath.Base(path)), limits, fetcher)
	if err != nil {
		return nil, nil, err
	}
	return sess, saver(sess, source), nil
}
