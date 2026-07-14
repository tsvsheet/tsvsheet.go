package rename

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/gomatic/go-module"
	"github.com/gomatic/go-rewrite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gomatic/template.cli/internal/constants"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelWarn}))
}

// fakeFS is an in-memory rewrite.FileSystem for domain tests.
type fakeFS struct {
	writeErr error
	data     map[rewrite.FilePath][]byte
	files    []rewrite.FilePath
}

func (f *fakeFS) List() ([]rewrite.FilePath, error) { return f.files, nil }

func (f *fakeFS) Read(path rewrite.FilePath) ([]byte, error) { return f.data[path], nil }

func (f *fakeFS) Write(path rewrite.FilePath, data []byte) error {
	if f.writeErr != nil {
		return f.writeErr
	}
	f.data[path] = data
	return nil
}

func (f *fakeFS) Move(rewrite.FilePath, rewrite.FilePath) error { return nil }

type fakeGit struct {
	err    error
	remote module.Remote
}

func (g fakeGit) Remote() (module.Remote, error) { return g.remote, g.err }

// The fixtures use a neutral before.cli -> after.cli identity that shares no
// substring with this project's own identity, so the rename command never
// rewrites these tests when it renames the project that ships them.
const targetRemote = module.Remote("git@example.com:org/after.cli.git")

func sourceFS() *fakeFS {
	return &fakeFS{
		files: []rewrite.FilePath{"go.mod", "project.go", "cmd/before.cli/main.go"},
		data: map[rewrite.FilePath][]byte{
			"go.mod":                 []byte("module example.com/org/before.cli\n"),
			"project.go":             []byte("package beforecli\n"),
			"cmd/before.cli/main.go": []byte("name = \"before.cli\"\n"),
		},
	}
}

// withDeps swaps the dependency seam for the duration of a test.
func withDeps(t *testing.T, git rewrite.Git, fs rewrite.FileSystem, err error) {
	t.Helper()
	original := deps
	t.Cleanup(func() { deps = original })
	deps = func() (rewrite.Git, rewrite.FileSystem, error) { return git, fs, err }
}

func TestRun(t *testing.T) {
	want, must := assert.New(t), require.New(t)

	fs := sourceFS()
	withDeps(t, fakeGit{remote: targetRemote}, fs, nil)

	result, err := Run(context.Background(), testLogger(), Config{})

	must.NoError(err)
	want.Equal("example.com/org/before.cli", result.FromModule)
	want.Equal("example.com/org/after.cli", result.ToModule)
	want.Equal("before.cli", result.FromName)
	want.Equal("after.cli", result.ToName)
	want.False(result.DryRunEnabled)
	want.ElementsMatch([]string{"go.mod", "project.go", "cmd/before.cli/main.go"}, result.Changed)
	want.Equal("module example.com/org/after.cli\n", string(fs.data["go.mod"]))
}

func TestRunOverrideName(t *testing.T) {
	want, must := assert.New(t), require.New(t)

	withDeps(t, fakeGit{remote: targetRemote}, sourceFS(), nil)

	result, err := Run(context.Background(), testLogger(), Config{}, "mytool")

	must.NoError(err)
	want.Equal("mytool", result.ToName)
	want.Equal("example.com/org/after.cli", result.ToModule, "module always follows the remote")
}

func TestRunDryRun(t *testing.T) {
	want, must := assert.New(t), require.New(t)

	fs := sourceFS()
	withDeps(t, fakeGit{remote: targetRemote}, fs, nil)

	result, err := Run(context.Background(), testLogger(), Config{DryRunEnabled: true})

	must.NoError(err)
	want.True(result.DryRunEnabled)
	want.Equal("module example.com/org/before.cli\n", string(fs.data["go.mod"]), "dry run must not write")
}

func TestRunInvalidName(t *testing.T) {
	must := require.New(t)

	_, err := Run(context.Background(), testLogger(), Config{}, "bad/name")

	must.ErrorIs(err, constants.ErrInvalidName)
}

func TestRunDepsError(t *testing.T) {
	must := require.New(t)

	withDeps(t, nil, nil, constants.ErrGitCommand)

	_, err := Run(context.Background(), testLogger(), Config{})

	must.ErrorIs(err, constants.ErrGitCommand)
}

func TestRunDiscoverError(t *testing.T) {
	must := require.New(t)

	withDeps(t, fakeGit{err: constants.ErrGitCommand}, sourceFS(), nil)

	_, err := Run(context.Background(), testLogger(), Config{})

	must.ErrorIs(err, constants.ErrGitCommand)
}

func TestRunApplyError(t *testing.T) {
	must := require.New(t)

	fs := sourceFS()
	fs.writeErr = constants.ErrWriteFile
	withDeps(t, fakeGit{remote: targetRemote}, fs, nil)

	_, err := Run(context.Background(), testLogger(), Config{})

	must.ErrorIs(err, constants.ErrWriteFile)
}

func TestOSDeps(t *testing.T) {
	want, must := assert.New(t), require.New(t)

	git, fs, err := osDeps()

	must.NoError(err)
	want.NotNil(git)
	want.NotNil(fs)
}
