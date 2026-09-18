package git

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// stubRunner returns a canned response, and records the arguments it was called
// with so a test can assert on the git command that was built.
type stubRunner struct {
	out  string
	err  error
	args []string
	dir  string
}

func (s *stubRunner) Run(_ context.Context, dir string, args ...string) (string, error) {
	s.dir = dir
	s.args = args
	return s.out, s.err
}

func TestClient_Worktrees(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		out  string
		want []*Worktree
	}{
		{
			name: "a bare repository and its worktrees",
			// The bare entry has no branch line, and a detached one has "detached"
			// where the branch would be. Both must survive the parse without swallowing
			// the entry that follows.
			out: "worktree /repos/x+worktrees/.bare\nbare\n\n" +
				"worktree /repos/x+worktrees/main\nHEAD abc\nbranch refs/heads/main\n\n" +
				"worktree /repos/x+worktrees/detached\nHEAD def\ndetached\n\n" +
				"worktree /repos/x+worktrees/feat/y\nHEAD ghi\nbranch refs/heads/feat/y\n\n",
			want: []*Worktree{
				{Path: "/repos/x+worktrees/.bare"},
				{Path: "/repos/x+worktrees/main", Branch: "main"},
				{Path: "/repos/x+worktrees/detached"},
				{Path: "/repos/x+worktrees/feat/y", Branch: "feat/y"},
			},
		},
		{
			name: "no output yields no worktree",
			out:  "",
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := NewWithRunner(&stubRunner{out: tt.out})
			got, err := c.Worktrees(t.Context(), "/repos/x+worktrees/.bare")
			if err != nil {
				t.Fatalf("Worktrees() error: %v", err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Worktrees() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestClient_HasBranch(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "a zero exit means the branch exists", want: true},
		{name: "a non-zero exit means it does not", err: errors.New("exit status 1")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			runner := &stubRunner{err: tt.err}
			if got := NewWithRunner(runner).HasBranch(t.Context(), "/hub", "main"); got != tt.want {
				t.Errorf("HasBranch() = %v, want %v", got, tt.want)
			}
			// The ref has to be fully qualified, or show-ref would also match a tag.
			want := []string{"show-ref", "--verify", "--quiet", "refs/heads/main"}
			if diff := cmp.Diff(want, runner.args); diff != "" {
				t.Errorf("git args mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestClient_AddWorktree(t *testing.T) {
	t.Parallel()
	runner := &stubRunner{}
	if err := NewWithRunner(runner).AddWorktree(t.Context(), "/hub", "--track", "-b", "x", "/dst", "origin/x"); err != nil {
		t.Fatalf("AddWorktree() error: %v", err)
	}
	want := []string{"worktree", "add", "--track", "-b", "x", "/dst", "origin/x"}
	if diff := cmp.Diff(want, runner.args); diff != "" {
		t.Errorf("git args mismatch (-want +got):\n%s", diff)
	}
	if runner.dir != "/hub" {
		t.Errorf("dir = %q, want %q", runner.dir, "/hub")
	}
}

func TestClient_OriginURL(t *testing.T) {
	t.Parallel()
	// git ends its output with a newline, which must not reach the caller.
	runner := &stubRunner{out: "https://github.com/o/r.git\n"}
	got, err := NewWithRunner(runner).OriginURL(t.Context(), "/hub")
	if err != nil {
		t.Fatalf("OriginURL() error: %v", err)
	}
	if want := "https://github.com/o/r.git"; got != want {
		t.Errorf("OriginURL() = %q, want %q", got, want)
	}
}
