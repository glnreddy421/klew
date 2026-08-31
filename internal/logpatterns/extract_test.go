package logpatterns_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/glnreddy421/klew/internal/logpatterns"
	"github.com/glnreddy421/klew/internal/model"
)

func TestExtractDrain3MergesVariableIPs(t *testing.T) {
	now := time.Now().UTC()
	ev := []model.EvidenceEvent{
		logEv("payment-api", "app", "payment-api/app: dial tcp 10.0.2.14:5432: connect: connection refused", model.SeverityHigh, now.Add(-3*time.Second)),
		logEv("payment-api", "app", "payment-api/app: dial tcp 10.0.2.15:5432: connect: connection refused", model.SeverityHigh, now.Add(-2*time.Second)),
		logEv("payment-api", "app", "payment-api/app: dial tcp 10.0.2.19:5432: connect: connection refused", model.SeverityHigh, now.Add(-1*time.Second)),
		logEv("payment-api", "app", "payment-api/app: GET /health 200 12ms userId=u1", model.SeverityInfo, now),
	}
	out := logpatterns.Extract(ev, nil, logpatterns.Options{})
	if out.Window.PatternCount == 0 {
		t.Fatalf("expected patterns, got %#v", out.Window)
	}
	var refused *model.LogTemplate
	for i := range out.Templates {
		tpl := strings.ToLower(out.Templates[i].Template)
		if strings.Contains(tpl, "refused") || strings.Contains(tpl, "dial") || strings.Contains(tpl, "connect") {
			refused = &out.Templates[i]
			break
		}
	}
	if refused == nil {
		t.Fatalf("expected refused/dial template, got %#v", out.Templates)
	}
	if refused.Count < 3 {
		t.Fatalf("expected count>=3, got %d tpl=%s", refused.Count, refused.Template)
	}
	if !strings.Contains(refused.Template, "<IP>") && !strings.Contains(refused.Template, "<*>") {
		t.Fatalf("expected IP/wildcard masking in template, got %q", refused.Template)
	}
	if len(out.Words) == 0 || out.Words[0].Score <= 0 {
		t.Fatalf("expected TF-IDF words, got %#v", out.Words)
	}
	foundUser := false
	for _, a := range out.Attributes {
		if a.Key == "userId" {
			foundUser = true
			if a.Score <= 0 {
				t.Fatalf("expected TF-IDF score on userId field, got %#v", a)
			}
		}
	}
	if !foundUser {
		t.Fatalf("expected userId field, got %#v", out.Attributes)
	}
}

func TestRankTopFieldsTfIdfPromotesEmergingKeys(t *testing.T) {
	now := time.Now().UTC()
	var ev []model.EvidenceEvent
	for i := 0; i < 20; i++ {
		ev = append(ev, logEv("p", "c",
			fmt.Sprintf("health check ok status=ready ts=%d", i),
			model.SeverityInfo, now.Add(time.Duration(i)*time.Second)))
	}
	for i := 20; i < 30; i++ {
		ev = append(ev, logEv("p", "c",
			fmt.Sprintf("payment failed userId=u%d error.code=TIMEOUT status=ready", i),
			model.SeverityHigh, now.Add(time.Duration(i)*time.Second)))
	}
	out := logpatterns.Extract(ev, nil, logpatterns.Options{MaxAttrs: 5})
	if len(out.Attributes) == 0 {
		t.Fatal("expected ranked fields")
	}
	top := map[string]float64{}
	for _, a := range out.Attributes {
		top[a.Key] = a.Score
	}
	if top["userId"] == 0 && top["error.code"] == 0 {
		t.Fatalf("expected emerging fields near top, got %#v", out.Attributes)
	}
	if st, ok := top["status"]; ok {
		if uid := top["userId"]; uid > 0 && st > uid*3 {
			t.Fatalf("status score unexpectedly dominates: %#v", out.Attributes)
		}
	}
}

