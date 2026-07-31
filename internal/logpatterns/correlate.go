package logpatterns

import (
	"fmt"
	"hash/fnv"
	"sort"
	"strings"

	"github.com/glnreddy421/klew/internal/model"
)

// DefaultJaccardThreshold is the minimum active-slot Jaccard similarity required
// to link an infrastructure event pattern with an application log pattern.
const DefaultJaccardThreshold = 0.6

// BuildEvidenceBoard correlates Event templates against Log templates using
// synchronized volume_history / sparkline series (Jaccard on active minutes).
//
// Isolation: operates only on already-built template payloads — never touches
// live Miner / MetaStore state. Returned cards deep-copy constituent templates.
//
// Ordering: highest Confidence first; EvidenceID ascending as a stable tie-break.
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

	for ei := range eventTemplates {
		evVol := volumeSeries(eventTemplates[ei])
		if !hasActiveSlot(evVol) {
			continue
		}

		links := make([]link, 0, 4)
		for li := range logTemplates {
			logVol := volumeSeries(logTemplates[li])
			score := jaccardActiveSlots(evVol, logVol)
			if score >= threshold {
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
		ids = append(ids, eventTemplates[ei].ID)
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
			RootEvent:     cloneLogTemplate(eventTemplates[ei]),
			TriggeredLogs: triggered,
		})
	}

	sort.Slice(cards, func(i, j int) bool {
		if cards[i].Confidence != cards[j].Confidence {
			return cards[i].Confidence > cards[j].Confidence
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

// volumeSeries prefers VolumeHistory, falls back to Sparkline.
func volumeSeries(t model.LogTemplate) []int64 {
	if len(t.VolumeHistory) > 0 {
		return t.VolumeHistory
	}
	return t.Sparkline
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
