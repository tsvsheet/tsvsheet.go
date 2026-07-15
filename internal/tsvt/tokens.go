package tsvt

import (
	"strconv"

	"github.com/antlr4-go/antlr/v4"

	"github.com/uplang/tsvsheet.go/internal/constants"
)

// quoted is a double-quoted STRING token's raw text (quotes included).
type quoted string

// unquote strips the enclosing quotes the STRING token guarantees.
func unquote(s quoted) string { return string(s[1 : len(s)-1]) }

// intToken parses a NUMBER terminal as an integer, failing with ErrSyntax on a
// fractional value (an A1 row is a whole number).
func intToken(node antlr.TerminalNode) (int, error) {
	n, err := strconv.Atoi(node.GetText())
	if err != nil {
		sym := node.GetSymbol()
		return 0, constants.ErrSyntax.With(err,
			"line", sym.GetLine(), "column", sym.GetColumn(), "message", "expected an integer row")
	}
	return n, nil
}
