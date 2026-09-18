package add

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"

	"github.com/suzuki-shunsuke/slog-error/slogerr"
	"github.com/suzuki-shunsuke/wt/pkg/git"
)

// errHubNotFound is returned when the repository is not on this machine. Cloning
// it here would block for minutes on a large repository with nothing said about
// it, so wt reports the clone command instead and lets the user decide.
var errHubNotFound = errors.New("the repository is not found on this machine")

// errOccupied is returned when the target path exists but git does not know it as
// a worktree, which is the one case where creating the worktree would have to
// destroy something.
var errOccupied = errors.New("the path already exists but is not a registered worktree")

// Run adds a worktree for the branch or pull request named by the argument and
// writes its absolute path to Stdout.
func (c *Controller) Run(ctx context.Context, logger *slog.Logger, input *InputAdd) error {
	t, err := parseArg(ctx, c.input.Git, input.Arg, input.Dir)
	if err != nil {
		return err
	}

	base := filepath.Join(input.Root, "github.com", t.owner, t.repo)
	hub, err := findHub(base)
	if err != nil {
		return err
	}

	branch, refspec, err := c.resolveBranch(ctx, logger, t)
	if err != nil {
		return err
	}

	// An existing worktree wins over the conventional path: the branch can only be
	// checked out once, so creating a second worktree for it would fail anyway, and
	// returning where it actually is makes the command idempotent even for a
	// worktree that predates this layout.
	worktrees, err := c.input.Git.Worktrees(ctx, hub)
	if err != nil {
		return fmt.Errorf("list the worktrees: %w", err)
	}
	for _, w := range worktrees {
		if w.Branch == branch {
			logger.Debug("the worktree already exists", "path", w.Path, "branch", branch)
			return c.print(w.Path)
		}
	}

	dst := filepath.Join(base+"+worktrees", branch)
	if _, err := os.Stat(dst); err == nil {
		//nolint:wrapcheck // The error is this package's own sentinel, only annotated.
		return slogerr.With(errOccupied, "path", dst)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("check if the worktree path exists: %w", slogerr.With(err, "path", dst))
	}

	if err := c.createWorktree(ctx, logger, hub, dst, branch, refspec); err != nil {
		return err
	}
	return c.print(dst)
}

// resolveBranch determines the branch to check out and, when it has to be
// fetched from a fork, the refspec that brings it in.
//
// A pull request from a fork has no branch in the base repository's remote, so it
// is fetched through refs/pull/<number>/head and named pr-<number> locally: the
// head branch name belongs to the fork's namespace, where it may well collide
// with a branch of the same name here.
func (c *Controller) resolveBranch(ctx context.Context, logger *slog.Logger, t *target) (branch, refspec string, err error) {
	if t.prNumber == 0 {
		return t.branch, "", nil
	}
	gh, err := c.input.NewGitHub(ctx, logger)
	if err != nil {
		return "", "", err
	}
	pr, err := gh.GetPR(ctx, t.owner, t.repo, t.prNumber)
	if err != nil {
		return "", "", err //nolint:wrapcheck // GetPR already describes the failure.
	}
	if !pr.CrossRepository {
		logger.Debug("resolved the pull request", "branch", pr.HeadBranch, "pull_request", t.prNumber)
		return pr.HeadBranch, "", nil
	}
	branch = "pr-" + strconv.Itoa(t.prNumber)
	logger.Debug("the pull request comes from a fork, so its head is fetched through refs/pull",
		"branch", branch, "head_branch", pr.HeadBranch, "pull_request", t.prNumber)
	return branch, fmt.Sprintf("refs/pull/%d/head:%s", t.prNumber, branch), nil
}

