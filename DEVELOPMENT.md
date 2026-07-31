# Local development

How to work on Klew day to day. User-facing install docs stay in [README.md](README.md).

## Repos

| Repo | Purpose |
|------|---------|
| **[klew](https://github.com/glnreddy421/klew)** | Desktop app (Go + Wails/React) — main codebase |
| **[klew-ui](https://github.com/glnreddy421/klew-ui)** | Marketing site (static HTML, Firebase Hosting) |
| **[homebrew-klew](https://github.com/glnreddy421/homebrew-klew)** | Homebrew tap — updated automatically on release |

Clone side by side if you work on everything:

```bash
git clone git@github.com:glnreddy421/klew.git
git clone git@github.com:glnreddy421/klew-ui.git
git clone git@github.com:glnreddy421/homebrew-klew.git
```

## Prerequisites (once)

- **macOS** (Apple Silicon) for the desktop app
- **Go 1.22+** — `go version`
- **Node.js 18+** — for the React frontend
- **Wails v2** — from repo root:

```bash
cd klew
make install-desktop-tools
echo 'export PATH="$(go env GOPATH)/bin:$PATH"' >> ~/.zshrc
source ~/.zshrc
wails doctor
```

Optional for live cluster testing: a working `~/.kube/config`.

## Daily app development

From **klew** repo root:

```bash
make desktop-dev    # Wails dev server + hot reload (primary loop)
go test ./...       # backend tests
go build ./...      # compile check
```

- **Go / engine / kube:** `internal/`
- **Desktop shell + bindings:** `cmd/klew-desktop/`
- **React UI:** `cmd/klew-desktop/frontend/src/`

`wails dev` rebuilds Go bindings when exported `App` methods or bound structs change. After changing types in `internal/model` or `internal/api`, restart dev if the frontend acts stale.

### Try changes without a cluster

1. Run the app (`make desktop-dev` or open a built `Klew.app`)
2. Welcome screen → **Demo** → pick `payment`, `vault`, or `checkout`

### Production build (local)

```bash
make desktop
# → cmd/klew-desktop/build/bin/Klew.app

open cmd/klew-desktop/build/bin/Klew.app
```

Signed release packages (DMG/ZIP) — needs Apple Developer secrets, normally CI only:

```bash
make release-macos VERSION=0.1.4
```

## Marketing site (klew-ui)

```bash
cd klew-ui
python3 -m http.server 5173
# http://localhost:5173
```

Deploy (after `firebase login --reauth`):

```bash
firebase deploy --only hosting
```

`site.js` pulls the latest GitHub Release for download links — no manual version bump on the site when you ship a tag.

## Homebrew (local formula test)

After building or releasing, test the tap formula without publishing:

```bash
cd klew
brew install ./packaging/homebrew/klew.rb
open "$(brew --prefix)/opt/klew/Klew.app"
```

The live tap at `glnreddy421/klew` is bumped by CI; edit `homebrew-klew/Formula/klew.rb` only when fixing the formula itself.

## Release (maintainers)

1. Merge to `main`
2. Tag and push:

```bash
git tag v0.1.5
git push origin v0.1.5
```

GitHub Actions then:

- Builds, signs, and notarizes macOS DMG + ZIP
- Publishes [GitHub Release](https://github.com/glnreddy421/klew/releases)
- Updates **homebrew-klew** (requires `HOMEBREW_TAP_TOKEN` on the klew repo)

No manual Homebrew sha256 bump needed when CI secrets are configured.

## Useful Makefile targets

| Target | What it does |
|--------|----------------|
| `make desktop-dev` | Hot-reload desktop app |
| `make desktop` | Production `Klew.app` build |
| `make test` | `go test ./...` |
| `make release-macos` | DMG + ZIP (local or CI) |
| `make tidy` | `go mod tidy` |

## Project layout (quick)

```
cmd/klew-desktop/     Wails app + React frontend
internal/service/       investigation session for UI
internal/api/           View JSON for React
internal/engine/        evidence bus, reducer, live session
internal/kube/          cluster client
internal/investigation/ discovery + scope graph
internal/model/         shared state types (Wails-safe timestamps)
```
