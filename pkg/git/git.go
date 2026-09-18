// Package git wraps the git commands wt needs.
//
// wt shells out to git rather than linking a git library: worktree creation is
// exactly what the git CLI is good at, and matching its behavior (DWIM branch
// resolution, hooks, config) matters more here than avoiding a process.
//
// Every call takes the directory to run in, which for wt is always the hub
// repository (a bare repository or a main worktree's .git). git resolves
// worktree paths against the repository rather than the current directory, so
// the caller never has to chdir.
package git

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/suzuki-shunsuke/slog-error/slogerr"
)

// Runner runs a git command in dir and returns its standard output. It is the
// seam a test replaces to drive the package without a real repository.
type Runner interface {
	Run(ctx context.Context, dir string, args ...string) (string, error)
}

// ExecRunner runs git as a child process.
type ExecRunner struct{}

// Run executes `git -C <dir> <args...>` and returns its standard output.
// The error carries git's standard error, which is where git explains itself;
// without it the caller would only see the exit status.
func (r *ExecRunner) Run(ctx context.Context, dir string, args ...string) (string, error) {
	a := make([]string, 0, len(args)+2) //nolint:mnd // -C and dir
	a = append(a, "-C", dir)
	a = append(a, args...)
	cmd := exec.CommandContext(ctx, "git", a...)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("run a git command: %w", slogerr.With(err,
			"git_args", strings.Join(args, " "),
			"git_dir", dir,
			"stderr", strings.TrimSpace(stderr.String())))
	}
	return stdout.String(), nil
}

// Client runs git commands through a Runner.
type Client struct {
	runner Runner
}

// New creates a Client running git as a child process.
func New() *Client {
	return &Client{runner: &ExecRunner{}}
}

// NewWithRunner creates a Client running git through runner.
func NewWithRunner(runner Runner) *Client {
	return &Client{runner: runner}
}

// Worktree is one entry of `git worktree list`.
type Worktree struct {
	// Path is the absolute path of the worktree.
	Path string
	// Branch is the branch checked out there, without the refs/heads/ prefix.
	// It is empty for a bare repository and for a detached HEAD.
	Branch string
}

// Worktrees lists the worktrees of the repository at dir.
//
// It parses the porcelain format rather than the human one, which is documented
// as stable and gives each field on its own line.
func (c *Client) Worktrees(ctx context.Context, dir string) ([]*Worktree, error) {
	out, err := c.runner.Run(ctx, dir, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err //nolint:wrapcheck // Run already describes the failure.
	}
	const branchPrefix = "branch refs/heads/"
	var worktrees []*Worktree
	var current *Worktree
	for line := range strings.SplitSeq(out, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			// A blank line separates the entries, but a bare repository's entry has no
			// branch line, so the entry is appended as soon as it starts and filled in
			// afterwards rather than on the separator.
			current = &Worktree{Path: strings.TrimPrefix(line, "worktree ")}
			worktrees = append(worktrees, current)
		case strings.HasPrefix(line, branchPrefix) && current != nil:
			current.Branch = strings.TrimPrefix(line, branchPrefix)
		}
	}
	return worktrees, nil
}

// HasBranch reports whether the repository at dir has a local branch.
//
// A non-zero exit is how show-ref says "no such ref", so the error is swallowed
// rather than reported; a genuine failure (a broken repository) surfaces at the
// next command, which needs the repository anyway.
func (c *Client) HasBranch(ctx context.Context, dir, branch string) bool {
	_, err := c.runner.Run(ctx, dir, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	return err == nil
}

// Fetch runs `git fetch origin <refspec>` in the repository at dir.
func (c *Client) Fetch(ctx context.Context, dir, refspec string) error {
	_, err := c.runner.Run(ctx, dir, "fetch", "origin", refspec)
	return err //nolint:wrapcheck // Run already describes the failure.
}

// AddWorktree runs `git worktree add` in the repository at dir with args.
func (c *Client) AddWorktree(ctx context.Context, dir string, args ...string) error {
	_, err := c.runner.Run(ctx, dir, append([]string{"worktree", "add"}, args...)...)
	return err //nolint:wrapcheck // Run already describes the failure.
}

// DefaultFetchRefspec is the refspec a normal clone configures for origin. It is
// what makes refs/remotes/origin/* exist, and with it everything git does with a
// remote-tracking branch — origin/<branch> as a name, --track, @{upstream}, a
// bare `git fetch` — behaves the same on a bare hub as in a normal clone.
const DefaultFetchRefspec = "+refs/heads/*:refs/remotes/origin/*"

// FetchRefspec returns the fetch refspec configured for origin, or an empty
// string when there is none. `git clone --bare` configures none.
func (c *Client) FetchRefspec(ctx context.Context, dir string) string {
	// A missing key exits non-zero, which is the answer rather than a failure.
	out, err := c.runner.Run(ctx, dir, "config", "--get", "remote.origin.fetch")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// SetDefaultFetchRefspec configures the refspec a normal clone would have.
func (c *Client) SetDefaultFetchRefspec(ctx context.Context, dir string) error {
	_, err := c.runner.Run(ctx, dir, "config", "remote.origin.fetch", DefaultFetchRefspec)
	return err //nolint:wrapcheck // Run already describes the failure.
}

// RemoteBranchExists reports whether origin has a branch of this name.
//
// It asks the remote rather than looking at refs/remotes/origin/*, which say
// what the last fetch saw and are absent altogether on a hub created by
// `git clone --bare`.
func (c *Client) RemoteBranchExists(ctx context.Context, dir, branch string) (bool, error) {
	out, err := c.runner.Run(ctx, dir, "ls-remote", "--heads", "origin", "refs/heads/"+branch)
	if err != nil {
		return false, err //nolint:wrapcheck // Run already describes the failure.
	}
	return strings.TrimSpace(out) != "", nil
}

// HeadCommit returns the commit HEAD points at in the repository at dir.
func (c *Client) HeadCommit(ctx context.Context, dir string) (string, error) {
	out, err := c.runner.Run(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		return "", err //nolint:wrapcheck // Run already describes the failure.
	}
	return strings.TrimSpace(out), nil
}

// OriginURL returns the URL of the remote named origin of the repository at dir.
func (c *Client) OriginURL(ctx context.Context, dir string) (string, error) {
	out, err := c.runner.Run(ctx, dir, "remote", "get-url", "origin")
	if err != nil {
		return "", err //nolint:wrapcheck // Run already describes the failure.
	}
	return strings.TrimSpace(out), nil
}
