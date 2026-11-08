package cli

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/glnreddy421/klew/v2/internal/model"
)

// Sections klew investigate can print, in order.
var sections = []string{"overview", "failures", "patterns", "evidence"}

// printer writes the text report. With color it adds ANSI styles; without,
// the output is plain and safe to pipe.
type printer struct {
	w     io.Writer
	color bool
	// truncated is set when a list was cut by the limit, to suggest -all.
	truncated bool
}

const (
	bold   = "1"
	dim    = "2"
	red    = "31"
	yellow = "33"
	green  = "32"
	cyan   = "36"
)

func (p *printer) style(code, s string) string {
	if !p.color || s == "" {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

func (p *printer) line(format string, args ...any) { fmt.Fprintf(p.w, format+"\n", args...) }

func (p *printer) heading(title string) {
	p.line("")
	p.line("%s", p.style(bold, strings.ToUpper(title)))
}

func (p *printer) sub(title string, shown, total int) {
	if total > shown {
		p.truncated = true
		p.line("  %s %s", p.style(bold, title), p.style(dim, fmt.Sprintf("(%d of %d)", shown, total)))
		return
	}
	p.line("  %s", p.style(bold, title))
}

// severity colors s by its level, padded to width first so columns line up.
func (p *printer) severity(s string, width int) string {
	padded := fmt.Sprintf("%-*s", width, s)
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "critical", "high":
		return p.style(red, padded)
	case "warning":
		return p.style(yellow, padded)
	case "healthy", "ok":
		return p.style(green, padded)
	}
	return padded
}

func writeText(w io.Writer, r Report, show map[string]bool, color bool) {
	p := &printer{w: w, color: color}
	p.header(r)
	if show["overview"] {
		p.overview(r)
	}
	if show["failures"] {
		p.failures(r.Failures)
	}
	if show["patterns"] {
		p.patterns(r.Patterns)
	}
	if show["evidence"] {
		p.evidence(r.Evidence)
	}
	warnings := r.Warnings
	if !show["evidence"] {
		warnings = append(append([]string{}, r.Evidence.Gaps...), warnings...)
	}
	if len(warnings) > 0 {
		p.heading("Warnings")
		for _, w := range warnings {
			p.line("  ! %s", w)
		}
	}
	if p.truncated {
		p.line("")
		p.line("%s", p.style(dim, "Some lists are cut short; -all shows everything, -o json gives every field."))
	}
}

func (p *printer) header(r Report) {
	scope := r.Namespace
	if r.Workload != "" {
		scope += " / " + r.Workload
	}
	meta := []string{"context " + r.Context}
	if t := r.CollectedAt.Time(); !t.IsZero() {
		meta = append(meta, "collected "+t.Local().Format("15:04:05"))
	}
	if r.ObservedSeconds > 0 {
		meta = append(meta, fmt.Sprintf("watched %ds", r.ObservedSeconds))
	}
	p.line("%s  %s", p.style(bold, scope), p.style(dim, strings.Join(meta, " · ")))
	p.line("")

	status := strings.ToUpper(string(r.Status))
	if status == "" {
		status = "UNKNOWN"
	}
	verdict := p.style(bold, p.severity(status, 0))
	if r.LeadingSignal != "" {
		verdict += "  " + r.LeadingSignal
	}
	p.line("%s  %s", verdict, p.style(dim, fmt.Sprintf("confidence %.0f%% (%s)", r.Confidence*100, r.ConfidenceLabel)))
	if r.Summary != "" {
		p.line("%s", r.Summary)
	}
	if r.LikelyTrigger != "" && r.LikelyTrigger != r.Summary {
		p.line("Likely trigger: %s", r.LikelyTrigger)
	}
	if r.Headline != "" {
		p.line("%s", r.Headline)
	}
	pods := fmt.Sprintf("Pods: %d ready, %d unready, %d restarts", r.Pods.Ready, r.Pods.Unready, r.Pods.Restarts)
	if r.EndpointsTotal > 0 {
		pods += fmt.Sprintf(" · endpoints %d/%d ready", r.EndpointsReady, r.EndpointsTotal)
	}
	p.line("%s", pods)
}

func (p *printer) overview(r Report) {
	o := r.Overview
	p.heading("Overview")
	if pc := o.PinnedChange; pc != nil {
		when := ""
		if pc.BeforeIncident {
			when = " (before the first incident)"
		}
		p.line("  %s %s%s", p.style(cyan, "Change:"), pc.Text, when)
	} else if rc := o.RecentChange; rc != nil && (rc.RevisionTo != "" || rc.Image != "") {
		p.line("  %s %s", p.style(cyan, "Recent change:"), recentChange(rc))
	}
	if len(o.Problems) > 0 {
		p.sub("Problems", len(o.Problems), o.ProblemsTotal)
		for _, pr := range o.Problems {
			p.line("    %s %s  %s", p.severity(string(pr.Severity), 8), pr.Object, pr.State)
			for _, f := range pr.Facts {
				p.line("             %s", p.style(dim, oneLine(f)))
			}
		}
	}
	if len(o.CausalChain) > 0 {
		p.sub("What Klew connected", len(o.CausalChain), len(o.CausalChain))
		for i, c := range o.CausalChain {
			p.line("    %d. %s", i+1, c)
		}
	} else if r.Status == model.VerdictHealthy && len(o.Problems) == 0 {
		p.line("  No significant failure chain detected.")
	}
	if o.Hypothesis != "" && o.Hypothesis != r.Summary {
		p.hypothesis(o)
	}
	if len(o.Incidents) > 0 {
		p.sub("Incidents", len(o.Incidents), o.IncidentsTotal)
		for _, in := range o.Incidents {
			state := ""
			if in.Recovered {
				state = p.style(green, " recovered")
			}
			if in.Recorded {
				state += p.style(dim, " recorded")
			}
			p.line("    %-17s  %s %s ×%d%s%s", span(in.Start, in.End), p.severity(string(in.Severity), 8), in.LeadingReason, in.Count, objects(in.Objects), state)
		}
	}
	if len(o.Timeline) > 0 {
		p.sub("Timeline", len(o.Timeline), o.TimelineTotal)
		for _, t := range o.Timeline {
			obj := strings.TrimPrefix(t.SourceKind+"/"+t.SourceName, "/")
			p.line("    %s  %s %s", clock(t.Timestamp), p.severity(string(t.Severity), 8), strings.TrimSpace(prefixed("", obj)+"  "+oneLine(withReason(t.Reason, t.Message))))
		}
	}
	if h := o.EventHistory; h != nil && h.RecordedEvents > 0 {
		p.line("  %s", p.style(dim, fmt.Sprintf("Includes %d events from the desktop app's history that the API no longer keeps.", h.RecordedEvents)))
	}
}

func (p *printer) hypothesis(o Overview) {
	p.line("  %s %s", p.style(bold, "Hypothesis:"), o.Hypothesis)
	for _, reason := range o.HypothesisReasons {
		p.line("    - %s", reason)
	}
	var alts []string
	for _, a := range o.Alternatives {
		if !a.Leading {
			alts = append(alts, fmt.Sprintf("%s %.0f%%", a.Label, a.Confidence*100))
		}
	}
	if len(alts) > 0 {
		p.line("    %s", p.style(dim, "Alternatives: "+strings.Join(alts, ", ")))
	}
}

func (p *printer) failures(f Failures) {
	p.heading("Failures")
	switch {
	case f.PodsTotal == 0:
		p.line("  No pods in scope.")
	case f.FailingTotal == 0:
		p.line("  All %d pods stable.", f.PodsTotal)
	default:
		p.line("  %d of %d pods failing", f.FailingTotal, f.PodsTotal)
	}
	if a := f.Admission; a != nil {
		p.line("  %s workloads expect pods but none exist; admission webhooks may be rejecting them (%s).", p.style(yellow, "!"), a.Status)
		for _, c := range a.Candidates {
			p.line("    %s %s (webhook %s, failurePolicy %s)", c.ConfigKind, c.ConfigName, c.WebhookName, firstNonEmpty(c.FailurePolicy, "-"))
		}
	}
	if len(f.Pods) == 0 {
		return
	}
	if len(f.Pods) < f.FailingTotal {
		p.truncated = true
	}
	for _, pod := range f.Pods {
		ready := "not ready"
		if pod.Ready {
			ready = "ready"
		}
		reason := p.style(toneColor(pod), firstNonEmpty(pod.Reason, pod.Phase))
		p.line("")
		p.line("  %s  %s · %s · %d restarts%s", p.style(bold, pod.Name), reason, ready, pod.Restarts, prefixed(" · ", pod.Node))
		if pod.Pending != "" {
			p.line("    Unschedulable: %s", oneLine(pod.Pending))
		}
		for _, c := range pod.Containers {
			p.line("    container %s: %s", c.Name, container(c))
			if c.Message != "" {
				p.line("      %s", p.style(dim, oneLine(c.Message)))
			}
		}
		for _, e := range pod.Events {
			reason := e.Reason
			if e.Warning {
				reason = p.style(yellow, reason)
			}
			p.line("    %s  %s %s%s", clock(e.Time), reason, oneLine(e.Message), recorded(p, e.Recorded))
		}
	}
}

func (p *printer) patterns(pt Patterns) {
	p.heading("Patterns")
	if pt.LogsTotal == 0 && pt.EventsTotal == 0 {
		p.line("  No log or event patterns in the window.")
		return
	}
	p.patternList("Log patterns", pt.Logs, pt.LogsTotal)
	p.patternList("Event patterns", pt.Events, pt.EventsTotal)
}

func (p *printer) patternList(title string, list []Pattern, total int) {
	if total == 0 {
		return
	}
	p.sub(title, len(list), total)
	for _, t := range list {
		trend := t.Trend
		if trend == "" {
			trend = "·"
		}
		p.line("    %s %5d× %3.0f%%  %s", trend, t.Count, t.Pct, oneLine(t.Template))
		if len(t.Pods) > 0 {
			p.line("              %s", p.style(dim, "pods: "+strings.Join(capped(t.Pods, 4), ", ")+more(len(t.Pods), 4)))
		}
		if t.Sample != "" {
			p.line("              %s", p.style(dim, "e.g. "+oneLine(t.Sample)))
		}
	}
}

func (p *printer) evidence(e Evidence) {
	p.heading("Evidence")
	if len(e.Correlated) > 0 {
		p.sub("Correlated (event ↔ logs)", len(e.Correlated), len(e.Correlated))
		for _, c := range e.Correlated {
			p.line("    %3.0f%%  %s", c.Confidence*100, oneLine(c.Event.Template))
			for _, l := range c.Logs {
				p.line("          → %s", oneLine(l.Template))
			}
		}
	}
	if len(e.Signals) > 0 {
		p.sub("Claims", len(e.Signals), e.SignalsTotal)
		for _, s := range e.Signals {
			count := ""
			if s.Count > 1 {
				count = fmt.Sprintf(" ×%d", s.Count)
			}
			p.line("    %-6s %s%s", s.Strength, s.Label, count)
			if s.Evidence != "" {
				p.line("           %s", p.style(dim, oneLine(s.Evidence)))
			}
		}
	}
	if len(e.Observations) > 0 {
		p.sub("Observations", len(e.Observations), e.ObservationsTotal)
		for _, o := range e.Observations {
			reason := o.Reason
			if o.Warning {
				reason = p.style(yellow, reason)
			}
			p.line("    %s  %-6s %s %s%s", clock(o.Time), o.Kind, strings.TrimSpace(prefixed("", o.Object)+" "+reason), oneLine(o.Message), recorded(p, o.Recorded))
		}
	}
	if len(e.NotCollected) > 0 {
		p.sub("Not checked", len(e.NotCollected), len(e.NotCollected))
		for _, c := range e.NotCollected {
			p.line("    %s: %s%s", c.Collector, c.Status, prefixed(" — ", c.Detail))
		}
	}
	if len(e.Gaps) > 0 {
		p.sub("Gaps", len(e.Gaps), len(e.Gaps))
		for _, g := range e.Gaps {
			p.line("    - %s", g)
		}
	}
	if len(e.NextChecks) > 0 {
		p.sub("Next checks", len(e.NextChecks), len(e.NextChecks))
		for i, n := range e.NextChecks {
			p.line("    %d. %s", i+1, n)
		}
	}
	if len(e.SuggestedActions) > 0 {
		p.sub("Suggested actions (not run)", len(e.SuggestedActions), len(e.SuggestedActions))
		for _, a := range e.SuggestedActions {
			p.line("    - %s", a)
		}
	}
}

// toneColor is red for reasons that stop a container, else yellow.
func toneColor(pod FailingPod) string {
	r := strings.ToLower(pod.Reason + " " + pod.Phase)
	for _, s := range []string{"oom", "crash", "error", "image", "failed"} {
		if strings.Contains(r, s) {
			return red
		}
	}
	return yellow
}

func container(c model.ContainerStatus) string {
	parts := []string{firstNonEmpty(c.State, "unknown")}
	if c.Reason != "" {
		parts[0] += " (" + c.Reason + ")"
	}
	if c.ExitCode != 0 {
		parts = append(parts, fmt.Sprintf("exit %d", c.ExitCode))
	}
	if c.LastReason != "" {
		last := "last " + c.LastReason
		if c.LastExitCode != 0 {
			last += fmt.Sprintf(" exit %d", c.LastExitCode)
		}
		parts = append(parts, last)
	}
	if c.RestartCount > 0 {
		parts = append(parts, fmt.Sprintf("%d restarts", c.RestartCount))
	}
	if c.Init {
		parts = append(parts, "init")
	}
	if c.Image != "" {
		parts = append(parts, c.Image)
	}
	return strings.Join(parts, " · ")
}

func recentChange(rc *model.RecentChange) string {
	var parts []string
	if rc.RevisionTo != "" {
		parts = append(parts, "revision "+strings.TrimPrefix(rc.RevisionFrom+" → ", " → ")+rc.RevisionTo)
	}
	if rc.Image != "" {
		parts = append(parts, rc.Image)
	}
	if t := rc.DeployedAt.Time(); !t.IsZero() {
		parts = append(parts, "at "+t.Local().Format("15:04:05"))
	}
	if rc.RolloutStatus != "" {
		parts = append(parts, rc.RolloutStatus)
	}
	return strings.Join(parts, " · ")
}

func recorded(p *printer, r bool) string {
	if !r {
		return ""
	}
	return p.style(dim, " (recorded)")
}

func objects(refs []model.ObjectRef) string {
	if len(refs) == 0 {
		return ""
	}
	names := make([]string, 0, 3)
	for _, r := range capped(refs, 3) {
		names = append(names, r.Kind+"/"+r.Name)
	}
	return "  " + strings.Join(names, ", ") + more(len(refs), 3)
}

func more(n, shown int) string {
	if n > shown {
		return fmt.Sprintf(" +%d more", n-shown)
	}
	return ""
}

func withReason(reason, msg string) string {
	if reason == "" {
		return msg
	}
	return reason + ": " + msg
}

func prefixed(prefix, s string) string {
	if s == "" {
		return ""
	}
	return prefix + s
}

// clock is the local time of ts, with the date when it is not today.
func clock(ts model.Timestamp) string {
	t := ts.Time()
	if t.IsZero() {
		return "--:--:--"
	}
	t = t.Local()
	if y, m, d := t.Date(); y != now().Year() || m != now().Month() || d != now().Day() {
		return t.Format("Jan 02 15:04:05")
	}
	return t.Format("15:04:05")
}

// now is replaced in tests.
var now = time.Now

func span(start, end model.Timestamp) string {
	s, e := clock(start), clock(end)
	if s == e || end == "" {
		return s
	}
	return s + "–" + e
}

// oneLine keeps a message on one terminal line.
func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	const max = 220
	if r := []rune(s); len(r) > max {
		return string(r[:max-1]) + "…"
	}
	return s
}
