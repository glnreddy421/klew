# Klew desktop (Wails + React)

Native investigation UI for live Kubernetes incident analysis.

## Prerequisites

- Go 1.22+
- Node.js 18+
- Wails v2 CLI on your PATH

Install Wails once (from repo root):

```bash
make install-desktop-tools
export PATH="$(go env GOPATH)/bin:$PATH"
```

Add that `export` line to `~/.zshrc` so `wails` is always available.

Verify: `wails doctor`

## Development

```bash
make desktop-dev
```

Or manually:

```bash
cd cmd/klew-desktop
npm install --prefix frontend
wails dev
```

## Production build

```bash
make desktop
# → cmd/klew-desktop/build/bin/Klew.app (macOS)
```

## Architecture

- `internal/service` — live investigation backend
- `internal/api` — JSON view consumed by React
- `cmd/klew-desktop` — desktop shell (`brew tap glnreddy421/klew && brew install klew`)
