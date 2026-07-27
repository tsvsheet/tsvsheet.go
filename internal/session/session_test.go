package session_test

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsvsheet/go-tsvsheet"

	"github.com/tsvsheet/tsvsheet.go/internal/importer"
	"github.com/tsvsheet/tsvsheet.go/internal/session"
)

// countingFetcher is a fake tsvsheet.Fetcher that tallies its calls and returns a
// fixed single-cell import body, so a test can observe whether a recompute
// actually re-fetches.
type countingFetcher struct {
	calls *int32
}

func (f countingFetcher) Fetch(_ tsvsheet.ImportURL, accept tsvsheet.MediaType) (tsvsheet.FetchResult, error) {
	atomic.AddInt32(f.calls, 1)
	return tsvsheet.FetchResult{ContentType: accept, Body: []byte("42\n")}, nil
}

func TestRefreshImports_ClearsCacheAndRefetches(t *testing.T) {
	t.Parallel()

	var calls int32
	cache := importer.NewCache(countingFetcher{calls: &calls})
	s, err := session.NewEmbeddable(
		[]byte(`=importcell("https://x/a")`+"\n"), nil, "", tsvsheet.DefaultLimits(), cache,
	)
	require.NoError(t, err)
	s.OnRefresh(cache.Clear)

	// The eager initial compute fetched once.
	assert.Equal(t, "42", s.Snapshot().Computed[0][0])
	assert.EqualValues(t, 1, atomic.LoadInt32(&calls))

	// A plain recompute reuses the cross-pass cache — no new fetch.
	s.Recompute()
	assert.EqualValues(t, 1, atomic.LoadInt32(&calls))

	// RefreshImports clears the cache first, so the recompute re-fetches.
	st := s.RefreshImports()
	assert.Equal(t, "42", st.Computed[0][0])
	assert.EqualValues(t, 2, atomic.LoadInt32(&calls))
}

func TestRefreshImports_NoClearIsPlainRecompute(t *testing.T) {
	t.Parallel()

	// With no clear registered and no imports, RefreshImports is a safe recompute
	// that does not dirty the session.
	s := newSession(t)
	st := s.RefreshImports()
	assert.Equal(t, "5", st.Computed[1][3])
	assert.False(t, st.IsDirty)
}

// sampleSheet is a small spreadsheet: three data columns and a formula in D
// that sums B and C for each row.
var sampleSheet = []byte(
	"name\tb\tc\ttotal\n" +
		"Alice\t2\t3\t=B2+C2\n" +
		"Bob\t4\t5\t=B3+C3\n",
)

func newSession(t *testing.T) *session.Session {
	t.Helper()
	s, err := session.New(sampleSheet)
	require.NoError(t, err)
	return s
}

func TestReferences_PrecedentsAndDependents(t *testing.T) {
	t.Parallel()

	// B2 is read by D2 (=B2+C2); D2 reads B2 and C2.
	s := newSession(t)
	prec, deps := s.References(tsvsheet.Address{Row: 1, Col: 3})
	require.Len(t, prec, 2)
	assert.Equal(t, tsvsheet.Address{Row: 1, Col: 1}, prec[0].From) // B2
	assert.Empty(t, deps)                                           // nothing reads D2

	_, deps = s.References(tsvsheet.Address{Row: 1, Col: 1})
	assert.Equal(t, []tsvsheet.Address{{Row: 1, Col: 3}}, deps) // B2 read by D2
}

func TestNewEmbeddable_ZeroLimitsUseDefault(t *testing.T) {
	t.Parallel()

	// A zero (unset) Limits falls back to DefaultLimits, so an edit within the
	// generous default grid dimension succeeds — a degenerate zero cap would
	// reject every edit.
	s, err := session.NewEmbeddable([]byte("1\n"), nil, "", tsvsheet.Limits{}, nil)
	require.NoError(t, err)
	require.NoError(t, s.SetCell(tsvsheet.Address{Row: 3, Col: 0}, "x"))
}

func TestNewEmbeddable_HonorsInjectedLimits(t *testing.T) {
	t.Parallel()

	// A non-zero Limits is threaded into the session's edit path: an address
	// beyond the injected grid dimension is rejected.
	s, err := session.NewEmbeddable(
		[]byte("1\n"),
		nil,
		"",
		tsvsheet.Limits{ResultCells: 5, GridDim: 5, ResultBytes: 5},
		nil,
	)
	require.NoError(t, err)
	err = s.SetCell(tsvsheet.Address{Row: 5, Col: 0}, "x")
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrInvalidValue)
}

