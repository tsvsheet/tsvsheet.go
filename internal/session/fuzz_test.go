package session_test

import (
	"bytes"
	"testing"

	tsvsheet "github.com/tsvsheet/go-tsvsheet"

	"github.com/tsvsheet/tsvsheet.go/internal/session"
)

// FuzzNew asserts the session constructor's contract over arbitrary untrusted
// input: a source that parses builds a session whose Source (what Ctrl+S
// writes back) is the engine's canonical form — byte-identical to
// ParseDocument(src).Text() — and whose eager compute completed without
// panicking. Session-level budgets ride the engine's BrowserLimits so a
// fuzz-found pathological formula exercises the refusal paths rather than the
// machine.
func FuzzNew(f *testing.F) {
	f.Add([]byte("a\tb\n=A1+1\t=sum(A1:B1)\n"))
	f.Add([]byte("#. hidden\tB\n=sum(A1:DEJTLX1)\t#LIMIT!\n"))
	f.Fuzz(func(t *testing.T, src []byte) {
		s, err := session.NewEmbeddable(src, nil, "", tsvsheet.BrowserLimits(), nil)
		if err != nil {
			return
		}
		doc, err := tsvsheet.ParseDocument(src)
		if err != nil {
			t.Fatalf("session accepted what the engine refuses: %v", err)
		}
		if !bytes.Equal(s.Source(), doc.Text()) {
			t.Fatal("a fresh session's saved text diverged from the engine's canonical form")
		}
	})
}
