# Contributing

## Development

The tasks are defined in [cmdx.yaml](cmdx.yaml) and run with [cmdx](https://github.com/suzuki-shunsuke/cmdx). The tools they need are pinned in [aqua](aqua/).

```sh
cmdx t   # test
cmdx v   # go vet
cmdx l   # golangci-lint run
cmdx i   # go install
cmdx fmt # gofumpt and nllint
```

## Package layout

```
cmd/wt/main.go              the entry point, which only calls cobrautil.Main
pkg/cli/                    the CLI interface and DI: flags, args, wiring
pkg/cli/<name>/             one package per subcommand
pkg/controller/<name>/      the core logic behind a subcommand
pkg/{config,git,github}/    supporting packages the controllers call into
```

The dependency runs one way: `cli` builds what `controller` needs and hands it over; `controller` never reaches back.

## Conventions

- **Resolve the environment in the CLI layer.** Environment variables, the working directory, and configuration paths are read in `pkg/cli` and passed to the controller as settled values, so the controller has nothing to stub but its own inputs.
- **Declare interfaces where they are consumed.** A controller defines the smallest interface it needs rather than importing a concrete client, which is what makes the tests free of the network and of git.
- **Logging**: use [slog](https://pkg.go.dev/log/slog) and [slog-error](https://github.com/suzuki-shunsuke/slog-error). A controller takes a `*slog.Logger`; only the CLI layer knows about `slogutil`.
- **Errors**: wrap with `fmt.Errorf("<verb in lowercase>: %w", err)` — "get a pull request", not "failed to get a pull request". Attach structured attributes with `slogerr.With`. An error that already describes itself is returned as it is with `//nolint:wrapcheck`.
- **stdout carries the answer, nothing else.** `wt add` prints one path, because it is meant to be used as `cd "$(wt add ...)"`. Everything else goes to the logger, which writes to stderr.
- **Comments say why.** What the code does is in the code; a comment that repeats it earns nothing.

## Tests

- **Don't use testify.** Use [google/go-cmp](https://github.com/google/go-cmp), with plain `if ... { t.Errorf(...) }` assertions.
- Internal tests go in `<name>_internal_test.go` with `package foo`; everything else is `package foo_test`.
- Table-driven, with `t.Parallel()` on both the parent and the subtests. When a test cannot run in parallel, say why in a `//nolint:paralleltest` comment.
- No `testdata/` and no golden files: `t.TempDir()` and inline expectations keep a case readable in one place.
- Assert on the error message, not just on the fact that there was an error.

## Lint

[.golangci.yml](.golangci.yml) enables every linter and disables the ones that do not pay for themselves, so expect `wrapcheck`, `funlen`, `mnd`, and friends to have opinions. Every `//nolint` needs a reason after it.
