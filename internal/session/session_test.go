package session_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uplang/tsvsheet.go/internal/constants"
	"github.com/uplang/tsvsheet.go/internal/session"
	"github.com/uplang/tsvsheet.go/internal/sheet"
)

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
	prec, deps := s.References(sheet.Address{Row: 1, Col: 3})
	require.Len(t, prec, 2)
	assert.Equal(t, sheet.Address{Row: 1, Col: 1}, prec[0].From) // B2
	assert.Empty(t, deps)                                        // nothing reads D2

	_, deps = s.References(sheet.Address{Row: 1, Col: 1})
	assert.Equal(t, []sheet.Address{{Row: 1, Col: 3}}, deps) // B2 read by D2
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
	assert.ErrorIs(t, err, constants.ErrSyntax)
}

func TestSetCell_EditsLiteralAndRecomputes(t *testing.T) {
	t.Parallel()

	s := newSession(t)
	require.NoError(t, s.SetCell(sheet.Address{Row: 1, Col: 1}, "10")) // B2 = 10
	state := s.Snapshot()
	assert.Equal(t, "10", state.Source[1][1])
	assert.Equal(t, "13", state.Computed[1][3]) // D2 = 10+3
	assert.True(t, state.IsDirty)
}

func TestSetCell_EditsFormula(t *testing.T) {
	t.Parallel()

	s := newSession(t)
	require.NoError(t, s.SetCell(sheet.Address{Row: 1, Col: 3}, "=B2*C2")) // D2 = 2*3
	assert.Equal(t, "6", s.Snapshot().Computed[1][3])
}

func TestSetCell_AtomicOnSyntaxError(t *testing.T) {
	t.Parallel()

	s := newSession(t)
	before := s.Snapshot()

	err := s.SetCell(sheet.Address{Row: 1, Col: 3}, "=sum(")
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrSyntax)

	after := s.Snapshot()
	assert.Equal(t, before.Computed, after.Computed)
	assert.Equal(t, before.Source, after.Source)
	assert.False(t, after.IsDirty) // rejected before any mutation
}

func TestSetCell_GrowsGridOnAppend(t *testing.T) {
	t.Parallel()

	s := newSession(t)
	require.NoError(t, s.SetCell(sheet.Address{Row: 3, Col: 0}, "Carol")) // one past last row
	state := s.Snapshot()
	require.Len(t, state.Source, 4)
	assert.Equal(t, "Carol", state.Source[3][0])
}

func TestDirtyLifecycle(t *testing.T) {
	t.Parallel()

	s := newSession(t)
	assert.False(t, s.Snapshot().IsDirty)

	require.NoError(t, s.SetCell(sheet.Address{Row: 0, Col: 0}, "9"))
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

	trace, err := newSession(t).Explain(sheet.Address{Row: 1, Col: 3}) // D2 = B2+C2
	require.NoError(t, err)
	assert.Equal(t, "5", trace.Value)
	assert.Equal(t, "B2 + C2", trace.Formula)
}

func TestExplain_OutOfGrid(t *testing.T) {
	t.Parallel()

	_, err := newSession(t).Explain(sheet.Address{Row: 99, Col: 0})
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrNotFound)
}

func TestConcurrentAccess(t *testing.T) {
	t.Parallel()

	s := newSession(t)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.SetCell(sheet.Address{Row: 0, Col: 0}, "x")
			_ = s.Snapshot()
			_ = s.Source()
			s.MarkSaved()
		}()
	}
	wg.Wait()
}