func TestNewEmbeddable_ResolvesSheetOutput(t *testing.T) {
	t.Parallel()

	loader := func(_, ref tsvsheet.Path) (tsvsheet.Sheet, tsvsheet.Path, error) {
		s, err := tsvsheet.Parse([]byte("=output(7)\n"))
		return s, ref, err
	}
	s, err := session.NewEmbeddable([]byte("=sheet(\"child\")\n"), loader, "root", tsvsheet.DefaultLimits(), nil)
	require.NoError(t, err)
	assert.Equal(t, "7", s.Snapshot().Computed[0][0])
}

func TestEmbedded_ReturnsSubSheetOrNotOK(t *testing.T) {
	t.Parallel()

	loader := func(_, ref tsvsheet.Path) (tsvsheet.Sheet, tsvsheet.Path, error) {
		s, err := tsvsheet.Parse([]byte("=output(9)\n"))
		return s, ref, err
	}
	s, err := session.NewEmbeddable([]byte("=sheet(\"c\")\n"), loader, "root", tsvsheet.DefaultLimits(), nil)
	require.NoError(t, err)

	path, grid, ok := s.Embedded(tsvsheet.Address{Row: 0, Col: 0})
	require.True(t, ok)
	assert.Equal(t, tsvsheet.Path("c"), path)
	assert.Equal(t, "9", grid[0][0])

	// A non-embed session returns ok=false.
	_, _, ok = newSession(t).Embedded(tsvsheet.Address{Row: 0, Col: 0})
	assert.False(t, ok)
}

func TestIsVolatileAndRecompute(t *testing.T) {
	t.Parallel()

	// sampleSheet has no clock functions.
	assert.False(t, newSession(t).IsVolatile())
	v, err := session.New([]byte("=volatile(now())\n"))
	require.NoError(t, err)
	assert.True(t, v.IsVolatile())

	// Recompute refreshes the read model without dirtying it.
	state := newSession(t).Recompute()
	assert.Equal(t, "5", state.Computed[1][3])
	assert.False(t, state.IsDirty)
}

func TestInsertRow_GrowsAndDirties(t *testing.T) {
	t.Parallel()

	s := newSession(t)
	s.InsertRow(tsvsheet.Address{Row: 1})
	st := s.Snapshot()
	assert.Len(t, st.Source, 4) // 3 rows → 4
	assert.True(t, st.IsDirty)
}

func TestDeleteRow_Shrinks(t *testing.T) {
	t.Parallel()

	s := newSession(t)
	s.DeleteRow(tsvsheet.Address{Row: 1})
	assert.Len(t, s.Snapshot().Source, 2)
}

func TestInsertCol_Widens(t *testing.T) {
	t.Parallel()

	s := newSession(t)
	s.InsertCol(tsvsheet.Address{Col: 1})
	assert.Len(t, s.Snapshot().Source[0], 5) // 4 cols → 5
}

func TestDeleteCol_Narrows(t *testing.T) {
	t.Parallel()

	s := newSession(t)
	s.DeleteCol(tsvsheet.Address{Col: 1})
	assert.Len(t, s.Snapshot().Source[0], 3)
}

func TestNew_ComputesEagerly(t *testing.T) {
	t.Parallel()

	state := newSession(t).Snapshot()
	assert.Equal(t, "5", state.Computed[1][3]) // D2 = B2+C2 = 2+3
	assert.Equal(t, "9", state.Computed[2][3]) // D3 = B3+C3 = 4+5
	assert.Equal(t, "=B2+C2", state.Source[1][3])
	assert.False(t, state.IsDirty)
	assert.Empty(t, state.Diagnostics)
}

func TestNew_SyntaxError(t *testing.T) {
	t.Parallel()

	_, err := session.New([]byte("1\t=sum(\n"))
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrSyntax)
}

func TestSetCell_EditsLiteralAndRecomputes(t *testing.T) {
	t.Parallel()

	s := newSession(t)
	require.NoError(t, s.SetCell(tsvsheet.Address{Row: 1, Col: 1}, "10")) // B2 = 10
	state := s.Snapshot()
	assert.Equal(t, "10", state.Source[1][1])
	assert.Equal(t, "13", state.Computed[1][3]) // D2 = 10+3
	assert.True(t, state.IsDirty)
}

func TestSetCell_EditsFormula(t *testing.T) {
	t.Parallel()

	s := newSession(t)
	require.NoError(t, s.SetCell(tsvsheet.Address{Row: 1, Col: 3}, "=B2*C2")) // D2 = 2*3
	assert.Equal(t, "6", s.Snapshot().Computed[1][3])
}

