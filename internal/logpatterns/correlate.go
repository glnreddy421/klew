package logpatterns

import (
	"fmt"
	"hash/fnv"
	"sort"
	"strings"

	"github.com/glnreddy421/klew/v2/internal/model"
)

// DefaultJaccardThreshold is the minimum active-slot Jaccard similarity required
// to link an infrastructure event pattern with an application log pattern when
// both rise and fall together.
const DefaultJaccardThreshold = 0.6

// DefaultContainmentThreshold is used for failure-class events that fire across
// many minutes (CrashLoop BackOff, OOM, …). Jaccard against a short log burst
// falls well below 60%; containment asks whether the log's active minutes fall
// inside the event's (|A∩B|/|B|).
const DefaultContainmentThreshold = 0.8

// BuildEvidenceBoard correlates Event templates against Log templates using
// synchronized volume_history / sparkline series.
//
// Two link paths (same pod required when both name pods):
//  1. Co-movement — Jaccard on active minutes ≥ threshold (default 60%).
//  2. Failure coverage — for failure-class events (BackOff, OOMKilled, Failed, …),
//     containment of the log's active minutes inside the event's ≥ 80%.
//     This catches chronic CrashLoop BackOff ↔ a one-minute FATAL without
//     lowering the Jaccard bar for everything else.
//
// Isolation: operates only on already-built template payloads — never touches
// live Miner / MetaStore state. Returned cards deep-copy constituent templates.
//
// Ordering: highest Confidence first; failure-class roots before others on a
// tie; EvidenceID ascending as a stable last key.
func BuildEvidenceBoard(
	logTemplates []model.LogTemplate,
	eventTemplates []model.LogTemplate,
	threshold float64,
	windowMinutes int,
) *model.EvidenceBoardPayload {
	if threshold <= 0 {
		threshold = DefaultJaccardThreshold
	}
	if windowMinutes <= 0 {
		windowMinutes = defaultSparklineMinutes
	}
	if len(logTemplates) == 0 || len(eventTemplates) == 0 {
		return &model.EvidenceBoardPayload{
			Cards:         nil,
			WindowMinutes: windowMinutes,
			Threshold:     threshold,
		}
	}

	type link struct {
		logIdx int
		score  float64
	}

	cards := make([]model.EvidenceCard, 0, len(eventTemplates))
	correlatedLogs := 0

	for _, ev := range eventTemplates {
		evVol := volumeForCorrelation(ev)
		if !hasActiveSlot(evVol) {
			continue
		}
		failureEv := isFailureEvent(ev)

		links := make([]link, 0, 4)
		for li, lt := range logTemplates {
			if !sharePod(ev.Pods, lt.Pods) {
				continue
			}
			logVol := volumeSeries(lt)
			if !hasActiveSlot(logVol) {
				continue
			}
			score := linkScore(evVol, logVol, threshold, failureEv, isSignalLog(lt))
			if score > 0 {
				links = append(links, link{logIdx: li, score: score})
			}
		}
		if len(links) == 0 {
			continue
		}

		sort.Slice(links, func(i, j int) bool {
			if links[i].score != links[j].score {
				return links[i].score > links[j].score
			}
			return logTemplates[links[i].logIdx].ID < logTemplates[links[j].logIdx].ID
		})

		triggered := make([]model.LogTemplate, 0, len(links))
		ids := make([]string, 0, len(links)+1)
		ids = append(ids, ev.ID)
		maxConf := 0.0
		for _, l := range links {
			cp := cloneLogTemplate(logTemplates[l.logIdx])
			triggered = append(triggered, cp)
			ids = append(ids, cp.ID)
			if l.score > maxConf {
				maxConf = l.score
			}
		}
		correlatedLogs += len(triggered)

		cards = append(cards, model.EvidenceCard{
			EvidenceID:    evidenceID(ids...),
			Confidence:    round4(maxConf),
			RootEvent:     cloneLogTemplate(ev),
			TriggeredLogs: triggered,
		})
	}

	sort.Slice(cards, func(i, j int) bool {
		if cards[i].Confidence != cards[j].Confidence {
			return cards[i].Confidence > cards[j].Confidence
		}
		fi, fj := isFailureEvent(cards[i].RootEvent), isFailureEvent(cards[j].RootEvent)
		if fi != fj {
			return fi
		}
		return cards[i].EvidenceID < cards[j].EvidenceID
	})

	return &model.EvidenceBoardPayload{
		Cards:          cards,
		WindowMinutes:  windowMinutes,
		Threshold:      threshold,
		CardCount:      len(cards),
		CorrelatedLogs: correlatedLogs,
	}
}

// linkScore returns the best qualifying score for an event↔log pair, or 0.
func linkScore(evVol, logVol []int64, jaccardThreshold float64, failureEvent bool, signalLog bool) float64 {
	j := jaccardActiveSlots(evVol, logVol)
	best := 0.0
	if j >= jaccardThreshold {
		best = j
	}
	// Rescue path: chronic failure events (many active minutes) vs a short
	// signal log burst. Jaccard is dominated by the event; containment asks
	// whether the log fired inside the failure. Routine Pulled/Created and
	// chatter logs stay on the Jaccard path only.
	if failureEvent && signalLog && densityRescue(evVol, logVol) {
		if c := containmentActiveSlots(evVol, logVol); c >= DefaultContainmentThreshold && c > best {
			best = c
		}
	}
	return best
}

