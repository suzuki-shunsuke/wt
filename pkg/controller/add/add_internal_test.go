package add

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/suzuki-shunsuke/wt/pkg/git"
	"github.com/suzuki-shunsuke/wt/pkg/github"
)

// stubGitHub returns a canned pull request.
type stubGitHub struct {
	pr  *github.PullRequest
	err error
}

func (s *stubGitHub) GetPR(_ context.Context, _, _ string, _ int) (*github.PullRequest, error) {
	return s.pr, s.err
}

// newLogger returns a logger that discards its output, for the cases that do not
// assert on it.
func newLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// setupRoot creates a root holding owner/repo with the given hub directory, and
// returns the root and the repository's base path.
func setupRoot(t *testing.T, hub string) (root, base string) {
	t.Helper()
	root = t.TempDir()
	base = filepath.Join(root, "github.com", "o", "r")
	if hub != "" {
		if err := os.MkdirAll(filepath.Join(base+"+worktrees", hub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root, base
}

// wantPath is the path a test expects on stdout. It is resolved through symlinks
// because Run resolves what it prints, and t.TempDir is under /var on macOS,
// which is a symlink to /private/var.
func wantPath(t *testing.T, p string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(p)
	if err != nil {
		return p
	}
	return resolved
}

func TestController_Run(t *testing.T) { //nolint:funlen,gocognit,cyclop // The length and the branching are the table of cases.
	t.Parallel()
	tests := []struct {
		name string
		arg  string
		// hub is created under <base>+worktrees. An empty hub creates nothing, which
		// is the repository-not-here case.
		hub string
		git *stubGit
		pr  *github.PullRequest
		// wantBranch is the branch whose worktree is expected, under <base>+worktrees.
		wantBranch string
		// wantAdd is the argument list expected of `git worktree add`.
		wantAdd []string
		// wantFetch is the refspec expected of `git fetch origin`.
		wantFetch string
		wantErr   string
	}{
		{
			name:    "a repository that is not here reports the clone command",
			arg:     "main",
			git:     &stubGit{originURL: "https://github.com/o/r.git"},
			wantErr: "the repository is not found on this machine",
		},
		{
			name: "an existing worktree is returned without creating anything",
			arg:  "main",
			hub:  ".bare",
			git: &stubGit{
				originURL: "https://github.com/o/r.git",
				worktrees: []*git.Worktree{{Path: "/elsewhere/main", Branch: "main"}},
			},
		},
		{
			name: "a local branch is checked out as it is",
			arg:  "feat/x",
			hub:  ".bare",
			git: &stubGit{
				originURL: "https://github.com/o/r.git",
				hasBranch: true,
			},
			wantBranch: "feat/x",
		},
		{
			name: "a branch only on origin is fetched and tracked",
			arg:  "topic",
			hub:  ".bare",
			git: &stubGit{
				originURL: "https://github.com/o/r.git",
				hasBranch: false,
			},
			wantBranch: "topic",
			wantFetch:  "topic",
		},
		{
			name:       "a pull request resolves to its head branch",
			arg:        "42",
			hub:        ".bare",
			git:        &stubGit{originURL: "https://github.com/o/r.git", hasBranch: true},
			pr:         &github.PullRequest{HeadBranch: "topic"},
			wantBranch: "topic",
		},
		{
			name:       "a pull request from a fork is fetched through refs/pull",
			arg:        "https://github.com/o/r/pull/42",
			hub:        ".bare",
			git:        &stubGit{},
			pr:         &github.PullRequest{HeadBranch: "their-topic", CrossRepository: true},
			wantBranch: "pr-42",
			wantFetch:  "refs/pull/42/head:pr-42",
		},
		{
			name:       "a normal clone can host the worktree too",
			arg:        "topic",
			hub:        "",
			git:        &stubGit{originURL: "https://github.com/o/r.git", hasBranch: true},
			wantBranch: "topic",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			hub := tt.hub
			root, base := setupRoot(t, hub)
			if tt.hub == "" && tt.wantBranch != "" {
				// The "normal clone" case: the hub is <repo>/.git rather than the
				// conventional one, and the worktree still goes to the conventional path.
				if err := os.MkdirAll(filepath.Join(base, ".git"), 0o755); err != nil {
					t.Fatal(err)
				}
			}

			stdout := &bytes.Buffer{}
			c := New(&Input{
				Git: tt.git,
				NewGitHub: func(_ context.Context, _ *slog.Logger) (GitHub, error) {
					return &stubGitHub{pr: tt.pr}, nil
				},
				Stdout: stdout,
			})

			err := c.Run(t.Context(), newLogger(), &InputAdd{Arg: tt.arg, Root: root, Dir: "/cwd"})
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Run() error = %v, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Run() error: %v", err)
			}

			if tt.wantBranch == "" {
				// The existing-worktree case: git's path is printed and nothing is created.
				if got := strings.TrimSpace(stdout.String()); got != "/elsewhere/main" {
					t.Errorf("stdout = %q, want %q", got, "/elsewhere/main")
				}
				if tt.git.added != nil {
					t.Errorf("a worktree was created: %v", tt.git.added)
				}
				return
			}

			dst := filepath.Join(base+"+worktrees", tt.wantBranch)
			if got, want := strings.TrimSpace(stdout.String()), wantPath(t, dst); got != want {
				t.Errorf("stdout = %q, want %q", got, want)
			}
			if tt.git.fetched != tt.wantFetch {
				t.Errorf("fetched = %q, want %q", tt.git.fetched, tt.wantFetch)
			}
			wantAdd := tt.wantAdd
			if wantAdd == nil {
				wantAdd = []string{dst, tt.wantBranch}
				if tt.wantFetch != "" && !strings.HasPrefix(tt.wantFetch, "refs/pull/") {
					wantAdd = []string{"--track", "-b", tt.wantBranch, dst, "origin/" + tt.wantBranch}
				}
			}
			if diff := cmp.Diff(wantAdd, tt.git.added); diff != "" {
				t.Errorf("worktree add args mismatch (-want +got):\n%s", diff)
			}
			// The parent of a branch name holding a slash has to exist before git runs.
			if _, err := os.Stat(filepath.Dir(dst)); err != nil {
				t.Errorf("the parent directory was not created: %v", err)
			}
		})
	}
}

func TestController_Run_occupied(t *testing.T) {
	t.Parallel()
	root, base := setupRoot(t, ".bare")
	dst := filepath.Join(base+"+worktrees", "main")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}

	c := New(&Input{
		Git:    &stubGit{originURL: "https://github.com/o/r.git", hasBranch: true},
		Stdout: io.Discard,
	})
	err := c.Run(t.Context(), newLogger(), &InputAdd{Arg: "main", Root: root, Dir: "/cwd"})
	if !errors.Is(err, errOccupied) {
		t.Fatalf("Run() error = %v, want %v", err, errOccupied)
	}
}

func TestCloneHint(t *testing.T) {
	t.Parallel()
	// The hint has to be a command that can be pasted, so it names the repository
	// and the destination wt will look in next time.
	got := cloneHint("/root/github.com/o/r")
	want := "git clone --bare https://github.com/o/r.git /root/github.com/o/r+worktrees/.bare"
	if got != want {
		t.Errorf("cloneHint() = %q, want %q", got, want)
	}
}
