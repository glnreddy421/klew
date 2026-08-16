# Changelog

All notable changes to Klew are documented here. Version tags follow [SemVer](https://semver.org/).

## [0.1.5] — 2026-08-15

### Desktop app

- Add Klew logo mark to the sidebar and update the Dock icon to match the marketing site.
- Redesign the top bar: refined scope selectors, command-style search, cleaner typography (Plus Jakarta Sans), and a compact connection status rail.
- Split **Settings** and **Help** in the sidebar; Help includes keyboard shortcuts and documentation links.
- Add native macOS menu bar menus: **Edit**, **Tools** (sync, new window, settings, kubeconfig folder), and **Window**.
- Enable the green maximize traffic-light button by configuring Wails macOS options.
- Remove redundant “empty search” helper copy from the search field and welcome screens.

### Other

- Remove demo mode and simulated investigation paths.
- Move release scripts under `.github/scripts/`; package script now stages the app icon from `packaging/macos/appicon.png`.

## [0.1.4] — 2026-07-31

- Sign and notarize macOS releases with Developer ID in CI.
- Automate Homebrew tap bumps on release.
- Fix Homebrew zip install using `cp_r` and buildpath detection.

## [0.1.3] — earlier

- Prior desktop and CLI improvements; see git history and GitHub releases.

[0.1.5]: https://github.com/glnreddy421/klew/releases/tag/v0.1.5
[0.1.4]: https://github.com/glnreddy421/klew/releases/tag/v0.1.4
[0.1.3]: https://github.com/glnreddy421/klew/releases/tag/v0.1.3
