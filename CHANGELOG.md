# Changelog

All notable changes to Klew are documented here. Version tags follow [SemVer](https://semver.org/).

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

[0.1.7]: https://github.com/glnreddy421/klew/releases/tag/v0.1.7
[0.1.6]: https://github.com/glnreddy421/klew/releases/tag/v0.1.6
[0.1.5]: https://github.com/glnreddy421/klew/releases/tag/v0.1.5
[0.1.4]: https://github.com/glnreddy421/klew/releases/tag/v0.1.4
[0.1.3]: https://github.com/glnreddy421/klew/releases/tag/v0.1.3
