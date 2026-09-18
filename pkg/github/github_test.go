package github_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	gogithub "github.com/google/go-github/v92/github"
	"github.com/suzuki-shunsuke/wt/pkg/github"
)

// stubPRs returns a canned pull request.
type stubPRs struct {
	pr  *gogithub.PullRequest
	err error
}

func (s *stubPRs) Get(_ context.Context, _, _ string, _ int) (*gogithub.PullRequest, *gogithub.Response, error) {
	return s.pr, nil, s.err
}

// baseRepo is the repository every case in this file opens its pull request
// against. Only the head side varies, which is what decides CrossRepository.
const baseRepo = "o/r"

// pr builds a pull request against baseRepo whose head is headRepo:branch. An
// empty headRepo leaves the head repository unset, as the API reports it once a
// fork has been deleted.
func pr(branch, headRepo string) *gogithub.PullRequest {
	base := baseRepo
	p := &gogithub.PullRequest{
		Head: &gogithub.PullRequestBranch{Ref: &branch},
		Base: &gogithub.PullRequestBranch{
			Repo: &gogithub.Repository{FullName: &base},
		},
	}
	if headRepo != "" {
		p.Head.Repo = &gogithub.Repository{FullName: &headRepo}
	}
	return p
}

func TestClient_GetPR(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                string
		pr                  *gogithub.PullRequest
		err                 error
		wantBranch          string
		wantCrossRepository bool
		wantErr             string
	}{
		{
			name:       "a branch of the same repository",
			pr:         pr("topic", "o/r"),
			wantBranch: "topic",
		},
		{
			name:                "a branch of a fork",
			pr:                  pr("topic", "fork/r"),
			wantBranch:          "topic",
			wantCrossRepository: true,
		},
		{
			name: "a deleted fork is still cross-repository",
			// The head repository is gone, which happens to a fork after the pull
			// request is merged. Treating it as the same repository would send wt
			// looking for a branch the base repository never had.
			pr:                  pr("topic", ""),
			wantBranch:          "topic",
			wantCrossRepository: true,
		},
		{
			name:    "an API failure is reported",
			err:     errors.New("404 Not Found"),
			wantErr: "get a pull request",
		},
		{
			name:    "a pull request without a head branch is reported",
			pr:      pr("", "o/r"),
			wantErr: "the pull request has no head branch",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := github.New(&stubPRs{pr: tt.pr, err: tt.err}).GetPR(t.Context(), "o", "r", 42)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("GetPR() error = %v, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("GetPR() error: %v", err)
			}
			if got.HeadBranch != tt.wantBranch {
				t.Errorf("HeadBranch = %q, want %q", got.HeadBranch, tt.wantBranch)
			}
			if got.CrossRepository != tt.wantCrossRepository {
				t.Errorf("CrossRepository = %v, want %v", got.CrossRepository, tt.wantCrossRepository)
			}
		})
	}
}

func TestToken(t *testing.T) {
	t.Parallel()
	// GITHUB_TOKEN is used as it is, without reaching for ghtkn, which would try to
	// authenticate. GH_TOKEN is deliberately not read: it belongs to the gh CLI.
	got, err := github.Token(t.Context(), nil, func(k string) string {
		return map[string]string{"GITHUB_TOKEN": "tkn", "GH_TOKEN": "gh-tkn"}[k]
	})
	if err != nil {
		t.Fatalf("Token() error: %v", err)
	}
	if got != "tkn" {
		t.Errorf("Token() = %q, want %q", got, "tkn")
	}
}
