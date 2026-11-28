# bitissues-cli — AGENTS.md

## Commands (from `cli/`)
- `make deps` — go mod download
- `make gen` — go generate ./... (no generators)
- `make fmt` — golangci-lint fmt, then lint
- `make lint` — golangci-lint run --timeout=5m (strict golden config, keep at zero)
- `make test` — go test -race -shuffle=on -count=1 -covermode=atomic -coverpkg=./... ./...
- `make build` — binary to `bin/bitissues-cli`
- `GOWORK=off go build ./... && GOWORK=off go test ./...` — verify the pinned client-go pseudo-version

## Architecture
- Entrypoint `main.go`: godotenv BEFORE logger, go-core-fx/cli-logger, signal context,
  version flags, `internal.Run`, exit codes via `internal/cli/codes`, logger close.
  Two pre-existing `//nolint` directives in main.go are template metadata/urfave specific; add none.
- Root command `internal/app.go`: `bitissues-cli`, global `--debug`, `--base-url` (env `BITISSUES_URL`), no root action.
- Commands: `internal/cmd/` exposes `Commands()`, `Auth()` (children `login`/`logout`), `MCP()`;
  `internal/commands/commands.go` aggregates them. urfave/cli/v3, no fx.
- Config: `internal/config` is env-only (`BITISSUES_URL`, `BITISSUES_TOKEN`).
  The token is never a flag and is never logged.
- Credentials: `internal/credential` writes ONE 0600 `credentials.json` at
  `$XDG_CONFIG_HOME/bitissues/` (fallback `~/.config/bitissues/`) with an atomic temp+rename.
- MCP: `internal/mcp` serves six READ-ONLY tools over `mcpsdk.StdioTransport` on a
  `*clientgo.Session`; hand-built `*jsonschema.Schema` inputs, typed Out structs,
  structuredContent + text fallback, sanitized isError messages. stdout is SDK-only.
- Boot precedence: `BITISSUES_TOKEN` (static-style, wins) > credentials file > fail fast on stderr.
  `mcp` defers `save(session.Tokens())` (save-on-exit); a crash leaves a stale file -> ErrSessionExpired.

## Hard rules
- module `github.com/bit-issues/cli`; binary `bitissues-cli`; Go 1.25.7.
- client-go pinned by pushed SHA pseudo-version; gitignored `cli/go.work` with `use (. ../client-go)`;
  NO committed `replace`, NO tags.
- No new `//nolint`; never edit `.golangci.yml`; no testability seams/interfaces just for tests.
- External test packages + `export_test.go` where internals are needed; `t.Parallel()` on every
  test and subtest; source-order convention on every new file.
- Secrets: password TTY-only via `golang.org/x/term`; no token/password in argv, env (except the
  static token), logs, errors or tool output. No presigned `download_url`/`upload_url` in results.
- `internal/mcp` and `internal/cmd` production code must not write to stdout (guarded by a test).

## Verification
- `make lint`, `make test`, `GOWORK=off go build ./...`, stdout guard, golden search vectors,
  schema/struct drift guard, and the stdio smoke matrix in the task plan.
