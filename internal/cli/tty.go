package cli

import (
	"io"
	"os"

	"golang.org/x/term"
)

// isTerminal reports whether w is a terminal, for progress lines and color.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

// colorFor is true for a terminal, unless NO_COLOR is set or TERM is dumb.
func colorFor(w io.Writer) bool {
	if _, ok := os.LookupEnv("NO_COLOR"); ok || os.Getenv("TERM") == "dumb" {
		return false
	}
	return isTerminal(w)
}
