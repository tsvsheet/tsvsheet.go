package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRun_Version(t *testing.T) {
	assert.Equal(t, 0, run([]string{"tsv", "--version"}))
}

func TestMain_InvokesRunAndExits(t *testing.T) {
	oldArgs, oldExit := os.Args, osExit
	defer func() { os.Args, osExit = oldArgs, oldExit }()

	os.Args = []string{"tsv", "--version"}
	code := -1
	osExit = func(c int) { code = c }

	main()
	assert.Equal(t, 0, code)
}
