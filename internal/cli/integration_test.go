//go:build integration

// The 018 integration matrix: every FILE-loading command driven through the
// REAL command tree (flag parsing, limits threading, bounded parse) against
// in-budget and over-budget documents, with the budget raised and lowered —
// budgets are policy, so the matrix proves the policy end to end with tiny
// files and tiny --max-cells rather than gigabyte fixtures.
package cli

import (
	"io"
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsvsheet/go-tsvsheet"

	"github.com/tsvsheet/tsvsheet.go/internal/tui"
)

// sixCells is the matrix fixture: 6 cells, one formula, over a --max-cells 2
// budget and under a --max-cells 100 one.
const sixCells = "1\t2\n3\t=A1*7\n5\t6\n"

// overBudgetArgs prefixes args with a two-cell budget.
func overBudgetArgs(args ...string) []string { return append([]string{"--max-cells", "2"}, args...) }

// raisedArgs prefixes args with a budget the fixture fits.
func raisedArgs(args ...string) []string { return append([]string{"--max-cells", "100"}, args...) }

// TestIntegration_EveryLoadingCommandHonorsTheBudget drives each one-shot
// command through the full CLI: under a two-cell budget the six-cell fixture
// refuses as ErrDocTooLarge naming the flag; under a raised budget the very
// same invocation succeeds with computed output where the command computes.
func TestIntegration_EveryLoadingCommandHonorsTheBudget(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args func(path string) []string
		want string // a fragment the raised-budget run must print
	}{
		{"render", func(p string) []string { return []string{"render", p} }, "7"},
		{"parse", func(p string) []string { return []string{"parse", p} }, "=A1*7"},
		{"explain", func(p string) []string { return []string{"explain", "B2", p} }, "7"},
		{"check", func(p string) []string { return []string{"check", p} }, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := writeTemp(t, "six.tsvt", sixCells)

			_, err := runCLI(t, overBudgetArgs(tc.args(path)...)...)
			require.ErrorIs(t, err, tsvsheet.ErrDocTooLarge, "over budget must refuse")
			assert.Contains(t, err.Error(), "--max-cells", "the refusal names the remedy")

			out, err := runCLI(t, raisedArgs(tc.args(path)...)...)
			require.NoError(t, err, "the same file under a raised budget succeeds")
			assert.Contains(t, out, tc.want)
		})
	}
}

// TestIntegration_ApplyHonorsTheBudget pins the edit pipeline's load: an
// edits batch against an over-budget sheet refuses before anything applies,
// and applies cleanly under the raised budget.
func TestIntegration_ApplyHonorsTheBudget(t *testing.T) {
	t.Parallel()

	sheet := writeTemp(t, "six.tsvt", sixCells)
	edits := writeTemp(t, "batch.tsv", "setCell\tA1\t9\n")

	_, err := runCLI(t, overBudgetArgs("apply", sheet, edits)...)
	require.ErrorIs(t, err, tsvsheet.ErrDocTooLarge)
	unchanged, err := os.ReadFile(sheet)
	require.NoError(t, err)
	assert.Equal(t, sixCells, string(unchanged), "a refused apply must not touch the file")

	_, err = runCLI(t, raisedArgs("apply", sheet, edits)...)
	require.NoError(t, err)
	applied, err := os.ReadFile(sheet)
	require.NoError(t, err)
	assert.Contains(t, string(applied), "9\t2", "the edit landed on disk")
	assert.Contains(t, string(applied), "=A1*7", "the untouched formula survives the write-back")
}

// TestIntegration_StdinIsBoundedToo pins the pipe angle: a document streamed
// through stdin obeys the same budget as a file path.
func TestIntegration_StdinIsBoundedToo(t *testing.T) {
	prev := stdin
	t.Cleanup(func() { stdin = prev })
	stdin = strings.NewReader(sixCells)

	_, err := runCLI(t, overBudgetArgs("render", "-")...)
	require.ErrorIs(t, err, tsvsheet.ErrDocTooLarge)

	stdin = strings.NewReader(sixCells)
	out, err := runCLI(t, raisedArgs("render", "-")...)
	require.NoError(t, err)
	assert.Contains(t, out, "7")
}

