package tfidf_test

import (
	"testing"

	"github.com/glnreddy421/klew/tools/k8s-log-engine/internal/tfidf"
)

func TestTopNRanksDistinctiveTerms(t *testing.T) {
	e := tfidf.New()
	e.UpsertDocument(1, "health check ok status ready")
	e.UpsertDocument(2, "dial tcp connect connection refused")
	e.UpsertDocument(3, "payment failed timeout connection refused")

	top := e.TopN(3, 3)
	if len(top) == 0 {
		t.Fatal("expected keywords")
	}
	found := false
	for _, term := range top {
		if term.Word == "payment" || term.Word == "timeout" || term.Word == "failed" {
			found = true
			if term.Score <= 0 {
				t.Fatalf("expected positive score: %#v", term)
			}
		}
	}
	if !found {
		t.Fatalf("expected distinctive payment terms, got %#v", top)
	}
}

func TestUpsertSkipsUnchangedTemplate(t *testing.T) {
	e := tfidf.New()
	e.UpsertDocument(1, "user login failed")
	if e.DocCount() != 1 {
		t.Fatalf("docs=%d", e.DocCount())
	}
	e.UpsertDocument(1, "user login failed")
	if e.DocCount() != 1 {
		t.Fatalf("docs changed unexpectedly: %d", e.DocCount())
	}
	e.UpsertDocument(1, "user login <*> failed")
	if e.DocCount() != 1 {
		t.Fatalf("docs=%d after template change", e.DocCount())
	}
}

func TestStripsWildcardsAndStructuralKeys(t *testing.T) {
	e := tfidf.New()
	e.UpsertDocument(1, `{"msg":"payment <*> failed","level":"error"}`)
	top := e.TopN(1, 10)
	for _, term := range top {
		switch term.Word {
		case "msg", "level", "error":
			t.Fatalf("structural/stopword leaked: %#v", top)
		}
	}
}
