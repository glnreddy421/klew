class Klew < Formula
  desc "Klew — live Kubernetes incident investigation (desktop app)"
  homepage "https://github.com/glnreddy421/klew"
  license "Apache-2.0"

  depends_on "go" => :build
  depends_on "node" => :build
  depends_on :macos

  if build.head?
    url "https://github.com/glnreddy421/klew.git", branch: "main"
  else
    url "https://github.com/glnreddy421/klew/archive/refs/tags/v0.1.0.tar.gz"
    sha256 "UPDATE_ON_RELEASE"
    version "0.1.0"
  end

  def install
    wails_bin = buildpath/"wails-bin"
    wails_bin.mkpath
    ENV["GOBIN"] = wails_bin
    system "go", "install", "github.com/wailsapp/wails/v2/cmd/wails@v2.13.0"
    ENV.prepend_path "PATH", wails_bin.to_s

    cd "cmd/klew-desktop" do
      system "npm", "install", "--prefix", "frontend"
      system "npm", "run", "build", "--prefix", "frontend"
      system "wails", "build", "-clean"
      prefix.install "build/bin/Klew.app"
    end
  end

  def caveats
    <<~EOS
      Klew.app is installed at:
        #{prefix}/Klew.app

      Launch from Finder or run:
        open #{prefix}/Klew.app
    EOS
  end

  test do
    assert_path_exists prefix/"Klew.app"
    assert_path_exists prefix/"Klew.app/Contents/MacOS/Klew"
  end
end
