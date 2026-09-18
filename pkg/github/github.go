// Package github reads the pull request information wt needs from the GitHub API.
//
// wt talks to the API directly rather than shelling out to the gh CLI, so that it
// works wherever a token is available rather than only where gh is installed and
// logged in.
package github

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/go-github/v92/github"
	"github.com/suzuki-shunsuke/ghtkn-go-sdk/ghtkn"
	"github.com/suzuki-shunsuke/slog-error/slogerr"
)

// errNoHeadBranch is returned when the API reports a pull request without a head
// branch name. It should not happen, but the name is what wt names the worktree
// after, so an empty one must not reach the filesystem.
var errNoHeadBranch = errors.New("the pull request has no head branch")

// PullRequest is the part of a pull request wt acts on.
type PullRequest struct {
	// HeadBranch is the name of the branch the pull request was opened from.
	HeadBranch string
	// CrossRepository reports whether the head branch lives in a fork. Such a
	// branch is absent from the base repository's remote, so it has to be fetched
	// through refs/pull/<number>/head instead.
	CrossRepository bool
}

// PullRequests is the subset of go-github's pull request service wt uses.
// It is declared here, where it is consumed, so a test can supply a stub
// without a network round trip.
type PullRequests interface {
	Get(ctx context.Context, owner, repo string, number int) (*github.PullRequest, *github.Response, error)
}

// Client reads pull requests from GitHub.
type Client struct {
	prs PullRequests
}

// New creates a Client authenticated with a token from getToken.
func New(prs PullRequests) *Client {
	return &Client{prs: prs}
}

// NewClient creates a Client authenticated with the token from Token.
func NewClient(ctx context.Context, logger *slog.Logger, getenv func(string) string) (*Client, error) {
	token, err := Token(ctx, logger, getenv)
	if err != nil {
		return nil, err
	}
	client, err := github.NewClient(github.WithAuthToken(token))
	if err != nil {
		return nil, fmt.Errorf("create a GitHub client: %w", err)
	}
	return New(client.PullRequests), nil
}

// Token returns the GitHub access token to authenticate with.
//
// GITHUB_TOKEN wins when it is set, so CI and any environment that already
// exports a token needs no further setup. Otherwise the token comes from ghtkn,
// which issues short-lived GitHub App user access tokens; wt is a local
// development tool, and that is how the author's other tools authenticate.
//
// GH_TOKEN is deliberately not read: it belongs to the gh CLI, and wt reading it
// would silently borrow gh's credentials for a different program.
func Token(ctx context.Context, logger *slog.Logger, getenv func(string) string) (string, error) {
	if token := getenv("GITHUB_TOKEN"); token != "" {
		return token, nil
	}
	client, err := ghtkn.New()
	if err != nil {
		return "", fmt.Errorf("create a ghtkn client: %w", err)
	}
	token, _, err := client.Get(ctx, logger, &ghtkn.InputGet{})
	if err != nil {
		return "", fmt.Errorf("get a GitHub access token from ghtkn. Set GITHUB_TOKEN or configure ghtkn: %w", err)
	}
	return token.AccessToken, nil
}

// GetPR reads the pull request numbered number of owner/repo.
func (c *Client) GetPR(ctx context.Context, owner, repo string, number int) (*PullRequest, error) {
	pr, _, err := c.prs.Get(ctx, owner, repo, number)
	if err != nil {
		return nil, fmt.Errorf("get a pull request: %w", slogerr.With(err,
			"repo_owner", owner, "repo_name", repo, "pull_request", number))
	}
	head := pr.GetHead()
	branch := head.GetRef()
	if branch == "" {
		//nolint:wrapcheck // The error is this package's own sentinel, only annotated.
		return nil, slogerr.With(errNoHeadBranch, "repo_owner", owner, "repo_name", repo, "pull_request", number)
	}
	// The head repository is absent when it has been deleted, which happens to a
	// fork after the pull request is merged. Comparing full names rather than
	// trusting a missing repository keeps that case on the fork path, where the
	// branch is fetched through refs/pull/<number>/head and still exists.
	return &PullRequest{
		HeadBranch:      branch,
		CrossRepository: head.GetRepo().GetFullName() != pr.GetBase().GetRepo().GetFullName(),
	}, nil
}
