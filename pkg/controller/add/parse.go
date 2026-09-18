package add

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"

	"github.com/suzuki-shunsuke/slog-error/slogerr"
)

// prURLPattern matches a pull request URL and captures the owner, the repository
// name, and the number. The host is matched loosely so that both the HTTPS and
// the SSH form of a URL are accepted.
var prURLPattern = regexp.MustCompile(`github\.com[:/]+([^/]+)/([^/]+?)(?:\.git)?/pull/(\d+)`)

// originPattern captures the owner and the repository name from the URL of a
// remote, in either the HTTPS or the SSH form.
var originPattern = regexp.MustCompile(`github\.com[:/]+([^/]+)/([^/]+?)(?:\.git)?/?$`)

// errNoRepo is returned when the repository cannot be determined from the
// working directory, which is what happens outside a git repository or in one
// without an origin remote.
var errNoRepo = errors.New("determine the repository from the current directory. Run wt in a repository, or pass a pull request URL")

// target is what the positional argument resolved to.
type target struct {
	owner string
	repo  string
	// branch is the branch to check out. It is empty when the argument named a
	// pull request, whose head branch is only known after the API call.
	branch string
	// prNumber is the pull request to resolve, or 0 when the argument named a
	// branch.
	prNumber int
}

// parseArg resolves the positional argument into a repository and either a
// branch or a pull request number.
//
// A URL names its own repository. Anything else is relative to the repository
// the command was run in, and is read as a pull request number when it is all
// digits: `wt add 1234` is the common case, and a branch named "1234" can still
// be reached through its URL. The ambiguity is resolved this way round because a
// numeric branch name is rare and a pull request number is typed constantly.
func parseArg(ctx context.Context, gitClient Git, arg, dir string) (*target, error) {
	if m := prURLPattern.FindStringSubmatch(arg); m != nil {
		n, err := strconv.Atoi(m[3])
		if err != nil {
			// The pattern only matches digits, so this cannot happen for any number the
			// API would accept; an overflowing one lands here.
			return nil, fmt.Errorf("parse the pull request number: %w", slogerr.With(err, "arg", arg))
		}
		return &target{owner: m[1], repo: m[2], prNumber: n}, nil
	}

	owner, repo, err := repoFromDir(ctx, gitClient, dir)
	if err != nil {
		return nil, err
	}
	if n, err := strconv.Atoi(arg); err == nil && n > 0 {
		return &target{owner: owner, repo: repo, prNumber: n}, nil
	}
	return &target{owner: owner, repo: repo, branch: arg}, nil
}

// repoFromDir reads the owner and the repository name from the origin remote of
// the repository containing dir.
func repoFromDir(ctx context.Context, gitClient Git, dir string) (string, string, error) {
	url, err := gitClient.OriginURL(ctx, dir)
	if err != nil {
		return "", "", fmt.Errorf("%w: %w", errNoRepo, err)
	}
	m := originPattern.FindStringSubmatch(url)
	if m == nil {
		//nolint:wrapcheck // The error is this package's own sentinel, only annotated.
		return "", "", slogerr.With(errNoRepo, "origin_url", url)
	}
	return m[1], m[2], nil
}
