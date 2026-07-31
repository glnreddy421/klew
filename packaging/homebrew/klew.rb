class Klew < Formula
  desc "Klew — live Kubernetes incident investigation (desktop app)"
  homepage "https://github.com/glnreddy421/klew"
  license "Apache-2.0"
  version "0.1.4"

  depends_on :macos

  on_macos do
    on_arm do
      url "https://github.com/glnreddy421/klew/releases/download/v0.1.4/Klew-0.1.4-macos-arm64.zip"
      sha256 "1116cb39b811e1a755e9712743d857b402e22c6083823ce749afa929ae05ac19"
    end
  end

  def install
    prefix.install "Klew.app"
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
