# Contributing to Klew

Bug reports, ideas, documentation fixes, and code are all welcome. This guide covers how to report a problem, how to build and test a change, and what a pull request needs to pass.

Everyone taking part is expected to follow the [Code of Conduct](CODE_OF_CONDUCT.md).

## Ways to help

| I want to… | Go to |
| --- | --- |
| Report a bug | [Issues](https://github.com/glnreddy421/klew/issues/new) |
| Suggest a feature or ask a question | [Discussions](https://github.com/glnreddy421/klew/discussions) |
| Report a security problem | [SECURITY.md](SECURITY.md). Report it privately, not in a public issue |
| Add my own checks without changing Klew | [docs/extensions.md](docs/extensions.md): collectors and rules as YAML, no rebuild |

### A good bug report

- The Klew version (`klew version`) and how you installed it.
- macOS or Linux version, and the Kubernetes version and distribution (EKS, GKE, kind, …).
- What you did, what you expected, and what happened instead.
- For a wrong or missing verdict: the workload's state as `kubectl` shows it, and what Klew reported.

`klew investigate` output and screenshots are **not redacted**. Remove names, log lines, and anything else you cannot share before posting.

## Before you write code

For anything beyond a small fix, open an issue or discussion first so the approach is agreed before you spend time on it.

### Read-only is not negotiable

Klew never changes a cluster, and a change that breaks this will not be merged. It applies to the desktop app's Resources screen, `klew investigate`, and `klew mcp`:

- **Allowed:** list, get, watch, discovery, reading metrics, logs and manifests, and reading local event history.
- **Never:** create, update, patch, delete, apply, scale, exec, port-forward, or anything equivalent, in an API or in the UI.
- **Suggested actions are text.** Klew prints commands for a person to review and never runs them.
- **No Secret values**, anywhere: not in the UI, the terminal, or MCP results.

The desktop app's embedded terminal is a shell the user drives. It is not part of how Klew reads the cluster.

### Extra rules for the MCP server

Results from `klew mcp` end up in an AI provider's context, so `internal/mcpserver` has a few more:

- Every tool sets `readOnlyHint: true` and `destructiveHint: false`. `TestToolsAreReadOnly` enforces it.
- Every free-text field from the cluster (logs, events, messages) goes through `clean` in `internal/mcpserver/shape.go`, which redacts and then truncates.
- Any new field derived from logs must honour `-no-log-samples`.
- Keep results compact: cap lists and truncate long text.
- Logs go to stderr only. stdout carries the MCP protocol.
- Describe redaction as best effort, in code comments and docs alike. Never claim results are free of sensitive data.

## Set up

You need Go 1.26+. The desktop app also needs Node 24 and [Wails v2](https://wails.io), and builds on macOS only. On Linux you can work on the engine, `klew investigate` and `klew mcp`, but not the desktop app.

```bash
git clone https://github.com/<you>/klew.git    # your fork
cd klew

# klew command (CLI and MCP server)
go build -o klew ./cmd/klew

# Desktop app, with live reload
go install github.com/wailsapp/wails/v2/cmd/wails@latest
cd cmd/klew-desktop
wails dev
```

The [README](README.md#layout) has a map of the repository.

## Make the change

Run the same checks CI runs before you push:

```bash
# Go: engine and klew command
gofmt -l .                      # prints files that need gofmt -w
go mod tidy -diff
go vet ./internal/... ./cmd/klew/... ./cmd/klew-stress/...
go test -race -count=1 ./internal/... ./cmd/klew/... ./cmd/klew-stress/...

# Log engine: a separate Go module
(cd tools/k8s-log-engine && go mod tidy -diff && go vet ./... && go test -race -count=1 ./...)

# Desktop UI
cd cmd/klew-desktop/frontend
npm ci
npm test
npm run build
```

`go vet ./...` and `go test ./...` also cover the desktop app. It embeds `cmd/klew-desktop/frontend/dist`, so build the frontend first.

Lint uses `.golangci.yml` and judges only the code your branch adds or changes. CI runs golangci-lint v2.15.0; use the same version so local results match:

```bash
golangci-lint run --new-from-merge-base=origin/main
```

### Desktop app bindings

The frontend calls Go through generated Wails bindings in `cmd/klew-desktop/frontend/wailsjs/`. When you add or change an exported method on `App` or a type it returns, regenerate them and commit the result:

```bash
cd cmd/klew-desktop
wails generate module
git checkout -- frontend/wailsjs/runtime   # the generator only flips file modes here
```

Commit the changes under `wailsjs/go/` with the Go change. New `App` methods follow the read-only rule above.

Along with the code:

- **Tests.** Add or update tests for what you changed. A bug fix should come with a test that fails without it.
- **Docs.** When behaviour, a flag, or a tool changes, update `README.md` and the matching guide in `docs/` in the same pull request.
- **Changelog.** Add an entry under `[Unreleased]` in [CHANGELOG.md](CHANGELOG.md), in the style already there: a bold lead, then what changed for the user.

## Pull requests

1. Fork the repository and branch from `main`.
2. Keep the pull request to one change. Separate refactors from behaviour changes.
3. Write commit subjects in the imperative, without a trailing period, saying what the commit does: `Reject a missing -extensions-dir in klew mcp`.
4. Open the pull request against `main` and describe what changed, why, and how you tested it.

`main` only takes changes through pull requests, and these checks must pass before merging:

| Check | What it covers |
| --- | --- |
| Go tests (macos-latest), Go tests (ubuntu-latest) | `go mod tidy`, `gofmt` on changed files, `go vet`, tests with the race detector, and a `klew mcp` smoke test |
| Frontend tests and build | `npm test`, `npm run build`, `npm audit` for high and critical advisories |
| Go lint (new issues) | `golangci-lint`, including `gosec`, on new code |
| Workflow and script lint | `actionlint` and `shellcheck` |
| govulncheck, Trivy, Dependency review | Known vulnerabilities, secrets, misconfiguration, and new dependencies |
| CodeQL: Analyze (go), (javascript-typescript), (actions) | Static analysis |

If you add a GitHub Action, pin it to a commit SHA, as the existing workflows do.

## License

Klew is licensed under [Apache 2.0](LICENSE). By contributing, you agree that your contributions are licensed under the same terms.
