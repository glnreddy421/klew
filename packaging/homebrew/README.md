# Homebrew

Formula for local development and for copying into the separate **[klew-homebrew](https://github.com/glnreddy421/klew-homebrew)** tap repo on release.

## Local install

```bash
brew install --HEAD ./packaging/homebrew/klew.rb
open "$(brew --prefix)/opt/klew/Klew.app"
```

## Release (maintainers)

1. Tag `vX.Y.Z` on this repo.
2. Update `version` and `sha256` in `klew.rb`:

```bash
curl -L "https://github.com/glnreddy421/klew/archive/refs/tags/vX.Y.Z.tar.gz" | shasum -a 256
```

3. Copy `packaging/homebrew/klew.rb` → `Formula/klew.rb` in **klew-homebrew** and push.

## User install

```bash
brew tap glnreddy421/klew-homebrew
brew install klew
```
