package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/glnreddy421/klew/internal/model"
)

// SettingsRuntime carries live TUI controls shown in the settings overlay.
type SettingsRuntime struct {
	AutoRefresh  bool
	RefreshEvery time.Duration
}

// SettingsView is the settings overlay — scope, permissions, watches, refresh.
func SettingsView(st model.InvestigationState, width int, runtime SettingsRuntime) string {
	var s []string
	s = append(s, kv("Context", st.KubeContext.Context))
	s = append(s, kv("Cluster", st.KubeContext.Cluster))
	if st.NamespaceScope.AllNamespaces {
		s = append(s, kv("Namespace scope", "all namespaces (-A)"))
	} else {
		s = append(s, kv("Namespace scope", nsLabel(st)))
	}
	s = append(s, kv("Query", st.Query))
	s = append(s, kv("Mode", string(st.Mode)))
	s = append(s, kv("Active watches", fmt.Sprintf("%d", len(st.ActiveWatches))))
	if runtime.AutoRefresh {
		s = append(s, kv("Auto-refresh", fmt.Sprintf("on every %s (toggle with a)", runtime.RefreshEvery)))
	} else {
		s = append(s, kv("Auto-refresh", "off (toggle with a)"))
	}
	s = append(s, kv("Desktop UI", "brew install klew-desktop"))
	s = append(s, kv("Evidence stream", "ranked (toggle with e)"))
	s = append(s, kv("Ingested", fmt.Sprintf("%d events · %d logs · %d changes",
		st.Counters.EventsIngested, st.Counters.LogsIngested, st.Counters.ObjectChanges)))
	settingsPanel := Panel("Investigation Settings", width, strings.Join(s, "\n"))

	var perms []string
	for _, p := range groupPermissions(st.Permissions) {
		mark := okStyle.Render("✓")
		if !p.allowed {
			mark = critStyle.Render("✗ missing")
		}
		perms = append(perms, fmt.Sprintf("%s %s", padRight(p.label, width-16), mark))
	}
	if len(perms) == 0 {
		perms = append(perms, dimStyle.Render("no permission probes recorded"))
	}
	permsPanel := Panel("Permissions", width, strings.Join(perms, "\n"))

	var missing []string
	for _, w := range st.Warnings {
		missing = append(missing, warnStyle.Render("⚠ ")+truncVisual(w, width-8))
	}
	if len(missing) == 0 {
		missing = append(missing, okStyle.Render("no missing data — all probed permissions granted"))
	}
	missingPanel := Panel("Missing Data", width, strings.Join(missing, "\n"))

	return settingsPanel + "\n" + permsPanel + "\n" + missingPanel
}

type permGroup struct {
	label   string
	allowed bool
}

func groupPermissions(checks []model.PermissionCheck) []permGroup {
	verbs := map[string]map[string]bool{}
	order := []string{}
	for _, c := range checks {
		if _, ok := verbs[c.Resource]; !ok {
			verbs[c.Resource] = map[string]bool{}
			order = append(order, c.Resource)
		}
		verbs[c.Resource][c.Verb] = c.Allowed
	}
	var out []permGroup
	for _, res := range order {
		vs := verbs[res]
		var vlist []string
		all := true
		for _, v := range []string{"get", "list", "watch"} {
			if allowed, present := vs[v]; present {
				vlist = append(vlist, v)
				if !allowed {
					all = false
				}
			}
		}
		out = append(out, permGroup{label: res + " " + strings.Join(vlist, "/"), allowed: all})
	}
	return out
}
