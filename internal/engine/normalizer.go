package engine

import (
	"github.com/glnreddy421/klew/internal/kube"
	"github.com/glnreddy421/klew/internal/model"
)

// NormalizeEvents delegates to kube event normalization for bundle ingestion.
func NormalizeEvents(events []model.EventRecord) []model.EventRecord {
	return kube.NormalizeEvents(events)
}
