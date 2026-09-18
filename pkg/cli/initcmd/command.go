// Package initcmd implements the 'wt init' command, which writes the default
// configuration file so that wt knows where the repositories live.
//
// It is the first command to run on a new machine. Running it again on an
// existing configuration reports that it is there and leaves it untouched.
package initcmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/suzuki-shunsuke/cobra-util/cobrautil"
	"github.com/suzuki-shunsuke/slog-util/slogutil"
	"github.com/suzuki-shunsuke/wt/pkg/cli/flag"
	"github.com/suzuki-shunsuke/wt/pkg/config"
	"github.com/suzuki-shunsuke/wt/pkg/controller/initcmd"
)

// Args holds the flag values for the init command.
type Args struct {
	*flag.GlobalFlags
}

// New creates the init command.
func New(logger *slogutil.Logger, env *cobrautil.Env, gFlags *flag.GlobalFlags) *cobra.Command {
	args := &Args{GlobalFlags: gFlags}
	r := &runner{getenv: env.Getenv}
	return r.Command(logger, args)
}

type runner struct {
	getenv func(string) string
}

func (r *runner) Command(logger *slogutil.Logger, args *Args) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Create the configuration file",
		Long: `Create the configuration file with its default content.

The file is written to $XDG_CONFIG_HOME/wt/wt.yaml, or ~/.config/wt/wt.yaml when
XDG_CONFIG_HOME is unset. Pass -c to write it somewhere else.

It holds one setting, root, the directory under which your repositories live.
A repository is expected at <root>/github.com/<owner>/<repo>, and wt creates its
worktrees at <root>/github.com/<owner>/<repo>+worktrees/<branch>. Edit root if
the default does not match your machine.

An existing file is never overwritten, so this is safe to run again.

$ wt init`,
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return r.action(logger, args)
		},
	}
}

// action resolves the configuration file path and writes the file.
// The path is resolved here rather than in the controller so that the controller
// stays free of environment lookups and is easy to test.
func (r *runner) action(logger *slogutil.Logger, args *Args) error {
	if err := logger.SetLevel(args.LogLevel); err != nil {
		return fmt.Errorf("set log level: %w", err)
	}
	p, err := config.ResolvePath(args.Config, r.getenv)
	if err != nil {
		return err //nolint:wrapcheck // ResolvePath already describes the failure.
	}
	return initcmd.New().Init(logger.Logger, p) //nolint:wrapcheck // Init already describes the failure.
}
