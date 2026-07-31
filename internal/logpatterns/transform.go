package logpatterns

import (
	"fmt"
	"hash/fnv"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/glnreddy421/klew/internal/model"
)

const defaultMaxKeywords = 5

// BuildLogTemplates snapshots Drain3 under Miner.mu, then builds the View payload
// entirely outside the ingestion lock path.
func BuildLogTemplates(
	miner *Miner,
	meta *MetaStore,
	totalLines int,
	now time.Time,
	maxTemplates int,
	maxKeywords int,
) []model.LogTemplate {
	if miner == nil {
		return nil
	}
	// Isolation barrier: primitive cluster fields only; lock released on return.
	clusters := miner.SnapshotClusters()
	var metaSnap map[int]PatternMetaSnap
	if meta != nil {
		metaSnap = meta.Snapshot() // deep copy under MetaStore.RLock, then release
	}
	mins := defaultSparklineMinutes
	if meta != nil {
		mins = meta.SparklineMinutes()
	}
	return BuildLogTemplatesFromSnapshot(clusters, metaSnap, totalLines, now, maxTemplates, maxKeywords, mins)
}

// BuildLogTemplatesFromSnapshot performs copy-on-build + TF–IDF with zero lock held.
// clusters/meta must already be detached snapshots.
func BuildLogTemplatesFromSnapshot(
	clusters []clusterSnap,
	meta map[int]PatternMetaSnap,
	totalLines int,
	now time.Time,
	maxTemplates int,
	maxKeywords int,
	sparklineMins int,
) []model.LogTemplate {
	if len(clusters) == 0 {
		return nil
	}
	if totalLines < 1 {
		totalLines = 1
	}
	if maxTemplates <= 0 {
		maxTemplates = 40
	}
	if maxKeywords <= 0 {
		maxKeywords = defaultMaxKeywords
	}
	if sparklineMins <= 0 {
		sparklineMins = defaultSparklineMinutes
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	out := make([]model.LogTemplate, 0, len(clusters))
	tpls := make([]string, 0, len(clusters))

	for i := range clusters {
		row, ok := clusterSnapToLogTemplate(&clusters[i], meta, totalLines, now, sparklineMins)
		if !ok {
			continue
		}
		out = append(out, row)
		tpls = append(tpls, row.Template)
	}

	attachTemplateKeywords(out, tpls, maxKeywords)

	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].ID < out[j].ID
	})
	if len(out) > maxTemplates {
		out = out[:maxTemplates]
	}
	return out
}

// PatternID returns a deterministic id for a Drain3 template string.
// TrimSpace prevents trailing whitespace / framing from skewing the FNV-1a hash.
func PatternID(template string) string {
	template = strings.TrimSpace(template)
	h := fnv.New32a()
	_, _ = h.Write([]byte(template))
	return fmt.Sprintf("tpl_%x", h.Sum32())
}

func clusterSnapToLogTemplate(
	c *clusterSnap,
	meta map[int]PatternMetaSnap,
	totalLines int,
	now time.Time,
	sparklineMins int,
) (model.LogTemplate, bool) {
	if c == nil {
		return model.LogTemplate{}, false
	}
	tpl := strings.TrimSpace(c.Template)
	if tpl == "" {
		return model.LogTemplate{}, false
	}

	count := c.Size
	sev := model.SeverityInfo
	var pods []string
	var samples []model.LogSample
	var buckets map[int64]int64

	if m, ok := meta[c.ID]; ok {
		if m.Count > 0 {
			count = m.Count
		}
		sev = m.Severity
		// Copy-on-build: isolate View slices from MetaStore mutations.
		pods = copyStrings(m.Pods)
		samples = copySamples(m.Samples)
		buckets = m.MinuteBuckets // already a copied map from Snapshot
	}

	spark := flattenBuckets(buckets, now, sparklineMins)
	// VolumeHistory is an isolated chronological copy for evidence correlation.
	vol := make([]int64, len(spark))
	copy(vol, spark)
	pct := trendPct(spark)
	arrow := trendArrow(pct)

	return model.LogTemplate{
		ID:            PatternID(tpl),
		Template:      tpl,
		Count:         count,
		Pct:           float64(count) / float64(totalLines) * 100,
		Trend:         arrow,
		TrendPct:      round4(pct),
		Sparkline:     spark,
		VolumeHistory: vol,
		Severity:      sev,
		Pods:          pods,
		Samples:       samples,
		Score:         scoreTemplate(count, sev, arrow, len(pods)),
	}, true
}

// attachTemplateKeywords ranks TF–IDF terms treating each template as one document.
// Wildcards / JSON boilerplate are stripped via tokenizeWords; sparse → Keywords=nil.
func attachTemplateKeywords(rows []model.LogTemplate, templates []string, maxKeywords int) {
	n := len(templates)
	if n == 0 || maxKeywords <= 0 {
		return
	}

	docs := make([][]string, n)
	df := make(map[string]int, 64)
	for i, tpl := range templates {
		toks := uniqueTokenList(tokenizeWords(tpl))
		docs[i] = toks
		for _, t := range toks {
			df[t]++
		}
	}

	for i := range rows {
		toks := docs[i]
		if len(toks) == 0 {
			rows[i].Keywords = nil // omitempty
			continue
		}
		scored := make([]model.LogWord, 0, len(toks))
		for _, term := range toks {
			docFreq := df[term]
			if docFreq < 1 {
				docFreq = 1
			}
			idf := math.Log(float64(n+1)/float64(docFreq+1)) + 1
			tf := 1
			scored = append(scored, model.LogWord{
				Word:  term,
				Count: tf,
				TF:    tf,
				IDF:   round4(idf),
				Lift:  1,
				Score: round4(float64(tf) * idf),
			})
		}
		sort.Slice(scored, func(a, b int) bool {
			if scored[a].Score != scored[b].Score {
				return scored[a].Score > scored[b].Score
			}
			return scored[a].Word < scored[b].Word
		})
		if len(scored) > maxKeywords {
			scored = scored[:maxKeywords]
		}
		if len(scored) == 0 {
			rows[i].Keywords = nil
			continue
		}
		for r := range scored {
			scored[r].Rank = r + 1
		}
		rows[i].Keywords = scored
	}
}

func uniqueTokenList(tokens []string) []string {
	if len(tokens) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(tokens))
	out := make([]string, 0, len(tokens))
	for _, t := range tokens {
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	return out
}

func copyStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}

func copySamples(in []model.LogSample) []model.LogSample {
	if len(in) == 0 {
		return nil
	}
	out := make([]model.LogSample, len(in))
	copy(out, in)
	return out
}
