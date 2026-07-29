package cli

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"

	"github.com/tsvsheet/tsvsheet.go/internal/constants"
	"github.com/tsvsheet/tsvsheet.go/internal/dataserve"
	"github.com/tsvsheet/tsvsheet.go/internal/importer"
)

// dataDir writes a published directory and returns its path.
func dataDir(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "balances.tsv"), []byte("Brokerage\t310000\n"), 0o600))
	return dir
}

// runResolveData drives resolveData through a real flag parse.
func runResolveData(t *testing.T, args ...string) (importer.DataBase, error) {
	t.Helper()

	var (
		base   importer.DataBase
		gotErr error
	)
	cmd := &cli.Command{
		Name:  "x",
		Flags: dataFlags(),
		Action: func(_ context.Context, c *cli.Command) error {
			var closeData dataCloser
			base, closeData, gotErr = resolveData(c)
			if closeData != nil {
				_ = closeData()
			}
			return nil
		},
	}
	require.NoError(t, cmd.Run(context.Background(), append([]string{"x"}, args...)))
	return base, gotErr
}

func TestResolveData_AbsentLeavesNoBase(t *testing.T) {
	t.Parallel()

	base, err := runResolveData(t)
	require.NoError(t, err)
	assert.False(t, base.Configured())
}

func TestResolveData_URLNamesAnExistingServer(t *testing.T) {
	t.Parallel()

	base, err := runResolveData(t, "--data", "https://data.example.com/team/")
	require.NoError(t, err)
	assert.True(t, base.Configured())
}

func TestResolveData_RefusesCleartextToARemoteHostAtStartup(t *testing.T) {
	t.Parallel()

	// The transport rule has no exemption for being named on the command line;
	// and it fails HERE, not later as a deferred #IMPORT!.
	_, err := runResolveData(t, "--data", "http://data.example.com/team/")
	require.ErrorIs(t, err, constants.ErrImportScheme)
}

func TestResolveData_PathStartsAScopedServer(t *testing.T) {
	t.Parallel()

	base, err := runResolveData(t, "--data", dataDir(t))
	require.NoError(t, err)
	assert.True(t, base.Configured())
}

func TestResolveData_MistypedDirectoryFailsAtStartup(t *testing.T) {
	t.Parallel()

	// A directory that cannot be published is an operator error, surfaced now —
	// not as one #IMPORT! per reference at compute time.
	base, err := runResolveData(t, "--data", filepath.Join(t.TempDir(), "typo"))
	require.ErrorIs(t, err, constants.ErrDataRoot)
	assert.False(t, base.Configured())
}

func TestHasScheme_DistinguishesURLsFromPaths(t *testing.T) {
	t.Parallel()

	cases := map[string]bool{
		"https://data.example.com/": true,
		"http://127.0.0.1:8137/":    true,
		"./data":                    false,
		"data":                      false,
		"/var/lib/data":             false,
		`C:\data`:                   false, // a Windows drive is a path, not a scheme
	}
	for raw, want := range cases {
		assert.Equal(t, want, hasScheme(raw), raw)
	}
}

func TestStartScopedData_UnbindableAddressPropagates(t *testing.T) {
	t.Parallel()

	// Exercised through the exported server so the failure path is the real one.
	_, err := dataserve.Start(dataserve.Root(dataDir(t)), "127.0.0.1:-1")
	require.ErrorIs(t, err, constants.ErrDataListen)
}

func TestRunData_RequiresADirectory(t *testing.T) {
	t.Parallel()

	err := runData(context.Background(), dataConfig{})
	require.ErrorIs(t, err, constants.ErrMissingArgument)
}

func TestRunData_ServesUntilTheContextIsCancelled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.NoError(t, runData(ctx, dataConfig{root: dataserve.Root(dataDir(t)), host: "127.0.0.1"}))
}

func TestDataCommand_RunsThroughTheRootCommand(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cmd := Command("test")
	require.NoError(t, cmd.Run(ctx, []string{name, cmdData, "--port", "0", dataDir(t)}))
}

// TestRender_RelativeReferenceResolvesAgainstAScopedServer is the end-to-end
// acceptance: a sheet whose only data reference is a bare name, rendered with
// --data and NO import flags at all.
func TestRender_RelativeReferenceResolvesAgainstAScopedServer(t *testing.T) {
	t.Parallel()
	sheet := writeTemp(t, "rel.tsvt", "=importsheet(\"balances.tsv\")\n")
	out, err := runCLI(t, "render", "--data", dataDir(t), sheet)
	require.NoError(t, err)
	assert.Contains(t, out, "Brokerage\t310000")
}

func TestRender_RelativeReferenceWithoutDataIsImportError(t *testing.T) {
	t.Parallel()
	sheet := writeTemp(t, "rel.tsvt", "=importsheet(\"balances.tsv\")\n")
	out, err := runCLI(t, "render", sheet)
	require.NoError(t, err)
	assert.Contains(t, out, "#IMPORT!")
}

func TestRender_TraversalAboveTheBaseIsRefused(t *testing.T) {
	t.Parallel()
	sheet := writeTemp(t, "esc.tsvt", "=importcell(\"../../etc/hosts\")\n")
	out, err := runCLI(t, "render", "--data", dataDir(t), sheet)
	require.NoError(t, err)
	assert.Contains(t, out, "#IMPORT!")
}

func TestRender_DataDoesNotWidenTheImportAllowlist(t *testing.T) {
	t.Parallel()
	// A live server the sheet names absolutely: --data authorized a base, and a
	// base is not a host, so this must still be denied.
	server := httptest.NewServer(nil)
	t.Cleanup(server.Close)

	sheet := writeTemp(t, "abs.tsvt", "=importcell(\""+server.URL+"/x.tsv\")\n")
	out, err := runCLI(t, "render", "--data", dataDir(t), sheet)
	require.NoError(t, err)
	assert.Contains(t, out, "#IMPORT!")
}

func TestRender_BadDataFlagFailsBeforeComputing(t *testing.T) {
	t.Parallel()
	sheet := writeTemp(t, "rel.tsvt", "=importsheet(\"balances.tsv\")\n")
	_, err := runCLI(t, "render", "--data", "http://data.example.com/", sheet)
	require.ErrorIs(t, err, constants.ErrImportScheme)
}

func TestRunData_MistypedDirectoryFailsBeforeServing(t *testing.T) {
	t.Parallel()

	err := runData(context.Background(), dataConfig{
		root: dataserve.Root(filepath.Join(t.TempDir(), "typo")),
		host: "127.0.0.1",
	})
	require.ErrorIs(t, err, constants.ErrDataRoot)
}

func TestServe_BadDataFlagFailsBeforeBinding(t *testing.T) {
	t.Parallel()
	sheet := writeTemp(t, "s.tsvt", "1\n")
	_, err := runCLI(t, "serve", "--data", "http://data.example.com/", sheet)
	require.ErrorIs(t, err, constants.ErrImportScheme)
}

func TestTUI_BadDataFlagFailsBeforeStarting(t *testing.T) {
	t.Parallel()
	sheet := writeTemp(t, "t.tsvt", "1\n")
	_, err := runCLI(t, "tui", "--data", "http://data.example.com/", sheet)
	require.ErrorIs(t, err, constants.ErrImportScheme)
}

func TestLoopbackBase_ResolvesWithoutParsing(t *testing.T) {
	t.Parallel()

	base := importer.LoopbackBase("127.0.0.1:8137")
	assert.True(t, base.Configured())
}
