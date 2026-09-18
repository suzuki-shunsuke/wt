// Package add implements the 'wt add' command, which creates a git worktree at
// the conventional path and prints its absolute path.
//
// The argument is a branch name, a pull request number, or a pull request URL.
// Only the resulting path goes to stdout, so the command composes as
// `cd "$(wt add ...)"`.
package add

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/suzuki-shunsuke/cobra-util/cobrautil"
	"github.com/suzuki-shunsuke/slog-util/slogutil"
	"github.com/suzuki-shunsuke/wt/pkg/cli/flag"
	"github.com/suzuki-shunsuke/wt/pkg/config"
	"github.com/suzuki-shunsuke/wt/pkg/controller/add"
	"github.com/suzuki-shunsuke/wt/pkg/git"
	"github.com/suzuki-shunsuke/wt/pkg/github"
)

// Args holds the flag and argument values for the add command.
type Args struct {
	*flag.GlobalFlags

	// Arg is the positional argument: a branch name, a pull request number, or a
	// pull request URL.
	Arg string
}

// New creates the add command.
func New(logger *slogutil.Logger, env *cobrautil.Env, gFlags *flag.GlobalFlags) *cobra.Command {
	args := &Args{GlobalFlags: gFlags}
	r := &runner{getenv: env.Getenv, stdout: env.Stdout}
	return r.Command(logger, args)
}

type runner struct {
	getenv func(string) string
	stdout io.Writer
}

func (r *runner) Command(logger *slogutil.Logger, args *Args) *cobra.Command {
	return &cobra.Command{
		Use:   "add <branch | pull request number | pull request URL>",
		Short: "Create a git worktree and output its absolute path",
		Long: `Create a git worktree at the conventional path and output its absolute path.

The worktree is created at <root>/github.com/<owner>/<repo>+worktrees/<branch>,
where root comes from the configuration file. Run 'wt init' to create it.

The argument is read as a pull request URL, then as a pull request number if it
is all digits, and as a branch name otherwise. A URL names its own repository;
anything else refers to the repository of the current directory. A branch named
with digits can still be reached through its URL.

A branch that exists only on origin is fetched first. A pull request from a fork
is fetched through refs/pull/<number>/head and checked out as pr-<number>,
because its head branch name belongs to the fork's namespace.

If the branch already has a worktree, its path is printed and nothing is
created, so the command can be run again safely.

Only the path is written to stdout, so it composes with cd:

$ wt add 1234
$ cd "$(wt add feat/foo)"
$ cd "$(wt add https://github.com/suzuki-shunsuke/wt/pull/1)"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, positional []string) error {
			args.Arg = positional[0]
			return r.action(cmd.Context(), logger, args)
		},
	}
}

// action loads the configuration and delegates to the add controller.
//
// The configuration path, the working directory, and the GitHub client's
// environment are resolved here so the controller receives settled values and
// needs no environment of its own.
func (r *runner) action(ctx context.Context, logger *slogutil.Logger, args *Args) error {
	if err := logger.SetLevel(args.LogLevel); err != nil {
		return fmt.Errorf("set log level: %w", err)
	}
	p, err := config.ResolvePath(args.Config, r.getenv)
	if err != nil {
		return err //nolint:wrapcheck // ResolvePath already describes the failure.
	}
	cfg, err := config.Load(p)
	if err != nil {
		return err //nolint:wrapcheck // Load already describes the failure.
	}
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get the current directory: %w", err)
	}
	return add.New(&add.Input{ //nolint:wrapcheck // Run already describes the failure.
		Git: git.New(),
		NewGitHub: func(ctx context.Context, logger *slog.Logger) (add.GitHub, error) {
			return github.NewClient(ctx, logger, r.getenv)
		},
		Stdout: r.stdout,
	}).Run(ctx, logger.Logger, &add.InputAdd{
		Arg:  args.Arg,
		Root: cfg.Root,
		Dir:  wd,
	})
}
