package rename

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	app "github.com/gomatic/go-app"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"

	domain "github.com/gomatic/template.cli/internal/domain/rename"
)

// TestRenameCommand exercises the CLI wiring without running the real rename: it
// stubs runAction so flags and positional arguments reach the domain unchanged.
func TestRenameCommand(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantArgs   []string
		wantDryRun bool
	}{
		{name: "no argument", args: []string{"rename"}, wantArgs: nil},
		{name: "name argument", args: []string{"rename", "mytool"}, wantArgs: []string{"mytool"}},
		{name: "dry-run flag", args: []string{"rename", "--dry-run"}, wantDryRun: true},
		{
			name:       "short dry-run flag",
			args:       []string{"rename", "-n", "mytool"},
			wantArgs:   []string{"mytool"},
			wantDryRun: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want, must := assert.New(t), require.New(t)

			var gotArgs []string
			var gotDryRun bool

			originalAction := runAction
			t.Cleanup(func() { runAction = originalAction })
			runAction = func(_ context.Context, _ *slog.Logger, cfg domain.Config, args ...string) (domain.Result, error) {
				gotArgs = args
				gotDryRun = bool(cfg.DryRunEnabled)
				return domain.Result{ToName: "stub"}, nil
			}
			cfg.DryRunEnabled = false

			var buf bytes.Buffer
			testApp := &cli.Command{
				Name:     "test",
				Commands: []*cli.Command{Command()},
				Writer:   &buf,
			}
			logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelWarn}))
			testApp.Metadata = map[string]any{app.LoggerMetadataKey: logger}

			err := testApp.Run(context.Background(), append([]string{"test"}, tt.args...))

			must.NoError(err)
			want.ElementsMatch(tt.wantArgs, gotArgs)
			want.Equal(tt.wantDryRun, gotDryRun)
		})
	}
}
