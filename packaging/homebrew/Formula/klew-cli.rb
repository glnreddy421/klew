# Reference copy. The release workflow writes the real formula to
# glnreddy421/homebrew-klew (.github/scripts/update-homebrew-tap.sh).
class KlewCli < Formula
  desc "Kubernetes incident investigation, with a read-only MCP server"
  homepage "https://github.com/glnreddy421/klew"
  version "0.0.0"
  license "Apache-2.0"

  on_macos do
    on_arm do
      url "https://github.com/glnreddy421/klew/releases/download/v0.0.0/klew-cli-0.0.0-darwin-arm64.tar.gz"
      sha256 "REPLACE_ON_RELEASE"
    end
    on_intel do
      url "https://github.com/glnreddy421/klew/releases/download/v0.0.0/klew-cli-0.0.0-darwin-amd64.tar.gz"
      sha256 "REPLACE_ON_RELEASE"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/glnreddy421/klew/releases/download/v0.0.0/klew-cli-0.0.0-linux-arm64.tar.gz"
      sha256 "REPLACE_ON_RELEASE"
    end
    on_intel do
      url "https://github.com/glnreddy421/klew/releases/download/v0.0.0/klew-cli-0.0.0-linux-amd64.tar.gz"
      sha256 "REPLACE_ON_RELEASE"
    end
  end

  def install
    bin.install "klew"
  end

  def caveats
    <<~EOS
      Investigate a namespace:
        klew investigate -n NAMESPACE
      Add it to Claude Code:
        claude mcp add --scope user klew -- "$(command -v klew)" mcp
      More: https://github.com/glnreddy421/klew/blob/main/docs/cli.md
    EOS
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/klew version")
  end
end
