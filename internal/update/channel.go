package update

import (
	"os"
	"path/filepath"
	"strings"
)

// DetectChannel guesses how this build was installed.
// Homebrew cask installs live under Caskroom (often symlinked into /Applications).
func DetectChannel() string {
	exe, err := os.Executable()
	if err != nil || exe == "" {
		return ChannelUnknown
	}
	resolved := exe
	if link, err := filepath.EvalSymlinks(exe); err == nil && link != "" {
		resolved = link
	}
	return channelFromPath(resolved)
}

func channelFromPath(path string) string {
	path = strings.ToLower(filepath.ToSlash(path))
	switch {
	case strings.Contains(path, "/caskroom/klew/"):
		return ChannelHomebrew
	case strings.Contains(path, "/homebrew/") && strings.Contains(path, "/klew"):
		return ChannelHomebrew
	case strings.HasSuffix(path, "/klew.app/contents/macos/klew"):
		return ChannelDirect
	default:
		return ChannelUnknown
	}
}
