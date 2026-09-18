// Package add implements the controller for 'wt add', which creates a git
// worktree at the conventional path and prints its absolute path.
//
// The layout is <root>/github.com/<owner>/<repo>+worktrees/<branch>, with the
// hub repository at <repo>+worktrees/.bare. Grouping every worktree of a
// repository under one directory keeps them out of the owner directory that ghq
// and friends list, and the dot in .bare keeps the hub out of both that listing
// and a plain ls, so everything that shows up is a directory you can work in.
//
// The command is idempotent: asking for a branch that already has a worktree
// prints its path and changes nothing, so it can be used as `cd "$(wt add ...)"`
// whether or not the worktree is there yet.
package add

import (
	"context"
	"io"
	"log/slog"

	"github.com/suzuki-shunsuke/wt/pkg/git"
	"github.com/suzuki-shunsuke/wt/pkg/github"
)

// Git is the subset of the git client used to create a worktree.
type Git interface {
	Worktrees(ctx context.Context, dir string) ([]*git.Worktree, error)
	HasBranch(ctx context.Context, dir, branch string) bool
	Fetch(ctx context.Context, dir, refspec string) error
	AddWorktree(ctx context.Context, dir string, args ...string) error
	FetchRefspec(ctx context.Context, dir string) string
	SetDefaultFetchRefspec(ctx context.Context, dir string) error
	RemoteBranchExists(ctx context.Context, dir, branch string) (bool, error)
	HeadCommit(ctx context.Context, dir string) (string, error)
	OriginURL(ctx context.Context, dir string) (string, error)
}

// GitHub is the subset of the GitHub client used to resolve a pull request.
type GitHub interface {
	GetPR(ctx context.Context, owner, repo string, number int) (*github.PullRequest, error)
}

// Input contains the dependencies needed by the Controller.
type Input struct {
	Git Git
	// NewGitHub creates the GitHub client on demand. Building it acquires a token,
	// which may run an interactive device flow, so it must not happen when the
	// argument is a branch name and no pull request has to be looked up.
	NewGitHub func(ctx context.Context, logger *slog.Logger) (GitHub, error)
	// Stdout is where the resulting path is written. It carries nothing else, so
	// that `cd "$(wt add ...)"` works; everything else goes to the logger.
	Stdout io.Writer
}

// InputAdd holds the values needed to add a worktree.
type InputAdd struct {
	// Arg is the positional argument: a branch name, a pull request number, or a
	// pull request URL.
	Arg string
	// Root is the configured directory under which repositories live.
	Root string
	// Dir is the working directory, used to find the repository the command was
	// run in when Arg does not name one.
	Dir string
}

// Controller adds a worktree.
type Controller struct {
	input *Input
}

// New creates a Controller with the provided input.
func New(input *Input) *Controller {
	return &Controller{input: input}
}
