// Package cli provides the command-line interface layer for wt.
// It builds the command tree, registers the global flags, and routes to the
// subcommand packages, which delegate the actual work to the controllers.
package cli

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/suzuki-shunsuke/cobra-util/cobrautil"
	"github.com/suzuki-shunsuke/slog-util/slogutil"
	"github.com/suzuki-shunsuke/wt/pkg/cli/add"
	"github.com/suzuki-shunsuke/wt/pkg/cli/flag"
	"github.com/suzuki-shunsuke/wt/pkg/cli/initcmd"
)

// Run creates and executes the wt CLI.
func Run(ctx context.Context, logger *slogutil.Logger, env *cobrautil.Env) error {
	gFlags := &flag.GlobalFlags{}
	// The context reaches the commands through ExecuteContext below, which is what
	// cobra hands to the action as cmd.Context(); building the tree needs none.
	cmd := newCommand(logger, env, gFlags) //nolint:contextcheck
	return cmd.ExecuteContext(ctx)         //nolint:wrapcheck // Main reports it.
}

// newCommand builds the command tree. It is separate from Run so that a test can
// run the real tree with its output captured, which Run can't offer: the
// commands write to the process's stdout.
func newCommand(logger *slogutil.Logger, env *cobrautil.Env, gFlags *flag.GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wt",
		Short: "Manage git worktrees at a conventional path. https://github.com/suzuki-shunsuke/wt",
		Long: `Manage git worktrees at a conventional path.

wt keeps every worktree of a repository under one directory:

  <root>/github.com/<owner>/<repo>+worktrees/
  |-- .bare                 the bare repository, when the repository is cloned that way
  |-- main
  |-- fix-checksum
  '-- feat/x                a branch name holding a slash simply nests

root comes from the configuration file; run 'wt init' to create it.

'wt add' takes a branch name, a pull request number, or a pull request URL,
creates the worktree if it is not there yet, and prints its absolute path, so it
composes as 'cd "$(wt add 1234)"'.

See each subcommand's help with 'wt help <command>'.`,
	}
	// --log-level and --config are persistent, so they can be given either before
	// or after the subcommand, and every subcommand reads them through the same
	// GlobalFlags.
	flag.LogLevel(cmd.PersistentFlags(), &gFlags.LogLevel)
	flag.Config(cmd.PersistentFlags(), &gFlags.Config)
	cmd.AddCommand(
		initcmd.New(logger, env, gFlags),
		add.New(logger, env, gFlags),
	)
	return cobrautil.Command(env, cmd, &cobrautil.Options{})
}
