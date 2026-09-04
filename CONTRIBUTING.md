# Contributing

## Design principles

- **Zero runtime dependencies.** `pty` uses only the Go standard library and
  hand-maintained platform syscalls. Before adding an external dependency,
  prefer a `//go:build`-scoped helper; revisit the dependency decision only if
  a platform port becomes impossible to maintain by hand.
- **Keep the API small and platform-consistent.** The public surface is
  frozen for v1: `Pty`, `Winsize`, `Open`, `Start`, `StartWithSize`, `SetSize`,
  `GetSize`, `IsTerminal`, and `EnableVirtualTerminal`. Do not grow terminal
  line discipline (raw mode, echo, and friends) into this package; that scope
  belongs to consumers.
- **Honor the platform matrix in the README.** A capability may degrade
  gracefully (for example Windows `GetSize` returns a client-side snapshot),
  but it must behave as documented and never pretend to be something else.
- **Unsupported platforms compile.** `plan9`, `js/wasm`, `wasip1`, and similar
  targets must build so `Start` and friends can return
  `errors.ErrUnsupported`. Cross-build regressions are caught in CI.

## Verifying changes

Run the usual checks first:

```sh
go fmt ./...
go vet ./...
go test -race -count=1 ./...
```

Windows-only behavior (ConPTY, job objects, command-line construction) can
only be compiled here, so compile the Windows test binary too:

```sh
GOOS=windows GOARCH=amd64 go test -c -o /dev/null ./...
GOOS=windows GOARCH=arm64 go test -c -o /dev/null ./...
```

CI additionally cross-builds every supported GOOS/GOARCH, compiles the
unsupported-platform fallbacks, and runs the FreeBSD suite through Cirrus CI.

## Releasing

The library is a plain Go module: releases are version tags pushed to the
origin (for example `v1.0.0`), not binary artifacts. Before tagging:

1. All CI jobs pass on Linux, macOS, Windows, and FreeBSD.
2. `go vet ./...` and `go test -race ./...` pass.
3. The cross-build job in `.github/workflows/ci.yml` is green.

After the first v1 tag, public API changes require a new major version.