func TestPatternIDStableAcrossWindows(t *testing.T) {
	now := time.Now().UTC()
	ev := []model.EvidenceEvent{
		logEv("payment-api", "app", "dial tcp 10.0.2.14:5432: connect: connection refused", model.SeverityHigh, now),
		logEv("payment-api", "app", "dial tcp 10.0.2.15:5432: connect: connection refused", model.SeverityHigh, now.Add(time.Second)),
		logEv("auth", "app", "dial tcp 10.1.0.9:5432: connect: connection refused", model.SeverityHigh, now.Add(2*time.Second)),
	}
	a := logpatterns.Extract(ev, nil, logpatterns.Options{})
	b := logpatterns.Extract(ev, nil, logpatterns.Options{})
	if len(a.Templates) == 0 {
		t.Fatal("expected templates")
	}
	byTpl := map[string]string{}
	for _, row := range a.Templates {
		byTpl[row.Template] = row.ID
		if row.ID != logpatterns.PatternID(row.Template) {
			t.Fatalf("id not derived from template: %s != %s", row.ID, logpatterns.PatternID(row.Template))
		}
	}
	for _, row := range b.Templates {
		if id, ok := byTpl[row.Template]; !ok || id != row.ID {
			t.Fatalf("PatternID not stable across windows for %q: %q vs %q", row.Template, id, row.ID)
		}
	}
	foundKW := false
	for _, row := range a.Templates {
		if len(row.Keywords) > 0 {
			foundKW = true
			break
		}
	}
	if !foundKW {
		t.Fatalf("expected per-template TF-IDF keywords, got %#v", a.Templates)
	}
}

func TestTokenizeWordsStripsDrainWildcards(t *testing.T) {
	// Exported via Extract path: templates with <*> must not pollute top words.
	now := time.Now().UTC()
	ev := []model.EvidenceEvent{
		logEv("p", "c", "user login failed reason=bad_password", model.SeverityHigh, now),
		logEv("p", "c", "user <*> logged in from <IP>", model.SeverityInfo, now.Add(time.Second)),
		logEv("p", "c", "payment <*> timeout after retries", model.SeverityHigh, now.Add(2*time.Second)),
	}
	out := logpatterns.Extract(ev, nil, logpatterns.Options{MaxWords: 20})
	for _, w := range out.Words {
		if w.Word == "*" || w.Word == "<*>" || strings.Contains(w.Word, "<*>") {
			t.Fatalf("Drain wildcard leaked into TF-IDF words: %#v", out.Words)
		}
		if strings.HasPrefix(w.Word, "<") && strings.HasSuffix(w.Word, ">") {
			t.Fatalf("mask token leaked into TF-IDF words: %#v", out.Words)
		}
	}
}

func TestMaxClustersBound(t *testing.T) {
	now := time.Now().UTC()
	var ev []model.EvidenceEvent
	for i := 0; i < 80; i++ {
		ev = append(ev, logEv("p", "c",
			fmt.Sprintf("unique event type-%d code=%d detail=noise-%d", i, i, i),
			model.SeverityInfo, now.Add(time.Duration(i)*time.Millisecond)))
	}
	out := logpatterns.Extract(ev, nil, logpatterns.Options{MaxClusters: 16, MaxTemplates: 40})
	if len(out.Templates) > 16 {
		t.Fatalf("expected templates capped by MaxClusters/UI max, got %d", len(out.Templates))
	}
	// Drain3 LRU keeps ≤ MaxClusters; UI then caps to MaxTemplates.
	if out.Window.PatternCount > 40 {
		t.Fatalf("patternCount=%d", out.Window.PatternCount)
	}
}

func logEv(pod, container, msg string, sev model.Severity, ts time.Time) model.EvidenceEvent {
	return model.EvidenceEvent{
		Timestamp:  model.TimestampFrom(ts),
		SourceType: model.SourceLog,
		Pod:        pod,
		Container:  container,
		Message:    msg,
		Severity:   sev,
		Count:      1,
	}
}
