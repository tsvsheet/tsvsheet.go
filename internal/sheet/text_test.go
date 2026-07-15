package sheet_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/uplang/tsvsheet.go/internal/sheet"
)

func TestText_HappyPaths(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		`upper("hi")`:                        "HI",
		`lower("HI")`:                        "hi",
		`proper("john q. doe")`:              "John Q. Doe",
		`trim("  a   b  ")`:                  "a b",
		`clean(concat("a", char(7), "b"))`:   "ab",
		`left("hello", 3)`:                   "hel",
		`left("hello")`:                      "h",  // default length 1
		`left("hi", 9)`:                      "hi", // clamp
		`right("hello", 2)`:                  "lo",
		`mid("hello", 2, 3)`:                 "ell",
		`mid("hi", 1, 9)`:                    "hi", // clamp
		`rept("ab", 3)`:                      "ababab",
		`exact("A", "A")`:                    "TRUE",
		`exact("A", "a")`:                    "FALSE",
		`t("hi")`:                            "hi",
		`t(5)`:                               "",
		`concatenate("a", "b", "c")`:         "abc",
		`find("l", "hello")`:                 "3",
		`find("l", "hello", 4)`:              "4",
		`search("L", "hello")`:               "3", // case-insensitive
		`substitute("a-b-c", "-", "+")`:      "a+b+c",
		`substitute("a-b-a", "a", "X", 2)`:   "a-b-X",
		`replace("hello", 2, 3, "XY")`:       "hXYo",
		`char(65)`:                           "A",
		`code("ABC")`:                        "65",
		`unichar(97)`:                        "a",
		`unicode("z")`:                       "122",
		`value("  42 ")`:                     "42",
		`regexmatch("abc123", "[0-9]+")`:     "TRUE",
		`regexextract("abc123", "[0-9]+")`:   "123",
		`regexreplace("a1b2", "[0-9]", "X")`: "aXbX",
	}
	for expr, want := range cases {
		t.Run(expr, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, want, formula1(t, expr))
		})
	}
}

func TestText_ErrorsAndEdges(t *testing.T) {
	t.Parallel()

	v := string(sheet.ErrValue)
	cases := map[string]string{
		`find("x", "hello")`:            v, // not found
		`find("l", "hi", 5)`:            v, // start past end
		`find("l", "hello", 0)`:         v, // start below 1
		`left("hi", -1)`:                v, // negative count
		`mid("hi", 0, 1)`:               v, // start below 1
		`mid("hi", 1, -1)`:              v, // negative length
		`rept("a", -1)`:                 v, // negative count
		`replace("hi", 0, 1, "x")`:      v,
		`char(0)`:                       v,                   // code below 1
		`code("")`:                      v,                   // empty text
		`value("x")`:                    v,                   // non-numeric
		`regexmatch("x", "[")`:          v,                   // invalid pattern
		`regexextract("x", "[")`:        v,                   // invalid pattern
		`regexreplace("x", "[", "y")`:   v,                   // invalid pattern
		`regexextract("abc", "[0-9]+")`: string(sheet.ErrNA), // no match
		`regexextract("abc", "z*")`:     "",                  // matches empty, not #N/A
	}
	for expr, want := range cases {
		t.Run(expr, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, want, formula1(t, expr))
		})
	}
}

func TestText_NonNumericCountArgs(t *testing.T) {
	t.Parallel()

	// A number argument that is text propagates #VALUE! from each site.
	v := "#VALUE!"
	cases := []string{
		`=left("hi", A1)`, `=mid("hi", A1, 1)`, `=mid("hi", 1, A1)`, `=rept("a", A1)`,
		`=find("a", "b", A1)`, `=replace("a", A1, 1, "b")`, `=replace("a", 1, A1, "b")`,
		`=char(A1)`, `=substitute("a", "b", "c", A1)`,
	}
	for _, expr := range cases {
		t.Run(expr, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, v, cellAt(t, compute(t, "hi\t"+expr+"\n"), 0, 1))
		})
	}
}

func TestText_SubstituteInstances(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "abc", formula1(t, `substitute("abc", "z", "y")`))    // not found
	assert.Equal(t, "abc", formula1(t, `substitute("abc", "b", "X", 0)`)) // instance below 1
	assert.Equal(t, "abc", formula1(t, `substitute("abc", "", "X", 1)`))  // empty search
	assert.Equal(t, "aXc", formula1(t, `substitute("abc", "b", "X", 1)`)) // first instance
	assert.Equal(t, "a-b", formula1(t, `substitute("a-b", "z", "y", 5)`)) // nth not found
}
