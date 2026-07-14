// Package text provides line-oriented text processing primitives and a
// streaming processor that applies a transform to each line of an input.
//
// The per-line operations are small and pure; Process owns the scanning,
// context-cancellation, and joining. The package holds no CLI or orchestration
// logic and is reusable by any domain that needs line processing.
package text

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/gomatic/template.cli/internal/constants"
)

type (
	// Line is a single line of input or output.
	Line string
	// Prefix is text prepended to a line.
	Prefix string
	// Filter is a substring a line must contain to be kept.
	Filter string
	// LineNumber is a one-based line position.
	LineNumber int
	// Output is the joined result of processing.
	Output string
	// Transform maps a numbered line to its processed form and whether to keep it.
	Transform func(Line, LineNumber) (Line, bool)
)

// Uppercase returns the line converted to uppercase.
func Uppercase(line Line) Line {
	return Line(strings.ToUpper(string(line)))
}

// WithPrefix returns the line with prefix prepended.
func WithPrefix(line Line, prefix Prefix) Line {
	return Line(string(prefix) + string(line))
}

// Numbered returns the line prefixed with its right-aligned line number.
func Numbered(line Line, number LineNumber) Line {
	return Line(fmt.Sprintf("%4d | %s", int(number), string(line)))
}

// Contains reports whether the line contains the filter substring.
func Contains(line Line, filter Filter) bool {
	return strings.Contains(string(line), string(filter))
}

// Process scans reader line by line, applies transform, and joins the kept
// lines with newlines. It stops early if ctx is cancelled.
func Process(ctx context.Context, reader io.Reader, transform Transform) (Output, error) {
	lines, err := scan(ctx, reader, transform)
	if err != nil {
		return "", err
	}
	return Output(strings.Join(lines, "\n")), nil
}

// scan reads each line, applies transform, and collects the kept results.
func scan(ctx context.Context, reader io.Reader, transform Transform) ([]string, error) {
	scanner := bufio.NewScanner(reader)
	kept := []string{}
	var number LineNumber
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		number++
		if line, ok := transform(Line(scanner.Text()), number); ok {
			kept = append(kept, string(line))
		}
	}
	return kept, scanError(scanner.Err())
}

// scanError wraps a scanner failure in the package sentinel.
func scanError(err error) error {
	if err != nil {
		return constants.ErrReadInput.With(err, "failed to read input")
	}
	return nil
}
