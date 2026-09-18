package add

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/suzuki-shunsuke/wt/pkg/git"
)

// stubGit answers only what parseArg asks for. The rest of the interface panics,
// so a test that starts exercising it has to say what it expects rather than
// silently getting a zero value.
type stubGit struct {
	originURL string
	originErr error

	worktrees []*git.Worktree
	hasBranch bool
	fetchErr  error
	addErr    error

	// fetchRefspec is what origin already has configured. Empty is a hub created
	// by `git clone --bare`, which has none.
	fetchRefspec string

	fetched    string
	added      []string
	refspecSet bool
}

func (s *stubGit) FetchRefspec(_ context.Context, _ string) string { return s.fetchRefspec }

func (s *stubGit) SetDefaultFetchRefspec(_ context.Context, _ string) error {
	s.refspecSet = true
	return nil
}

func (s *stubGit) OriginURL(_ context.Context, _ string) (string, error) {
	return s.originURL, s.originErr
}

func (s *stubGit) Worktrees(_ context.Context, _ string) ([]*git.Worktree, error) {
	return s.worktrees, nil
}

func (s *stubGit) HasBranch(_ context.Context, _, _ string) bool { return s.hasBranch }

func (s *stubGit) Fetch(_ context.Context, _, refspec string) error {
	s.fetched = refspec
	return s.fetchErr
}

func (s *stubGit) AddWorktree(_ context.Context, _ string, args ...string) error {
	s.added = args
	return s.addErr
}

func TestParseArg(t *testing.T) { //nolint:funlen // The length is the table of cases.
	t.Parallel()
	tests := []struct {
		name      string
		arg       string
		originURL string
		originErr error
		want      *target
		wantErr   string
	}{
		{
			name: "an HTTPS pull request URL names its own repository",
			arg:  "https://github.com/suzuki-shunsuke/wt/pull/42",
			// The origin of the current directory must be ignored for a URL, so it is
			// set to a different repository here.
			originURL: "https://github.com/other/other.git",
			want:      &target{owner: "suzuki-shunsuke", repo: "wt", prNumber: 42},
		},
		{
			name: "an SSH pull request URL is accepted",
			arg:  "git@github.com:suzuki-shunsuke/wt.git/pull/42",
			want: &target{owner: "suzuki-shunsuke", repo: "wt", prNumber: 42},
		},
		{
			name:      "digits are a pull request of the current repository",
			arg:       "1234",
			originURL: "https://github.com/o/r.git",
			want:      &target{owner: "o", repo: "r", prNumber: 1234},
		},
		{
			name:      "anything else is a branch of the current repository",
			arg:       "feat/x",
			originURL: "git@github.com:o/r.git",
			want:      &target{owner: "o", repo: "r", branch: "feat/x"},
		},
		{
			name: "a branch that only looks numeric is still a branch when negative",
			// -1 is not a pull request number, so it stays a branch name rather than
			// being turned into one that cannot exist.
			arg:       "-1",
			originURL: "https://github.com/o/r",
			want:      &target{owner: "o", repo: "r", branch: "-1"},
		},
		{
			name:      "no origin is reported",
			arg:       "main",
			originErr: errors.New("no such remote"),
			wantErr:   "determine the repository from the current directory",
		},
		{
			name:      "an origin outside GitHub is reported",
			arg:       "main",
			originURL: "https://gitlab.com/o/r.git",
			wantErr:   "determine the repository from the current directory",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := &stubGit{originURL: tt.originURL, originErr: tt.originErr}
			got, err := parseArg(t.Context(), g, tt.arg, "/cwd")
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("parseArg() error = %v, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseArg() error: %v", err)
			}
			if diff := cmp.Diff(tt.want, got, cmp.AllowUnexported(target{})); diff != "" {
				t.Errorf("parseArg() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