func TestSetCell_AtomicOnSyntaxError(t *testing.T) {
	t.Parallel()

	s := newSession(t)
	before := s.Snapshot()

	err := s.SetCell(tsvsheet.Address{Row: 1, Col: 3}, "=sum(")
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrSyntax)

	after := s.Snapshot()
	assert.Equal(t, before.Computed, after.Computed)
	assert.Equal(t, before.Source, after.Source)
	assert.False(t, after.IsDirty) // rejected before any mutation
}

func TestSetCell_GrowsGridOnAppend(t *testing.T) {
	t.Parallel()

	s := newSession(t)
	require.NoError(t, s.SetCell(tsvsheet.Address{Row: 3, Col: 0}, "Carol")) // one past last row
	state := s.Snapshot()
	require.Len(t, state.Source, 4)
	assert.Equal(t, "Carol", state.Source[3][0])
}

func TestDirtyLifecycle(t *testing.T) {
	t.Parallel()

	s := newSession(t)
	assert.False(t, s.Snapshot().IsDirty)

	require.NoError(t, s.SetCell(tsvsheet.Address{Row: 0, Col: 0}, "9"))
	assert.True(t, s.Snapshot().IsDirty)

	s.MarkSaved()
	assert.False(t, s.Snapshot().IsDirty)
}

func TestSource_EncodesTSV(t *testing.T) {
	t.Parallel()

	assert.Equal(t, string(sampleSheet), string(newSession(t).Source()))
}

func TestSnapshot_IsIsolatedCopy(t *testing.T) {
	t.Parallel()

	s := newSession(t)
	state := s.Snapshot()
	state.Computed[0][0] = "mutated"                     // mutate the snapshot
	assert.Equal(t, "name", s.Snapshot().Computed[0][0]) // session unaffected
}

func TestExplain(t *testing.T) {
	t.Parallel()

	trace, err := newSession(t).Explain(tsvsheet.Address{Row: 1, Col: 3}) // D2 = B2+C2
	require.NoError(t, err)
	assert.Equal(t, "5", trace.Value)
	assert.Equal(t, "B2 + C2", trace.Formula)
}

func TestExplain_OutOfGrid(t *testing.T) {
	t.Parallel()

	_, err := newSession(t).Explain(tsvsheet.Address{Row: 99, Col: 0})
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrNotFound)
}

func TestConcurrentAccess(t *testing.T) {
	t.Parallel()

	s := newSession(t)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.SetCell(tsvsheet.Address{Row: 0, Col: 0}, "x")
			_ = s.Snapshot()
			_ = s.Source()
			s.MarkSaved()
		}()
	}
	wg.Wait()
}

func TestVolatileSchedules(t *testing.T) {
	t.Parallel()

	assert.Empty(t, newSession(t).VolatileSchedules())

	v, err := session.New([]byte("=volatile(rand(), \"5m\")\n"))
	require.NoError(t, err)
	assert.Equal(t, []string{"5m"}, v.VolatileSchedules())
}

func TestTickAdvancesEachRecompute(t *testing.T) {
	t.Parallel()

	// tick() reads the pass ordinal, which advances on every recompute so a
	// refreshing frontend can drive frame-based animation.
	s, err := session.New([]byte("=tick()\n"))
	require.NoError(t, err)
	assert.Equal(t, "0", s.Snapshot().Computed[0][0])  // first pass
	assert.Equal(t, "1", s.Recompute().Computed[0][0]) // next pass
	assert.Equal(t, "2", s.Recompute().Computed[0][0])
}

func TestFill_CopiesWithRebasedReferences(t *testing.T) {
	t.Parallel()

	// Fill D2 (=B2+C2) into D3: the copy rebases to that row's cells.
	s := newSession(t)
	d3 := tsvsheet.Address{Row: 2, Col: 3}
	s.Fill(tsvsheet.Address{Row: 1, Col: 3}, tsvsheet.Span{From: d3, To: d3})
	st := s.Snapshot()
	assert.Equal(t, "=B3 + C3", st.Source[2][3])
	assert.Equal(t, "9", st.Computed[2][3]) // 4 + 5
	assert.True(t, st.IsDirty)
}

