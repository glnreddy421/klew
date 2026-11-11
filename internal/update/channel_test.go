package update

import "testing"

func TestDetectChannelFromPath(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"/opt/homebrew/Caskroom/klew/2.0.0/Klew.app/Contents/MacOS/Klew", ChannelHomebrew},
		{"/usr/local/Caskroom/klew/2.0.0/Klew.app/Contents/MacOS/Klew", ChannelHomebrew},
		{"/Applications/Klew.app/Contents/MacOS/Klew", ChannelDirect},
		{"/tmp/klew-dev/klew", ChannelUnknown},
	}
	for _, tc := range cases {
		got := channelFromPath(tc.path)
		if got != tc.want {
			t.Fatalf("path %q: got %q want %q", tc.path, got, tc.want)
		}
	}
}
