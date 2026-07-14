package process

import (
	"context"
	"io"
	"log/slog"
	"os"

	"github.com/gomatic/template.cli/internal/constants"
	"github.com/gomatic/template.cli/internal/domain"
	"github.com/gomatic/template.cli/internal/text"
)

// Result is the outcome of the process command.
type Result struct {
	Output text.Output `json:"output"`
}

// closeFunc releases an input source; it is a no-op for stdin.
type closeFunc func() error

// Run reads the input named in args (or stdin), applies the configured
// transformations line by line via the text package, and returns the result.
func Run(ctx context.Context, logger *slog.Logger, cfg Config, args ...domain.Argument) (Result, error) {
	reader, release, err := open(pathFrom(cfg, args))
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = release() }()

	output, err := text.Process(ctx, reader, transformFor(cfg))
	if err != nil {
		return Result{}, err
	}

	logger.Info("Processing completed.", "bytes", len(output))
	return Result{Output: output}, nil
}

// pathFrom resolves the input path from the positional arg or the config.
func pathFrom(cfg Config, args []string) filePath {
	if len(args) > 0 {
		return filePath(args[0])
	}
	return cfg.FilePath
}

// open returns a reader for path, using stdin when path is empty.
func open(path filePath) (io.Reader, closeFunc, error) {
	if path == "" {
		return os.Stdin, func() error { return nil }, nil
	}
	file, err := os.Open(string(path))
	if err != nil {
		return nil, nil, constants.ErrOpenFile.With(err, string(path))
	}
	return file, file.Close, nil
}

// transformFor builds the per-line transform from the command's flags.
func transformFor(cfg Config) text.Transform {
	return func(line text.Line, number text.LineNumber) (text.Line, bool) {
		if !included(cfg, line) {
			return "", false
		}
		return apply(cfg, line, number), true
	}
}

// included reports whether a line survives the configured filter.
func included(cfg Config, line text.Line) bool {
	return cfg.Filter == "" || text.Contains(line, text.Filter(cfg.Filter))
}

// apply runs the configured transformations over a kept line.
func apply(cfg Config, line text.Line, number text.LineNumber) text.Line {
	result := line
	if bool(cfg.UppercaseEnabled) {
		result = text.Uppercase(result)
	}
	if cfg.Prefix != "" {
		result = text.WithPrefix(result, text.Prefix(cfg.Prefix))
	}
	if bool(cfg.LineNumbersEnabled) {
		result = text.Numbered(result, number)
	}
	return result
}
