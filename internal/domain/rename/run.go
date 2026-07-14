package rename

import (
	"context"
	"log/slog"
	"strings"

	"github.com/gomatic/go-module"
	"github.com/gomatic/go-rewrite"

	"github.com/gomatic/template.cli/internal/constants"
	"github.com/gomatic/template.cli/internal/domain"
)

// Result is the outcome of the rename command: the identity it moved from and to,
// whether it was a dry run, and the files whose contents changed.
type Result struct {
	FromModule    string   `json:"from_module"`
	ToModule      string   `json:"to_module"`
	FromName      string   `json:"from_name"`
	ToName        string   `json:"to_name"`
	Changed       []string `json:"changed"`
	DryRunEnabled bool     `json:"dry_run"`
}

// deps builds the git and filesystem seams from the working directory. It is
// indirected through a variable so tests substitute in-memory fakes.
var deps = osDeps

// Run renames the project's template identity to the target derived from the git
// origin remote, optionally overriding the command name with the first argument.
// It validates the name, discovers the identities, builds the plan, and applies
// it through the rewrite engine, honoring dry-run mode.
func Run(_ context.Context, logger *slog.Logger, cfg Config, args ...domain.Argument) (Result, error) {
	override := overrideName(args)
	if err := validate(override); err != nil {
		return Result{}, err
	}

	git, fs, err := deps()
	if err != nil {
		return Result{}, err
	}

	current, target, err := rewrite.Discover(git, fs, override)
	if err != nil {
		return Result{}, err
	}

	changed, err := rewrite.BuildPlan(current, target).Apply(fs, rewrite.DryRun(cfg.DryRunEnabled))
	if err != nil {
		return Result{}, err
	}

	logger.Info(
		"Rename complete.",
		"from",
		current.Module,
		"to",
		target.Module,
		"files",
		len(changed),
		"dry_run",
		bool(cfg.DryRunEnabled),
	)
	return result(current, target, cfg, changed), nil
}

// overrideName returns the optional command-name override from the positional
// args, or empty when none is supplied (meaning: use the repository name).
func overrideName(args []string) module.Name {
	if len(args) > 0 {
		return module.Name(args[0])
	}
	return ""
}

// validate rejects a non-empty override that is not a single path segment.
func validate(name module.Name) error {
	if name != "" && strings.ContainsAny(string(name), "/ \t") {
		return constants.ErrInvalidName.With(nil, "name", string(name))
	}
	return nil
}

// result assembles the command's Result from the discovered identities.
func result(current, target rewrite.Identity, cfg Config, changed rewrite.Changed) Result {
	return Result{
		FromModule:    string(current.Module),
		ToModule:      string(target.Module),
		FromName:      string(current.Name),
		ToName:        string(target.Name),
		DryRunEnabled: bool(cfg.DryRunEnabled),
		Changed:       paths(changed),
	}
}

// paths converts the changed file list to plain strings for the result.
func paths(changed rewrite.Changed) []string {
	out := make([]string, len(changed))
	for i, file := range changed {
		out[i] = string(file)
	}
	return out
}

// osDeps wires the OS-backed git and filesystem seams rooted at the working
// directory; the filesystem enumerates tracked files lazily through git.
func osDeps() (rewrite.Git, rewrite.FileSystem, error) {
	git := rewrite.OSGit{Dir: "."}
	return git, rewrite.OSFileSystem{Root: ".", Lister: git.Files}, nil
}
