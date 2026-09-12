package details

import (
	"strings"

	"github.com/glnreddy421/klew/internal/model"
)

// Canonical section group IDs. Not every Kind emits every group.
const (
	GroupSummary       = "summary"
	GroupContainers    = "containers"
	GroupRelationships = "relationships"
	GroupSpec          = "spec"
	GroupRuntime       = "runtime"
	GroupEvents        = "events"
	GroupMetadata      = "metadata"
)

// ObjectDetail is the kind-aware inspector payload for desktop/TUI.
type ObjectDetail struct {
	Ref      model.ObjectRef `json:"ref"`
	Kind     string          `json:"kind"`
	Title    string          `json:"title"`
	Category string          `json:"category,omitempty"`
	Status   StatusBadge     `json:"status"`
	Summary  []Field         `json:"summary,omitempty"`
	Sections []Section       `json:"sections"`
}

// StatusBadge is the compact health chip in the inspector header.
type StatusBadge struct {
	Tone  string `json:"tone"`  // healthy|warning|critical|unknown|degraded
	Label string `json:"label"`
}

// Section is one inspector block/tab. Empty sections are omitted before return.
type Section struct {
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	Group     string     `json:"group,omitempty"`
	Fields    []Field    `json:"fields,omitempty"`
	Table     *Table     `json:"table,omitempty"`
	KeyValues      []KeyValue `json:"keyValues,omitempty"`
	Notes          []string   `json:"notes,omitempty"`
	Code           string     `json:"code,omitempty"`
	AltCode        string     `json:"altCode,omitempty"`
	AltToggleLabel string     `json:"altToggleLabel,omitempty"`
}

// Field is a single key/value row.
type Field struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// KeyValue is a label/annotation style pair.
type KeyValue struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Table is a columnar relationship/list view.
type Table struct {
	Columns     []string   `json:"columns"`
	Rows        [][]string `json:"rows"`
	Sensitive   bool       `json:"sensitive,omitempty"`   // value column holds base64; UI toggles decode
	ValueColumn int        `json:"valueColumn,omitempty"` // 0-based index; defaults to 1 when sensitive
}

// Empty reports whether the section has nothing to render.
func (s Section) Empty() bool {
	if len(s.Fields) > 0 {
		return false
	}
	if s.Table != nil && len(s.Table.Rows) > 0 {
		return false
	}
	if len(s.KeyValues) > 0 {
		return false
	}
	if len(s.Notes) > 0 {
		return false
	}
	if strings.TrimSpace(s.Code) != "" || strings.TrimSpace(s.AltCode) != "" {
		return false
	}
	return true
}