// createWorktree creates the worktree at dst.
//
// A branch already on this machine is checked out as it is, without contacting
// the remote, so that an offline or local-only branch works. Otherwise the branch
// is fetched first and created tracking origin, which is what the user means by
// naming a branch that only exists upstream.
func (c *Controller) createWorktree(ctx context.Context, logger *slog.Logger, hub, dst, branch, refspec string) error {
	// git worktree add creates the intermediate directories itself, but only under
	// a parent that exists; a branch name holding a slash needs the parent first.
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil { //nolint:mnd // The standard mode for a directory.
		return fmt.Errorf("create the parent directory of the worktree: %w", slogerr.With(err, "path", dst))
	}

	switch {
	case refspec != "":
		logger.Info("fetching the pull request head", "refspec", refspec)
		if err := c.input.Git.Fetch(ctx, hub, refspec); err != nil {
			return fmt.Errorf("fetch the pull request head: %w", err)
		}
	case c.input.Git.HasBranch(ctx, hub, branch):
	default:
		if err := c.ensureFetchRefspec(ctx, logger, hub); err != nil {
			return err
		}
		logger.Info("fetching the branch from origin", "branch", branch)
		if err := c.input.Git.Fetch(ctx, hub, branch); err != nil {
			return fmt.Errorf("fetch the branch from origin: %w", slogerr.With(err, "branch", branch))
		}
		// --track makes the new branch follow origin, so git push and git pull in
		// the worktree need no argument.
		if err := c.input.Git.AddWorktree(ctx, hub, "--track", "-b", branch, dst, "origin/"+branch); err != nil {
			return fmt.Errorf("add a worktree tracking origin: %w", slogerr.With(err, "branch", branch, "path", dst))
		}
		logger.Info("the worktree has been created", "path", dst, "branch", branch)
		return nil
	}

	if err := c.input.Git.AddWorktree(ctx, hub, dst, branch); err != nil {
		return fmt.Errorf("add a worktree: %w", slogerr.With(err, "branch", branch, "path", dst))
	}
	logger.Info("the worktree has been created", "path", dst, "branch", branch)
	return nil
}

// ensureFetchRefspec gives origin the fetch refspec a normal clone has, when it
// has none.
//
// `git clone --bare`, which is how the hub of this layout is created, configures
// no refspec, so nothing ever writes refs/remotes/origin/*. Without those refs
// there is no origin/<branch> to branch from, --track has nothing to record, and
// @{upstream} cannot be resolved, so a worktree for a branch that only exists
// upstream could not be created at all.
//
// An existing refspec is left alone, including a narrower one somebody chose on
// purpose. The change is logged rather than made quietly, because it is a change
// to the user's repository, and it makes a plain `git fetch` in the hub start
// populating remote-tracking branches as well.
func (c *Controller) ensureFetchRefspec(ctx context.Context, logger *slog.Logger, hub string) error {
	if c.input.Git.FetchRefspec(ctx, hub) != "" {
		return nil
	}
	logger.Info("configuring the fetch refspec of origin, which this repository has none of, "+
		"so that remote-tracking branches exist",
		"repository", hub, "refspec", git.DefaultFetchRefspec)
	if err := c.input.Git.SetDefaultFetchRefspec(ctx, hub); err != nil {
		return fmt.Errorf("configure the fetch refspec of origin: %w", slogerr.With(err, "repository", hub))
	}
	return nil
}

// findHub returns the git directory of the repository whose worktrees live under
// base+"+worktrees".
//
// The conventional hub comes first, then the two shapes a repository may still
// have before it is migrated: a normal clone, and a bare repository named
// <repo>.git. Either of them can host a worktree perfectly well, so wt creates
// the worktree at the conventional path regardless of where the hub is, rather
// than refusing until the repository is migrated.
func findHub(base string) (string, error) {
	candidates := []string{
		base + "+worktrees/.bare",
		filepath.Join(base, ".git"),
		base + ".git",
	}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && fi.IsDir() {
			return c, nil
		}
	}
	//nolint:wrapcheck // The error is this package's own sentinel, only annotated.
	return "", slogerr.With(errHubNotFound, "hint", cloneHint(base))
}

// cloneHint is the command that puts the repository where wt expects it.
func cloneHint(base string) string {
	owner := filepath.Base(filepath.Dir(base))
	repo := filepath.Base(base)
	return fmt.Sprintf("git clone --bare https://github.com/%s/%s.git %s+worktrees/.bare", owner, repo, base)
}

// print writes the resulting path, and nothing else, to stdout.
//
// The path is resolved through any symlink first, because the two ways it is
// produced disagree otherwise: a path wt just built by joining is literal, while
// one read back from `git worktree list` has been resolved by git. On macOS,
// where /tmp is a symlink, that is the difference between /tmp/x and
// /private/tmp/x for the same directory, which would make the output of two
// identical commands differ. Both forms work with cd, but a caller comparing or
// caching the output should not have to know which path it got.
//
// A path that cannot be resolved is printed as it is: it has just been created,
// so this only happens when something outside wt removed it, and reporting where
// it was put is more useful than failing.
func (c *Controller) print(path string) error {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	if _, err := fmt.Fprintln(c.input.Stdout, path); err != nil {
		return fmt.Errorf("write the worktree path: %w", err)
	}
	return nil
}
