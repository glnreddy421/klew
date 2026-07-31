package cli

import "testing"

func TestGraphCommandRegistered(t *testing.T) {
	found := false
	for _, c := range rootCmd.Commands() {
		if c.Name() == "graph" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("graph command not registered")
	}
}
