package format

import "fmt"

func Row(k, v string) string {
	return fmt.Sprintf("  %-18s %s", k+":", v)
}

func Truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
