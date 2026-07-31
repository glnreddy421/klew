package tui

import (
	"strings"
	"testing"
)

func TestRenderLaunchBanner(t *testing.T) {
	out := RenderLaunchBanner(80)
	if !strings.Contains(out, "K L E W") {
		t.Fatalf("missing wordmark: %q", out)
	}
	if !strings.Contains(out, logoTagline) {
		t.Fatalf("missing tagline: %q", out)
	}
	if !strings.Contains(out, "◈") {
		t.Fatalf("missing accent: %q", out)
	}
}

func TestRenderLogoBadge(t *testing.T) {
	if !strings.Contains(RenderLogoBadge(), "KLEW") {
		t.Fatal("badge missing KLEW")
	}
}

func TestFooterWithLogo(t *testing.T) {
	help := FooterHelp()
	out := FooterWithLogo(help, 120)
	if !strings.Contains(out, "KLEW") {
		t.Fatalf("footer missing logo: %q", out)
	}
}
