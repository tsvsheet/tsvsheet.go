package tsvt

import (
	"github.com/antlr4-go/antlr/v4"

	"github.com/uplang/tsvsheet.go/internal/constants"
)

// errorSink holds the first collected syntax error so an errorCollector can
// record it from a value-receiver method (the sink is shared by pointer).
type errorSink struct {
	err error
}

// errorCollector records the first syntax error as a sentinel. The mutable
// error lives behind the sink pointer, so the antlr ErrorListener callback is a
// value-receiver method whose write still persists.
type errorCollector struct {
	antlr.DefaultErrorListener
	sink *errorSink
}

// SyntaxError implements antlr.ErrorListener, converting the report into
// constants.ErrSyntax; only the first error is kept.
func (c errorCollector) SyntaxError(
	_ antlr.Recognizer, _ any, line, column int, msg string, _ antlr.RecognitionException,
) {
	if c.sink.err == nil {
		c.sink.err = constants.ErrSyntax.With(nil, "line", line, "column", column, "message", msg)
	}
}