// densityRescue is true when the event is busy enough that Jaccard against a
// short log is unfair (CrashLoop BackOff across the window vs one FATAL minute).
func densityRescue(evVol, logVol []int64) bool {
	ea, la := countActive(evVol), countActive(logVol)
	if ea < 4 || la == 0 || la > 2 {
		return false
	}
	return ea >= 3*la
}

func countActive(vol []int64) int {
	n := 0
	for _, v := range vol {
		if v > 0 {
			n++
		}
	}
	return n
}

// isFailureEvent reports whether an event template is a runtime/infra failure
// signal worth covering sparse logs (not routine Pulled/Created/Scheduled).
func isFailureEvent(t model.LogTemplate) bool {
	s := strings.ToLower(t.Template)
	for _, needle := range failureEventNeedles {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}

// isSignalLog reports whether a log template is worth failure-coverage linking
// (errors / warnings), so INFO chatter under a chronic BackOff stays out.
func isSignalLog(t model.LogTemplate) bool {
	switch t.Severity {
	case model.SeverityCritical, model.SeverityHigh, model.SeverityWarning:
		return true
	}
	s := strings.ToLower(t.Template)
	for _, needle := range signalLogNeedles {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}

// Bracketed reason tokens from Kubernetes events, plus a few free-text forms.
var failureEventNeedles = []string{
	"[backoff]",
	"[oomkilled]",
	"[oomkilling]",
	"[failed]",
	"[error]",
	"[unhealthy]",
	"[evicted]",
	"[killing]",
	"[crashloopbackoff]",
	"[failedscheduling]",
	"[failedmount]",
	"[failedcreate]",
	"[failedbinding]",
	"[networknotready]",
	"back-off restarting",
	"crashloopbackoff",
	"oomkilled",
}

var signalLogNeedles = []string{
	"fatal",
	"panic",
	"oomkilled",
	"out of memory",
	"exception",
	"traceback",
	"error",
	"denied",
	"refused",
	"timeout",
	"unavailable",
	"unhealthy",
}

// sharePod reports whether a and b have a pod in common, or either is empty.
func sharePod(a, b []string) bool {
	if len(a) == 0 || len(b) == 0 {
		return true
	}
	for _, x := range a {
		for _, y := range b {
			if x == y {
				return true
			}
		}
	}
	return false
}

// jaccardActiveSlots computes |A∩B| / |A∪B| over minutes where volume > 0.
// Series are aligned oldest→newest; comparison length is min(len(a), len(b)).
func jaccardActiveSlots(a, b []int64) float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	if n == 0 {
		return 0
	}
	var inter, union int
	for i := 0; i < n; i++ {
		aa := a[i] > 0
		bb := b[i] > 0
		if aa || bb {
			union++
		}
		if aa && bb {
			inter++
		}
	}
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

// containmentActiveSlots is |A∩B| / |B| — how much of B's activity falls inside A.
// Used so a chronic BackOff (dense A) still covers a one-minute FATAL (sparse B).
func containmentActiveSlots(a, b []int64) float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	if n == 0 {
		return 0
	}
	var inter, bActive int
	for i := 0; i < n; i++ {
		bb := b[i] > 0
		if !bb {
			continue
		}
		bActive++
		if a[i] > 0 {
			inter++
		}
	}
	if bActive == 0 {
		return 0
	}
	return float64(inter) / float64(bActive)
}

// volumeSeries prefers VolumeHistory, falls back to Sparkline.
func volumeSeries(t model.LogTemplate) []int64 {
	if len(t.VolumeHistory) > 0 {
		return t.VolumeHistory
	}
	return t.Sparkline
}

// volumeForCorrelation returns the event series used for linking. Kubernetes
// often collapses a long CrashLoop into one Event with a huge Count and a
// single LastTimestamp, so the sparkline is one hot minute. For failure-class
// events with that shape, expand activity across the window so containment can
// still cover a same-pod signal log that landed in another recent minute.
func volumeForCorrelation(t model.LogTemplate) []int64 {
	vol := volumeSeries(t)
	if !isFailureEvent(t) || t.Count < 10 || countActive(vol) != 1 {
		return vol
	}
	out := make([]int64, len(vol))
	for i := range out {
		out[i] = 1
	}
	return out
}

func hasActiveSlot(vol []int64) bool {
	for _, v := range vol {
		if v > 0 {
			return true
		}
	}
	return false
}

// evidenceID is a deterministic FNV-1a fingerprint over sorted constituent pattern IDs.
func evidenceID(ids ...string) string {
	sorted := append([]string(nil), ids...)
	sort.Strings(sorted)
	h := fnv.New32a()
	_, _ = h.Write([]byte(strings.Join(sorted, "|")))
	return fmt.Sprintf("ev_%x", h.Sum32())
}

// cloneLogTemplate deep-copies dynamic slices so Evidence Board cards stay
// isolated from later template list mutations.
func cloneLogTemplate(in model.LogTemplate) model.LogTemplate {
	out := in
	out.Sparkline = copyInt64s(in.Sparkline)
	out.VolumeHistory = copyInt64s(in.VolumeHistory)
	out.Pods = copyStrings(in.Pods)
	out.Samples = copySamples(in.Samples)
	if len(in.Keywords) == 0 {
		out.Keywords = nil
	} else {
		kw := make([]model.LogWord, len(in.Keywords))
		copy(kw, in.Keywords)
		out.Keywords = kw
	}
	return out
}

func copyInt64s(in []int64) []int64 {
	if len(in) == 0 {
		return nil
	}
	out := make([]int64, len(in))
	copy(out, in)
	return out
}
