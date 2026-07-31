package logpatterns

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/glnreddy421/klew/internal/model"
)

type tfDoc struct {
	tokens []string
	ts     time.Time
	weight int
}

type scoredTerm struct {
	term  string
	count int
	tf    int
	idf   float64
	lift  float64
	score float64
}

var stopwords = map[string]struct{}{
	"a": {}, "an": {}, "the": {}, "to": {}, "of": {}, "in": {}, "on": {}, "for": {},
	"with": {}, "and": {}, "or": {}, "at": {}, "by": {}, "from": {}, "is": {}, "are": {},
	"was": {}, "were": {}, "be": {}, "been": {}, "as": {}, "it": {}, "this": {}, "that": {},
	"not": {}, "no": {}, "yes": {}, "http": {}, "https": {}, "get": {}, "post": {},
	"put": {}, "patch": {}, "delete": {}, "null": {}, "true": {}, "false": {},
	"info": {}, "warn": {}, "warning": {}, "error": {}, "err": {}, "fatal": {},
	"debug": {}, "trace": {}, "level": {}, "msg": {}, "message": {}, "time": {},
	"timestamp": {}, "ts": {}, "ms": {}, "ns": {}, "pod": {}, "container": {},
	"kube": {}, "k8s": {}, "via": {}, "into": {}, "over": {}, "under": {},
	"than": {}, "then": {}, "also": {}, "just": {}, "only": {},
}

// drainPlaceholders must never enter TF–IDF term space (Drain3 wildcards / masks).
var drainPlaceholderReplacer = strings.NewReplacer(
	"<*>", " ",
	"<ip>", " ",
	"<ipv6>", " ",
	"<uuid>", " ",
	"<ts>", " ",
	"<pod>", " ",
	"<num>", " ",
	"***", " ", // legacy hand-rolled Drain wildcard, if echoed into messages
)

func stripDrainPlaceholders(s string) string {
	return drainPlaceholderReplacer.Replace(s)
}

func tokenizeWords(message string) []string {
	// Strip Drain3 placeholders + lowercase before regex tokenization so "<*>",
	// punctuation, and structural JSON fragments cannot enter TF–IDF space.
	// reWord only keeps [a-z][a-z0-9_./:-]{2,} — braces/quotes/commas are dropped.
	s := stripDrainPlaceholders(strings.ToLower(message))
	matches := reWord.FindAllString(s, -1)
	out := make([]string, 0, len(matches))
	for _, w := range matches {
		if strings.Contains(w, "://") {
			continue
		}
		if w[0] >= '0' && w[0] <= '9' {
			continue
		}
		if isDrainNoiseToken(w) {
			continue
		}
		if _, ok := stopwords[w]; ok {
			continue
		}
		if i := strings.LastIndexByte(w, '/'); i >= 0 {
			w = w[i+1:]
		}
		if len(w) < 3 {
			continue
		}
		if isDrainNoiseToken(w) {
			continue
		}
		if _, ok := stopwords[w]; ok {
			continue
		}
		if len(w) > 48 {
			w = w[:48]
		}
		out = append(out, w)
	}
	return out
}

func isDrainNoiseToken(w string) bool {
	switch w {
	case "*", "<*>", "***":
		return true
	default:
		// Residual angle-bracket mask tokens (e.g. "<ip>") if strip missed a variant.
		return strings.Contains(w, "<*>") || (strings.HasPrefix(w, "<") && strings.HasSuffix(w, ">"))
	}
}

func extractAttributes(message string) []string {
	matches := reAttr.FindAllStringSubmatch(message, -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		key := m[1]
		if _, ok := stopwords[strings.ToLower(key)]; ok {
			continue
		}
		switch strings.ToUpper(key) {
		case "DEBUG", "INFO", "WARN", "WARNING", "ERROR", "FATAL", "TRACE":
			continue
		}
		out = append(out, key)
	}
	return out
}

// rankTopWordsTfIdf ranks free-text tokens with residual TF–IDF.
func rankTopWordsTfIdf(docs []tfDoc, maxWords int) []model.LogWord {
	scored := rankTfIdfResidual(docs, maxWords)
	out := make([]model.LogWord, len(scored))
	for i, s := range scored {
		out[i] = model.LogWord{
			Rank:  i + 1,
			Word:  s.term,
			Count: s.count,
			TF:    s.tf,
			IDF:   s.idf,
			Lift:  s.lift,
			Score: s.score,
		}
	}
	return out
}

// rankTopFieldsTfIdf ranks structured key=value field names with the same residual TF–IDF model.
func rankTopFieldsTfIdf(docs []tfDoc, maxFields int) []model.LogAttribute {
	scored := rankTfIdfResidual(docs, maxFields)
	out := make([]model.LogAttribute, len(scored))
	for i, s := range scored {
		out[i] = model.LogAttribute{
			Rank:  i + 1,
			Key:   s.term,
			Count: s.count,
			TF:    s.tf,
			IDF:   s.idf,
			Lift:  s.lift,
			Score: s.score,
		}
	}
	return out
}

// rankTfIdfResidual: score = tf_now × idf × ln(1 + tf_now/(tf_base+1))
// Each doc is a log line; tokens are either words or field keys.
func rankTfIdfResidual(docs []tfDoc, maxN int) []scoredTerm {
	if len(docs) == 0 || maxN <= 0 {
		return nil
	}
	sorted := append([]tfDoc(nil), docs...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ts.Before(sorted[j].ts)
	})

	var baseline, current []tfDoc
	if len(sorted) >= 4 {
		split := len(sorted) / 2
		if split < 1 {
			split = 1
		}
		baseline = sorted[:split]
		current = sorted[split:]
	} else {
		current = sorted
	}

	nDocs := len(sorted)
	df := map[string]int{}
	tfAll := map[string]int{}
	tfNow := map[string]int{}
	tfBase := map[string]int{}

	for _, d := range sorted {
		w := d.weight
		if w <= 0 {
			w = 1
		}
		seen := map[string]struct{}{}
		for _, t := range d.tokens {
			seen[t] = struct{}{}
			tfAll[t] += w
		}
		for t := range seen {
			df[t]++
		}
	}
	for _, d := range current {
		w := d.weight
		if w <= 0 {
			w = 1
		}
		for _, t := range d.tokens {
			tfNow[t] += w
		}
	}
	for _, d := range baseline {
		w := d.weight
		if w <= 0 {
			w = 1
		}
		for _, t := range d.tokens {
			tfBase[t] += w
		}
	}

	useResidual := len(baseline) > 0
	scored := make([]scoredTerm, 0, len(tfNow))
	for term, tf := range tfNow {
		if tf < 1 {
			continue
		}
		docFreq := df[term]
		if docFreq < 1 {
			docFreq = 1
		}
		idf := math.Log(float64(nDocs+1)/float64(docFreq+1)) + 1
		base := tfBase[term]
		lift := 1.0
		residual := 1.0
		if useResidual {
			lift = float64(tf) / float64(base+1)
			residual = math.Log(1 + lift)
		}
		score := float64(tf) * idf * residual
		scored = append(scored, scoredTerm{
			term:  term,
			count: tfAll[term],
			tf:    tf,
			idf:   round4(idf),
			lift:  round4(lift),
			score: round4(score),
		})
	}
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		if scored[i].count != scored[j].count {
			return scored[i].count > scored[j].count
		}
		return scored[i].term < scored[j].term
	})
	if len(scored) > maxN {
		scored = scored[:maxN]
	}
	return scored
}

func round4(n float64) float64 {
	return math.Round(n*10000) / 10000
}
