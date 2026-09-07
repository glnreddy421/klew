cask "klew" do
  version "0.1.10"

  on_arm do
    sha256 "REPLACE_ON_RELEASE"
    url "https://github.com/glnreddy421/klew/releases/download/v0.1.10/Klew-0.1.10-macos-arm64.zip"
  end
  on_intel do
    sha256 "REPLACE_ON_RELEASE"
    url "https://github.com/glnreddy421/klew/releases/download/v0.1.10/Klew-0.1.10-macos-amd64.zip"
  end

  name "Klew"
  desc "Live Kubernetes incident investigation (desktop app)"
  homepage "https://github.com/glnreddy421/klew"

  app "Klew.app"

  livecheck do
    url "https://github.com/glnreddy421/klew/releases/latest"
    strategy :github_latest
  end
end
