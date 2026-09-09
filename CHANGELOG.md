# Changelog

All notable changes to Klew are documented here. Version tags follow [SemVer](https://semver.org/).

## [Unreleased]

## [2.0.0] — 2026-09-09

Major release: redesigned **Resources** workbench, shell inspector, terminal workspace, and cluster catalog — the “klew-ui” line.

### Resources workbench

- New **Resources** explorer with category navigation, lazy counts, and browse scope (cluster / namespace).
- **Entity tables** with kind-specific columns, column picker, and richer cell renderers (nodes, containers, deployment conditions, service endpoints).
- **Inspector panel** — slide-out detail view with manifest tab, relationship graph, and workload overview; starts collapsed on launch.
- Clickable inspect links (Node, ServiceAccount, etc.) with visible hover/focus states across themes.
- **Container inspect** — full image refs, container/image IDs, ports, command/args (no truncation).
- **Relationship graph** for workloads, services, ingress, and related objects.
- **Resource manifest** view via kubectl-equivalent fetch.

### Helm

- **Helm → Releases** — list releases from cluster Secrets with chart, revision, status, and updated time.
- Read-only release inspector (values summary, backing Secret ref); manifest resolves to the release Secret.
- Remove virtual **Helm Charts** tab (invalid manifest target).

### Terminal

- Per-tab **close** (×) on every tab, including single log-tail sessions.
- Right-click tab → **split** right or down when two or more tabs are open.
- **⌘F find** in terminal output (log tails and shells) with match navigation.

### Cluster connectivity

- Connection status dot and banner when the API is unreachable or credentials expire.
- Clearer connection error messages from the backend.

### Catalog & details (backend)

- Browse API with table fields, node enrichment, and builtin kind providers.
- Endpoints, webhook configurations, Helm release details, and generic unstructured body support.
- Virtual resource dispatch and kubectl manifest resolution for inspect targets.

## [0.1.10] — 2026-09-07

### Investigation UX

- Default to **Resources** when an investigation starts so you can browse the catalog immediately.
- Show a non-blocking “correlating in the background” banner while matches load; Overview still shows full progress if you switch there.

## [0.1.9] — 2026-09-07

### Investigation scope (large clusters)

- Searchable **context** and **namespace** pickers with type-to-filter and type-to-select.
- Namespace picker: Enter or “Use namespace …” when the list is incomplete or RBAC-limited.
- Scope picker: search resources, filter by kind, bulk actions (Workloads / Select visible / Clear).
- Smart default selection — top workload matches instead of checking every resource in huge namespaces.
- Remove “Search all namespaces” — investigations are single-namespace only (cross-namespace planned later).

## [0.1.8] — 2026-09-07

### Cluster connectivity (enterprise / read-only clusters)

- Bootstrap login-shell environment at startup so GUI launches get the same `PATH`, `KUBECONFIG`, and cloud credential vars as Terminal, Lens, and K9s.
- Align kubeconfig loading with kubectl (multi-file `KUBECONFIG`, default loading rules).
- Soft-fail when namespace listing is RBAC-restricted; show a warning instead of a blocking sync error.
- Allow typing a namespace manually when the list is incomplete.
- Fall back to `kubectl get ns` when the in-process API client fails.

### Bundled kubectl

- Ship kubectl v1.31.4 inside `Klew.app/Contents/Resources/` (signed with the app).
- Settings → Kubernetes: choose bundled, system PATH, or a custom kubectl path.
- Conditionally download a cluster-matched kubectl when skew exceeds ± one minor version (cached under `~/Library/Application Support/Klew/binaries/`).
- Terminal prepends the active kubectl directory to `PATH`.

### Release

- Switch Homebrew distribution from a formula to a cask so Klew installs to `/Applications` and upgrades with `brew upgrade --cask klew`.

## [0.1.7] — 2026-09-01

### Investigation engine

- Merge snapshot container logs (including previous-container logs) into pattern mining on every refresh, so Correlated Signals and Log Patterns update without requiring live log tail.
- Spread snapshot log timestamps across recent minutes so burst failures (OOM, crash loops) align with infrastructure event patterns.

### Desktop app

- Streamline Evidence: drop duplicate verdict header, causal notes, and time-window controls; focus on correlated signals, observations, and claims.
- Add bottom console dock for terminal and live logs; fix layout so the log panel stays visible during investigations.
- Refine Overview, Patterns, Resources, and shell navigation; move Settings and Help to the top bar.
- Fix component inspector hook ordering crash and bottom inspector layout.

### Docs

- Add full screenshot set (Overview, Failures, Patterns, Evidence, Resources, Graph) to the README.

### Tooling

- Add `cmd/klew-stress` and engine stress benchmarks for ingestion and correlation load testing.

## [0.1.6] — 2026-09-01

### Desktop app

- Ship the Wails desktop shell as the primary Klew experience, replacing the prior TUI/CLI workflow.
- Add an investigation workbench with scope-aware resource catalog navigation, entity tables, and deep inspection panels.
- Introduce RBAC-aware browsing: catalog entries and secret-backed environment values respect cluster permissions.
- Add lazy resource counts in the sidebar so empty or zero-count kinds no longer show misleading totals.
- Fix resource-kind switching crashes when browsing catalog kinds with no entities (Events, Nodes, and similar).
- Streamline the resources explorer layout and investigation context banner for clearer cross-surface navigation.
- Add terminal workspace tabs, shell selection, and appearance controls integrated into the desktop shell.

### Release

- Fix the Homebrew formula template so installed apps keep their code signature.
- Add a notarization credential preflight in release CI with clearer failure messages.
- Bump `wails.json` product version to 0.1.6.

## [0.1.5] — 2026-08-15

### Desktop app

- Add Klew logo mark to the sidebar and update the Dock icon to match the marketing site.
- Redesign the top bar: refined scope selectors, command-style search, cleaner typography (Plus Jakarta Sans), and a compact connection status rail.
- Split **Settings** and **Help** in the sidebar; Help includes keyboard shortcuts and documentation links.
- Add native macOS menu bar menus: **Edit**, **Tools** (sync, new window, settings, kubeconfig folder), and **Window**.
- Enable the green maximize traffic-light button by configuring Wails macOS options.
- Remove redundant “empty search” helper copy from the search field and welcome screens.

### Release

- Harden macOS CI signing (avoid brittle `--deep --strict` codesign) and pin Node 22 for release builds.

### Other

- Remove demo mode and simulated investigation paths.
- Move release scripts under `.github/scripts/`; package script now stages the app icon from `packaging/macos/appicon.png`.

## [0.1.4] — 2026-07-31

- Sign and notarize macOS releases with Developer ID in CI.
- Automate Homebrew tap bumps on release.
- Fix Homebrew zip install using `cp_r` and buildpath detection.

## [0.1.3] — earlier

- Prior desktop and CLI improvements; see git history and GitHub releases.

[2.0.0]: https://github.com/glnreddy421/klew/releases/tag/v2.0.0
[0.1.10]: https://github.com/glnreddy421/klew/releases/tag/v0.1.10
[0.1.9]: https://github.com/glnreddy421/klew/releases/tag/v0.1.9
[0.1.8]: https://github.com/glnreddy421/klew/releases/tag/v0.1.8
[0.1.7]: https://github.com/glnreddy421/klew/releases/tag/v0.1.7
[0.1.6]: https://github.com/glnreddy421/klew/releases/tag/v0.1.6
[0.1.5]: https://github.com/glnreddy421/klew/releases/tag/v0.1.5
[0.1.4]: https://github.com/glnreddy421/klew/releases/tag/v0.1.4
[0.1.3]: https://github.com/glnreddy421/klew/releases/tag/v0.1.3