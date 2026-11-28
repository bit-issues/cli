# bitissues-cli

`bitissues-cli` is the command-line client and read-only MCP server for BitIssues. It talks to the
BitIssues REST API through [`github.com/bit-issues/client-go`](https://github.com/bit-issues/client-go)
over HTTP; it does not import backend internals and does not touch a database.

## Commands

```
bitissues-cli auth login    # TTY password (echo off), writes one 0600 credentials.json
bitissues-cli auth logout   # revokes the stored token and deletes the file
bitissues-cli mcp           # serves six read-only tools over stdio JSON-RPC
```

There is no `auth status` command. `mcp` never prompts and never writes to stdout except protocol
frames: logs and errors go to stderr.

## Configuration

| Variable | Purpose |
|---|---|
| `BITISSUES_URL` | API base URL, **including** the `/api/v1` prefix (or `--base-url`) |
| `BITISSUES_TOKEN` | static access token; wins over the file and never refreshes |
| `BITISSUES_EMAIL` | optional default email for `auth login` (the password is never an env var) |
| `DEBUG`, `LOG_*`, `NO_COLOR` | template logging settings (loaded from `.env` before the logger) |

Credentials live at `$XDG_CONFIG_HOME/bitissues/credentials.json`, falling back to
`~/.config/bitissues/credentials.json`. The file is mode `0600` and written atomically.
`mcp` boots from `BITISSUES_TOKEN` first, then the file; with neither it exits non-zero with
`not logged in - run 'bitissues-cli auth login'` and emits no stdout. On a clean exit the current
tokens are saved again (save-on-exit), so a rotation survives; a crash can leave a stale file,
which surfaces as `session expired - log in again` and requires a fresh login.

## MCP tools (read-only)

`list_projects`, `get_project`, `list_tasks`, `get_task`, `list_task_comments`, `get_comment`.

All tools are annotated `readOnlyHint`/`idempotentHint`/`openWorldHint`. Every result has a text
content block and `structuredContent`; list tools echo `total`/`limit`/`offset` and a pagination
note when more pages exist. Long text is truncated with `[truncated]`, and presigned attachment
URLs are never returned. There are no write tools and no catch-all "call a method" tool.

## Claude Code registration

Local stdio is supported in **Claude Code and Cowork on the operator's machine**:

```sh
claude mcp add --transport stdio bitissues -- bitissues-cli mcp
```

Set `BITISSUES_URL` and either `BITISSUES_TOKEN` or a prior `auth login` in that environment.
The exact stdio `.mcp.json` schema is documented at `code.claude.com/docs/en/mcp`, not in the
general connector-building docs; consult that page before hand-editing a config file.

**Not supported:** adding this as a hosted connector for `claude.ai` chat - that surface has no
local stdio and no access to the operator's credentials file. A remote HTTP/Streamable-HTTP
transport and an MCPB/plugin packaging path are deferred items; no directory submission happens
in this wave.

Verify with MCP Inspector:

```sh
npx @modelcontextprotocol/inspector --cli bitissues-cli mcp --method tools/list
npx @modelcontextprotocol/inspector --cli bitissues-cli mcp \
  --method tools/call --tool-name list_projects --tool-arg limit=5
```

## Trust boundary

Reads are **unscoped**: any stored credential can read every project, task and comment, and the
model may echo that data to a cloud LLM. Run the MCP server only in trusted contexts. A
per-credential project allowlist is a deferred item.

## Development

```
make build    # bin/bitissues-cli
make fmt
make lint     # zero issues
make test
```

Local cross-repo development uses a gitignored `cli/go.work` with `use (. ../client-go)`.
`GOWORK=off go build ./...` verifies that the SHA-pinned client-go pseudo-version in `go.mod`
resolves from the module proxy. The module is never tagged; client-go is referenced by its
pushed commit SHA only.

Exit codes: `0` success, `1` bad params/config, `2` external failure, `3` output failure,
`4` internal failure.