func TestDuplicateRow_AddsRebasedRow(t *testing.T) {
	t.Parallel()

	// Duplicating row 2 rebases the duplicate's formula and shifts the row
	// below it, exactly as the engine's insert does.
	s := newSession(t)
	s.DuplicateRow(tsvsheet.Address{Row: 1})
	st := s.Snapshot()
	assert.Len(t, st.Source, 4)
	assert.Equal(t, "=B3 + C3", st.Source[2][3]) // the duplicate
	assert.Equal(t, "=B4 + C4", st.Source[3][3]) // the shifted original
	assert.Equal(t, "5", st.Computed[2][3])
	assert.True(t, st.IsDirty)
}

func TestDuplicateCol_AddsRebasedColumn(t *testing.T) {
	t.Parallel()

	// Duplicating column B: the duplicate carries B's literals, and the D
	// formulas' C references follow their shifted data.
	s := newSession(t)
	s.DuplicateCol(tsvsheet.Address{Col: 1})
	st := s.Snapshot()
	assert.Len(t, st.Source[0], 5)
	assert.Equal(t, "2", st.Source[1][2])        // duplicated literal
	assert.Equal(t, "=B2 + D2", st.Source[1][4]) // C followed its data to D
	assert.Equal(t, "5", st.Computed[1][4])
	assert.True(t, st.IsDirty)
}

// TestSessionPreservesTheFileThroughEdits is the defect this refactor closes:
// a session used to rebuild its source from the grid, so saving from serve or
// the TUI silently deleted every comment and shebang line — and would have
// deleted every view directive too. The document layer keeps them, in place.
func TestSessionPreservesTheFileThroughEdits(t *testing.T) {
	t.Parallel()

	const src = "#!/usr/bin/env tsvsheet\n" +
		"#.a note that must survive\n" +
		"#.header\trows(count(1))\n" +
		"#.hide\tcols(range(C:C))\n" +
		"name\tqty\tscratch\nwidget\t3\tx\n"

	s, err := session.New([]byte(src))
	require.NoError(t, err)

	// Untouched, the file comes back byte-identical.
	assert.Equal(t, src, string(s.Source()))

	// After an edit, every non-grid line is still there and in position.
	require.NoError(t, s.SetCell(tsvsheet.Address{Row: 1, Col: 1}, "4"))
	saved := string(s.Source())
	assert.Contains(t, saved, "#!/usr/bin/env tsvsheet\n")
	assert.Contains(t, saved, "#.a note that must survive\n")
	assert.Contains(t, saved, "#.header\trows(count(1))\n")
	assert.Contains(t, saved, "widget\t4\tx\n")
}

// TestSessionShiftsDirectivesWithTheGrid proves a structural edit keeps the
// declarations true: a row inserted at the top widens the header block that
// took it in, and the column directive on the other axis is untouched.
func TestSessionShiftsDirectivesWithTheGrid(t *testing.T) {
	t.Parallel()

	s, err := session.New([]byte(
		"#.header\trows(count(1))\n#.hide\tcols(range(C:C))\nname\tqty\tscratch\nwidget\t3\tx\n"))
	require.NoError(t, err)

	s.InsertRow(tsvsheet.Address{Row: 0, Col: 0})
	saved := string(s.Source())
	assert.Contains(t, saved, "#.header\trows(count(2))")
	assert.Contains(t, saved, "#.hide\tcols(range(C:C))")
}

// TestSessionStateCarriesTheView proves a frontend reads the declared view from
// the same snapshot it reads the grid from, rather than deriving it.
func TestSessionStateCarriesTheView(t *testing.T) {
	t.Parallel()

	s, err := session.New([]byte(
		"#.header\trows(count(1))\n#.freeze\trows(count(1), count(-1))\nname\tqty\nwidget\t3\ntotal\t3\n"))
	require.NoError(t, err)

	state := s.Snapshot()
	assert.Equal(t, tsvsheet.Selection{1: true}, state.View.HeaderRows)
	assert.Equal(t, tsvsheet.Selection{1: true, 3: true}, state.View.FreezeRows)
	assert.Empty(t, state.Diagnostics)
}

// TestSessionReportsDirectiveDiagnostics proves a malformed directive reaches a
// frontend on the same list as an unknown function, so an editor surfaces both
// the same way.
func TestSessionReportsDirectiveDiagnostics(t *testing.T) {
	t.Parallel()

	s, err := session.New([]byte("#.hide\trows(3)\nname\t=BADFN(1)\n"))
	require.NoError(t, err)

	diags := s.Snapshot().Diagnostics
	require.Len(t, diags, 2)
	assert.Equal(t, 1, diags[0].Line, "the directive finding is addressed by its line")
	assert.Contains(t, diags[0].Message, "range(3:3)")
	assert.Equal(t, "B1", diags[1].Cell)
}
