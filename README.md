# wt

Create git worktrees at a conventional path and print where they are.

`wt add` takes a branch name, a pull request number, or a pull request URL, creates the worktree if it isn't there yet, and writes its absolute path to stdout — so it composes with `cd`:

```sh
cd "$(wt add 1234)"
```

## :warning: The problem

Worktrees are the right tool for working on several branches at once, but they leave you to decide where each one goes:

- `git worktree add ../myrepo-fix-checksum` scatters worktrees next to the repository, where they are indistinguishable from unrelated clones.
- Getting to one means remembering the path you invented weeks ago.
- Checking out someone's pull request is a `gh pr view`, a `git fetch`, and a `git worktree add` — and twice as much for a pull request from a fork.

## :white_check_mark: What wt does

Every worktree of a repository lives under one directory, named after its branch:

```
<root>/github.com/<owner>/<repo>+worktrees/
├── .bare                 the bare repository, when the repository is cloned that way
├── main
├── fix-checksum
└── feat/x                a branch name holding a slash simply nests
```

- `<repo>+worktrees` sits beside `<repo>`, so a tool that lists repositories under the owner directory keeps working.
- The hub is `.bare`: a leading dot keeps it out of `ls` and out of those listings, so everything you see is a directory you can work in. It cannot collide with a branch name either, because git rejects a ref whose component starts with a dot.
- Paths are derived, never invented, so `wt add` is the only thing you have to remember.

## :rocket: Getting started

**1. Install**

```sh
go install github.com/suzuki-shunsuke/wt/cmd/wt@latest
```

**2. Create the configuration file**

```sh
wt init
```

It writes `$XDG_CONFIG_HOME/wt/wt.yaml` (or `~/.config/wt/wt.yaml`) with one setting, the directory your repositories live under:

```yaml
root: ~/repos/src
```

Edit `root` if your repositories are elsewhere. A repository is expected at `<root>/github.com/<owner>/<repo>`, which is the layout [ghq](https://github.com/x-motemen/ghq) uses.

**3. Add a worktree**

```sh
# a branch of the repository you are in
cd "$(wt add feat/x)"

# a pull request of the repository you are in
cd "$(wt add 1234)"

# a pull request of any repository
cd "$(wt add https://github.com/suzuki-shunsuke/wt/pull/1)"
```

## How the argument is read

1. A pull request URL names its own repository.
2. All digits is a pull request number of the repository you are in.
3. Anything else is a branch name of the repository you are in.

A branch named with digits can still be reached through its URL.

A branch that exists only on origin is fetched first. A pull request from a fork is fetched through `refs/pull/<number>/head` and checked out as `pr-<number>`, because its head branch name belongs to the fork's namespace and may collide with a branch of yours.

If the branch already has a worktree, `wt add` prints its path and creates nothing, so it is safe to run again — including for a worktree that predates this layout.

## Authentication

Reading a pull request needs a GitHub token. `wt` uses `GITHUB_TOKEN` when it is set, and otherwise asks [ghtkn](https://github.com/suzuki-shunsuke/ghtkn) for a short-lived GitHub App user access token.

`GH_TOKEN` is deliberately not read: it belongs to the `gh` CLI, and borrowing another program's credentials should be a decision you make, not a default.

Adding a worktree for a branch needs no token at all.

## Migrating an existing repository

If your worktrees are somewhere else already, move them with `git worktree move` — never `mv`, which leaves both the repository and the worktree pointing at the old path:

```sh
R=~/repos/src/github.com/suzuki-shunsuke/wt+worktrees
mkdir -p "$R"
git -C <hub> worktree move <old-path> "$R/<branch>"
```

To move the hub itself, move it and then repair the links:

```sh
mv <old-hub> "$R/.bare"
git -C "$R/.bare" worktree repair
```

`wt add` works before you migrate: it finds the hub at `<repo>+worktrees/.bare`, `<repo>/.git`, or `<repo>.git`, and creates the new worktree at the conventional path regardless.

## License

[MIT](LICENSE)
