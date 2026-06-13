# Contributing

Thanks for considering a contribution. This is a small, focused tool — the bar
is "stays small, stays fast, stays dependency-free."

## Ground rules

- **Standard library only.** The whole point is a single static binary with no
  external dependencies. A PR that adds a `require` to `go.mod` will be asked to
  justify it heavily, and the answer is almost always "no."
- **One owner per concept.** Each file owns one layer (see the layout in the
  [README](README.md#development)). New code goes where the concept lives — the
  API shapes in `client.go`, output in `render.go`, dispatch in `main.go`.
- **Keep the safety guards.** Destructive operations require an explicit `--yes`;
  don't add paths around that.

## Development loop

```sh
go vet ./...
go test -race ./...
go build -o hetzner .
```

All three must pass before you open a PR — CI runs exactly this on Linux, macOS,
and Windows. Add or update tests for any behavior you change; the suite is
`httptest`-backed and runs without network access or a real token.

## Pull requests

1. Fork and branch off `main`.
2. Make the change, with tests.
3. Run the loop above.
4. Open a PR with a short description of *what* and *why*. Keep the diff scoped
   to one concern.

## Reporting bugs

Open an issue with the command you ran, what you expected, and what happened.
`hetzner version` and your OS/arch help. **Never paste an API token** — redact
it. For anything security-related, follow [SECURITY.md](SECURITY.md) instead.
