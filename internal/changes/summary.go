package changes

import (
	"fmt"
	"strings"

	"github.com/glnreddy421/klew/internal/model"
)

// MaxSummaryLen caps one-line summaries; the full diff is available on expand.
const MaxSummaryLen = 120

// Summarize renders diffs as one short line, most telling fields first:
// "image v2.3.1 → v2.4.0, env DB_POOL_SIZE 20 → 5, +2 more".
func Summarize(diffs []model.FieldDiff) string {
	if len(diffs) == 0 {
		return "No template changes"
	}
	var parts []string
	used := make([]bool, len(diffs))
	// Restarts and images read best in full; then env; then other areas by name.
	for _, area := range []string{AreaRestart, AreaImage, AreaContainers, AreaEnv, AreaValues, AreaChart} {
		for i, d := range diffs {
			if used[i] || d.Area != area {
				continue
			}
			used[i] = true
			parts = append(parts, describe(d))
		}
	}
	seenArea := map[string]bool{}
	for i, d := range diffs {
		if used[i] || seenArea[d.Area] {
			continue
		}
		seenArea[d.Area] = true
		parts = append(parts, areaLabel(d.Area))
	}
	return fitParts(parts)
}

// Areas lists the distinct changed areas in first-seen order.
func Areas(diffs []model.FieldDiff) []string {
	seen := map[string]bool{}
	var out []string
	for _, d := range diffs {
		if d.Area != "" && !seen[d.Area] {
			seen[d.Area] = true
			out = append(out, d.Area)
		}
	}
	return out
}

func describe(d model.FieldDiff) string {
	switch d.Area {
	case AreaRestart:
		return "restarted (rollout restart)"
	case AreaImage:
		before, after := shortImage(d.Before, d.After)
		return fmt.Sprintf("image %s → %s", before, after)
	case AreaContainers:
		name := lastBracket(d.Path)
		if d.Kind == model.DiffAdded {
			return "container +" + name
		}
		return "container −" + name
	}
	name := lastSegment(d.Path)
	switch d.Kind {
	case model.DiffAdded:
		return fmt.Sprintf("%s +%s", d.Area, name)
	case model.DiffRemoved:
		return fmt.Sprintf("%s −%s", d.Area, name)
	default:
		return fmt.Sprintf("%s %s %s → %s", d.Area, name, orDash(d.Before), orDash(d.After))
	}
}

// shortImage drops a shared repository so "repo/app:v1 → repo/app:v2" reads "v1 → v2".
func shortImage(before, after string) (string, string) {
	br, bt := splitImage(before)
	ar, at := splitImage(after)
	if br == ar && br != "" && bt != "" && at != "" {
		return bt, at
	}
	return orDash(before), orDash(after)
}

func splitImage(img string) (repo, tag string) {
	if i := strings.LastIndex(img, "@"); i >= 0 {
		return img[:i], shortDigest(img[i+1:])
	}
	slash := strings.LastIndex(img, "/")
	if i := strings.LastIndex(img, ":"); i > slash {
		return img[:i], img[i+1:]
	}
	return img, "latest"
}

func shortDigest(d string) string {
	if i := strings.Index(d, ":"); i >= 0 && len(d) > i+13 {
		return d[:i+13]
	}
	return d
}

func areaLabel(area string) string {
	switch area {
	case AreaServiceAccount:
		return "service account"
	case AreaSecurityContext:
		return "security context"
	case AreaEnvFrom:
		return "envFrom"
	}
	return area
}

func lastSegment(path string) string {
	if i := strings.LastIndex(path, "."); i >= 0 {
		return path[i+1:]
	}
	return path
}

func lastBracket(path string) string {
	if i := strings.LastIndex(path, "["); i >= 0 {
		return strings.TrimSuffix(path[i+1:], "]")
	}
	return path
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

// fitParts joins parts within MaxSummaryLen, ending with "+N more" when cut.
func fitParts(parts []string) string {
	var b strings.Builder
	for i, p := range parts {
		sep := ""
		if i > 0 {
			sep = ", "
		}
		rest := len(parts) - i
		more := ""
		if rest > 1 {
			more = fmt.Sprintf(", +%d more", rest-1)
		}
		if b.Len()+len(sep)+len(p)+len(more) > MaxSummaryLen && i > 0 {
			fmt.Fprintf(&b, ", +%d more", rest)
			return b.String()
		}
		if i == 0 && len(p) > MaxSummaryLen {
			p = p[:MaxSummaryLen-1] + "…"
		}
		b.WriteString(sep)
		b.WriteString(p)
	}
	return b.String()
}
