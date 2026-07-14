package main

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"testing"

	app "github.com/gomatic/go-app"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"

	"github.com/gomatic/template.cli/internal/constants"
)

func TestRun_Version(t *testing.T) {
	tests := []struct {
		name         string
		wantContains string
		args         []string
	}{
		{
			name:         "version flag outputs version",
			args:         []string{"template.cli", "--version"},
			wantContains: version,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want, must := assert.New(t), require.New(t)

			var stdout bytes.Buffer

			logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
				Level: slog.LevelWarn,
			}))

			// Create app and run with --version flag
			app := createApp(func(_ *cli.Command) *slog.Logger { return logger })
			app.Writer = &stdout

			err := app.Run(context.Background(), tt.args)
			must.NoError(err)

			output := stdout.String()
			want.Contains(output, tt.wantContains)
		})
	}
}

func TestCreateApp(t *testing.T) {
	tests := []struct {
		name             string
		expectedName     string
		expectedVersion  string
		expectedCommands []string
	}{
		{
			name:             "creates app with correct name and version",
			expectedName:     name,
			expectedVersion:  version,
			expectedCommands: []string{"config", "greet", "process", "rename", "serve"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want, must := assert.New(t), require.New(t)

			logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
			app := createApp(func(_ *cli.Command) *slog.Logger { return logger })

			want.Equal(tt.expectedName, app.Name)
			want.Equal(tt.expectedVersion, app.Version)
			must.NotEmpty(app.Commands, "expected app to have commands")

			// Verify expected commands exist
			for _, expected := range tt.expectedCommands {
				found := false
				for _, cmd := range app.Commands {
					if cmd.Name == expected {
						found = true
						break
					}
				}
				want.True(found, "expected command %q not found", expected)
			}
		})
	}
}

// TestIntegration_ConfigCommand demonstrates integration testing of the full CLI app.
// This shows how to test complete command execution with output capture.
func TestIntegration_ConfigCommand(t *testing.T) {
	tests := []struct {
		wantErr        error
		name           string
		wantOutputCont string
		args           []string
	}{
		{
			name:           "list all config",
			args:           []string{"template.cli", "config", "list"},
			wantOutputCont: `"app.name": "template.cli"`,
		},
		{
			name:           "list with prefix",
			args:           []string{"template.cli", "config", "list", "--prefix", "app."},
			wantOutputCont: `"app.name"`,
		},
		{
			name:           "get existing key",
			args:           []string{"template.cli", "config", "get", "app.name"},
			wantOutputCont: `"value": "template.cli"`,
		},
		{
			name:           "get with default",
			args:           []string{"template.cli", "config", "get", "--default", "fallback", "missing.key"},
			wantOutputCont: `"value": "fallback"`,
		},
		{
			name:    "get missing key without default",
			args:    []string{"template.cli", "config", "get", "nonexistent.key"},
			wantErr: constants.ErrNotFound,
		},
		{
			name: "set value (dry-run)",
			args: []string{"template.cli", "config", "set", "-n", "test.key", "test-value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want, must := assert.New(t), require.New(t)

			// Create a buffer to capture output
			var stdout bytes.Buffer

			// Create logger that doesn't pollute test output
			var logBuf bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{
				Level: slog.LevelWarn, // Only show warnings/errors in tests
			}))

			// Create and configure app
			app := createApp(func(_ *cli.Command) *slog.Logger { return logger })
			app.Writer = &stdout
			app.ErrWriter = &logBuf

			// Run command
			err := app.Run(context.Background(), tt.args)

			// Verify error expectation
			if tt.wantErr != nil {
				must.Error(err)
				want.ErrorIs(err, tt.wantErr)
				return
			}

			must.NoError(err)

			// Verify output if specified
			if tt.wantOutputCont != "" {
				want.Contains(stdout.String(), tt.wantOutputCont)
			}
		})
	}
}

func TestRun_ExitCodes(t *testing.T) {
	original := appCreator
	t.Cleanup(func() { appCreator = original })

	want := assert.New(t)

	appCreator = func(app.GetLoggerFunc) *cli.Command {
		return &cli.Command{Name: "x", Writer: &bytes.Buffer{}}
	}
	want.Equal(0, run([]string{"x"}), "successful run exits 0")

	appCreator = func(app.GetLoggerFunc) *cli.Command {
		return &cli.Command{
			Name:   "x",
			Writer: &bytes.Buffer{},
			Action: func(context.Context, *cli.Command) error { return constants.ErrInvalidValue },
		}
	}
	want.Equal(1, run([]string{"x"}), "failed run exits 1")
}

func TestMainEntry(t *testing.T) {
	originalCreator, originalExit, originalArgs := appCreator, osExit, os.Args
	t.Cleanup(func() { appCreator, osExit, os.Args = originalCreator, originalExit, originalArgs })

	var code int
	osExit = func(c int) { code = c }
	appCreator = func(app.GetLoggerFunc) *cli.Command {
		return &cli.Command{Name: "x", Writer: &bytes.Buffer{}}
	}
	os.Args = []string{"x"}

	main()
	assert.New(t).Equal(0, code)
}

func TestProductionLogger(t *testing.T) {
	assert.New(t).NotNil(productionLogger(nil))
}

// TestIntegration_GreetCommand demonstrates testing commands with various flag combinations.
func TestIntegration_GreetCommand(t *testing.T) {
	tests := []struct {
		name           string
		wantOutputCont string
		args           []string
	}{
		{
			name:           "simple greet",
			args:           []string{"template.cli", "greet", "World"},
			wantOutputCont: "Hello, World!",
		},
		{
			name:           "custom greeting",
			args:           []string{"template.cli", "greet", "--greeting", "Hola", "Alice"},
			wantOutputCont: "Hola, Alice!",
		},
		{
			name:           "uppercase",
			args:           []string{"template.cli", "greet", "--uppercase", "Bob"},
			wantOutputCont: "HELLO, BOB!",
		},
		{
			name:           "with enthusiasm",
			args:           []string{"template.cli", "greet", "--enthusiast", "World"},
			wantOutputCont: "Hello, World!!!",
		},
		{
			name:           "combined flags",
			args:           []string{"template.cli", "greet", "-g", "Hey", "-u", "-e", "Test"},
			wantOutputCont: "HEY, TEST!!!",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want, must := assert.New(t), require.New(t)

			var stdout bytes.Buffer
			var logBuf bytes.Buffer

			logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{
				Level: slog.LevelWarn,
			}))

			app := createApp(func(_ *cli.Command) *slog.Logger { return logger })
			app.Writer = &stdout

			err := app.Run(context.Background(), tt.args)
			must.NoError(err)

			want.Contains(stdout.String(), tt.wantOutputCont)
		})
	}
}
