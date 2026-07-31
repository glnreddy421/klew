package tfidf

import (
	"math"
	"sort"
	"strings"
	"sync"
	"unicode"
)

// Term is a scored token for a single Drain3 cluster document.
type Term struct {
	Word  string
	TF    int
	IDF   float64
	Score float64
}

// Engine treats each Drain3 cluster template as one document and maintains
// live DF / TF–IDF statistics. Safe for concurrent use.
type Engine struct {
	mu sync.RWMutex

	// docTokens[clusterID] = unique terms currently representing that template.
	docTokens map[int]map[string]struct{}
	// tf[clusterID][term] within that document (usually 1 after sanitize; kept for generality).
	tf map[int]map[string]int
	// df[term] = number of cluster documents containing term.
	df map[string]int
	// templates[clusterID] last ingested template string (for change detection).
	templates map[int]string
	nDocs     int
}

// New returns an empty Engine.
func New() *Engine {
	return &Engine{
		docTokens: make(map[int]map[string]struct{}),
		tf:        make(map[int]map[string]int),
		df:        make(map[string]int),
		templates: make(map[int]string),
	}
}

// UpsertDocument registers or refreshes the document for a Drain3 cluster.
// No-op when the template text is unchanged.
func (e *Engine) UpsertDocument(clusterID int, template string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if prev, ok := e.templates[clusterID]; ok && prev == template {
		return
	}

	tokens := tokenizeTemplate(template)
	uniq := uniqueCounts(tokens)

	if old, ok := e.docTokens[clusterID]; ok {
		for t := range old {
			e.df[t]--
			if e.df[t] <= 0 {
				delete(e.df, t)
			}
		}
	} else {
		e.nDocs++
	}

	set := make(map[string]struct{}, len(uniq))
	tf := make(map[string]int, len(uniq))
	for t, c := range uniq {
		set[t] = struct{}{}
		tf[t] = c
		e.df[t]++
	}
	e.docTokens[clusterID] = set
	e.tf[clusterID] = tf
	e.templates[clusterID] = template
}

// RemoveDocument drops a cluster document (e.g. LRU eviction). Optional helper.
func (e *Engine) RemoveDocument(clusterID int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	old, ok := e.docTokens[clusterID]
	if !ok {
		return
	}
	for t := range old {
		e.df[t]--
		if e.df[t] <= 0 {
			delete(e.df, t)
		}
	}
	delete(e.docTokens, clusterID)
	delete(e.tf, clusterID)
	delete(e.templates, clusterID)
	e.nDocs--
	if e.nDocs < 0 {
		e.nDocs = 0
	}
}

// TopN returns the highest TF–IDF terms for a cluster. Thread-safe.
func (e *Engine) TopN(clusterID int, n int) []Term {
	e.mu.RLock()
	defer e.mu.RUnlock()

	tf, ok := e.tf[clusterID]
	if !ok || n <= 0 {
		return nil
	}
	nDocs := e.nDocs
	if nDocs < 1 {
		nDocs = 1
	}

	out := make([]Term, 0, len(tf))
	for term, tfCount := range tf {
		df := e.df[term]
		if df < 1 {
			df = 1
		}
		idf := math.Log(float64(nDocs+1)/float64(df+1)) + 1
		out = append(out, Term{
			Word:  term,
			TF:    tfCount,
			IDF:   round4(idf),
			Score: round4(float64(tfCount) * idf),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Word < out[j].Word
	})
	if len(out) > n {
		out = out[:n]
	}
	return out
}

// DocCount returns the number of registered cluster documents.
func (e *Engine) DocCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.nDocs
}

var stopwords = map[string]struct{}{
	"a": {}, "an": {}, "the": {}, "to": {}, "of": {}, "in": {}, "on": {}, "for": {},
	"with": {}, "and": {}, "or": {}, "at": {}, "by": {}, "from": {}, "is": {}, "are": {},
	"was": {}, "were": {}, "be": {}, "been": {}, "as": {}, "it": {}, "this": {}, "that": {},
	"msg": {}, "message": {}, "log": {}, "level": {}, "time": {}, "timestamp": {}, "ts": {},
	"severity": {}, "stream": {}, "pod": {}, "container": {}, "namespace": {},
	"null": {}, "true": {}, "false": {}, "http": {}, "https": {},
	"info": {}, "warn": {}, "warning": {}, "error": {}, "err": {}, "fatal": {}, "debug": {}, "trace": {},
}

// tokenizeTemplate strips Drain wildcards / mask tokens and structural noise.
func tokenizeTemplate(template string) []string {
	replacer := strings.NewReplacer(
		"<*>", " ",
		"<IP>", " ",
		"<IPV6>", " ",
		"<UUID>", " ",
		"<TS>", " ",
		"<POD>", " ",
		"<NUM>", " ",
		"{", " ",
		"}", " ",
		"[", " ",
		"]", " ",
		`"`, " ",
		`'`, " ",
		",", " ",
		":", " ",
		"=", " ",
	)
	s := strings.ToLower(replacer.Replace(template))

	var out []string
	var b strings.Builder
	flush := func() {
		if b.Len() == 0 {
			return
		}
		w := b.String()
		b.Reset()
		if len(w) < 3 {
			return
		}
		if _, stop := stopwords[w]; stop {
			return
		}
		if w[0] >= '0' && w[0] <= '9' {
			return
		}
		if len(w) > 48 {
			w = w[:48]
		}
		out = append(out, w)
	}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_' || r == '-' {
			b.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	return out
}

func uniqueCounts(tokens []string) map[string]int {
	m := make(map[string]int, len(tokens))
	for _, t := range tokens {
		m[t]++
	}
	return m
}

func round4(n float64) float64 {
	return math.Round(n*10000) / 10000
}