// TestIntegration_TUIRoutesByBudgetThroughTheFullCLI pins the tui angle at
// the command layer: the same file pages view-only under a tight budget and
// opens the editor under a raised one — decided by nothing but --max-cells,
// through the full flag-parsing path.
func TestIntegration_TUIRoutesByBudgetThroughTheFullCLI(t *testing.T) {
	var gotModel tea.Model
	withRunProgram(t, func(m tea.Model, _ io.Reader, _ io.Writer) error {
		gotModel = m
		return nil
	})
	path := writeTemp(t, "six.tsvt", sixCells)

	_, err := runCLI(t, overBudgetArgs("tui", path)...)
	require.NoError(t, err)
	_, isPager := gotModel.(tui.Pager)
	assert.True(t, isPager, "a tight budget pages, got %T", gotModel)

	_, err = runCLI(t, raisedArgs("tui", path)...)
	require.NoError(t, err)
	_, isPager = gotModel.(tui.Pager)
	assert.False(t, isPager, "a raised budget edits, got %T", gotModel)
}

// TestIntegration_SiblingReferencesHonorTheBudget pins the adversary's HIGH
// finding closed: a one-cell sheet referencing an over-budget sibling refuses
// exactly as the direct load does — the side door pays the same toll — and
// the same reference under a raised budget computes.
func TestIntegration_SiblingReferencesHonorTheBudget(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(dir+"/six.tsvt", []byte(sixCells), 0o600))
	require.NoError(t, os.WriteFile(dir+"/ref.tsvt", []byte("=\"six.tsvt\"!B2\n"), 0o600))

	out, err := runCLI(t, overBudgetArgs("render", dir+"/ref.tsvt")...)
	require.NoError(t, err, "the referencing sheet itself is one cell — in budget")
	assert.Contains(t, out, "#REF!", "the over-budget sibling refuses as a reference error, never a value")

	out, err = runCLI(t, raisedArgs("render", dir+"/ref.tsvt")...)
	require.NoError(t, err)
	assert.Contains(t, out, "7", "the same reference under a raised budget computes")
}

// TestIntegration_ApplyEditsBatchIsBounded pins the edits positional: a batch
// with more lines than the budget refuses before any op parses; the sheet is
// untouched.
func TestIntegration_ApplyEditsBatchIsBounded(t *testing.T) {
	t.Parallel()

	sheet := writeTemp(t, "one.tsvt", "1\n") // IN budget: only the batch can refuse
	edits := writeTemp(t, "batch.tsv", strings.Repeat("setCell\tA1\t9\n", 3))

	_, err := runCLI(t, overBudgetArgs("apply", sheet, edits)...)
	require.ErrorIs(t, err, tsvsheet.ErrDocTooLarge, "the batch, not the sheet, must refuse")
	unchanged, err := os.ReadFile(sheet)
	require.NoError(t, err)
	assert.Equal(t, "1\n", string(unchanged))

	_, err = runCLI(t, raisedArgs("apply", sheet, edits)...)
	require.NoError(t, err, "the same batch under a raised budget applies")
	applied, err := os.ReadFile(sheet)
	require.NoError(t, err)
	assert.Equal(t, "9\n", string(applied))
}

// TestIntegration_FromJSONIsBounded pins the JSON round trip's budget: a grid
// over --max-cells refuses with the hint; in budget it round-trips.
func TestIntegration_FromJSONIsBounded(t *testing.T) {
	t.Parallel()

	big := writeTemp(t, "big.json", `{"rows":[["a","b"],["c","d"],["e","f"]]}`)
	_, err := runCLI(t, overBudgetArgs("from-json", big)...)
	require.ErrorIs(t, err, tsvsheet.ErrDocTooLarge)
	assert.Contains(t, err.Error(), "--max-cells")

	out, err := runCLI(t, raisedArgs("from-json", big)...)
	require.NoError(t, err)
	assert.Equal(t, "a\tb\nc\td\ne\tf\n", out)
}
