# Homebrew

Formula template for the **[homebrew-klew](https://github.com/glnreddy421/homebrew-klew)** tap.

Installs the signed, notarized **macOS arm64** zip from GitHub Releases (not a source build).

## Local install

```bash
brew install ./packaging/homebrew/klew.rb
open "$(brew --prefix)/opt/klew/Klew.app"
```

## Release automation

On each `v*` tag, `.github/workflows/release.yml` runs `scripts/update-homebrew-tap.sh`, which commits an updated `Formula/klew.rb` to **homebrew-klew**.

Requires this GitHub secret on **klew**:

| Secret | Value |
|--------|--------|
| `HOMEBREW_TAP_TOKEN` | Fine-grained PAT with **Contents: Read and write** on `glnreddy421/homebrew-klew` |

## User install

```bash
brew tap glnreddy421/klew
brew trust glnreddy421/klew
brew install klew
```
