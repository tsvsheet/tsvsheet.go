package session_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsvsheet/go-tsvsheet"

	"github.com/tsvsheet/tsvsheet.go/internal/session"
)

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

func TestSessionShiftsDirectivesWithTheGrid(t *testing.T) {
	t.Parallel()

	s, err := session.New([]byte(
		"#.header\trows(count(1))\n#.hide\tcols(range(C:C))\nname\tqty\tscratch\nwidget\t3\tx\n",
	))
	require.NoError(t, err)

	s.InsertRow(tsvsheet.Address{Row: 0, Col: 0})
	saved := string(s.Source())
	assert.Contains(t, saved, "#.header\trows(count(2))")
	assert.Contains(t, saved, "#.hide\tcols(range(C:C))")
}

func TestSessionStateCarriesTheView(t *testing.T) {
	t.Parallel()

	s, err := session.New([]byte(
		"#.header\trows(count(1))\n#.freeze\trows(count(1), count(-1))\nname\tqty\nwidget\t3\ntotal\t3\n",
	))
	require.NoError(t, err)

	state := s.Snapshot()
	assert.Equal(t, tsvsheet.Selection{1: true}, state.View.HeaderRows)
	assert.Equal(t, tsvsheet.Selection{1: true, 3: true}, state.View.FreezeRows)
	assert.Empty(t, state.Diagnostics)
}

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

func TestSnapshot_IsIsolatedCopy(t *testing.T) {
	t.Parallel()

	s := newSession(t)
	state := s.Snapshot()
	state.Computed[0][0] = "mutated"                     // mutate the snapshot
	assert.Equal(t, "name", s.Snapshot().Computed[0][0]) // session unaffected
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

// TestSessionPreservesTheFileThroughEdits is the defect this refactor closes:
// a session used to rebuild its source from the grid, so saving from serve or
// the TUI silently deleted every comment and shebang line — and would have
// deleted every view directive too. The document layer keeps them, in place.
// TestSessionShiftsDirectivesWithTheGrid proves a structural edit keeps the
// declarations true: a row inserted at the top widens the header block that
// took it in, and the column directive on the other axis is untouched.
// TestSessionStateCarriesTheView proves a frontend reads the declared view from
// the same snapshot it reads the grid from, rather than deriving it.
// TestSessionReportsDirectiveDiagnostics proves a malformed directive reaches a
// frontend on the same list as an unknown function, so an editor surfaces both
// the same way.
