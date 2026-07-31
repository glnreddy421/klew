package model

import "time"

// LogPatterns is the Patterns dashboard payload.
// Log* fields feed Live Tail → Patterns; Event* fields feed Infrastructure Patterns.
type LogPatterns struct {
	Templates  []LogTemplate     `json:"templates"`
	Words      []LogWord         `json:"words"`
	Attributes []LogAttribute    `json:"attributes"`
	Severity   LogSeverityHist   `json:"severity"`
	Histogram  []int             `json:"histogram"`
	Window     LogPatternsWindow `json:"window"`

	// Event* fields are patterns mined from Pod/Node/PVC Kubernetes events.
	EventTemplates  []LogTemplate     `json:"eventTemplates,omitempty"`
	EventWords      []LogWord         `json:"eventWords,omitempty"`
	EventReasons    []LogAttribute    `json:"eventReasons,omitempty"`
	EventSeverity   LogSeverityHist   `json:"eventSeverity,omitempty"`
	EventHistogram  []int             `json:"eventHistogram,omitempty"`
	EventWindow     LogPatternsWindow `json:"eventWindow,omitempty"`

	// EvidenceBoard is log↔infra event chains ranked by time-series Jaccard confidence.
	EvidenceBoard *EvidenceBoardPayload `json:"evidenceBoard,omitempty"`
}

type LogTemplate struct {
	ID       string  `json:"id"`
	Template string  `json:"template"`
	Count    int     `json:"count"`
	Pct      float64 `json:"pct"`
	// Trend is a compact UI arrow derived from TrendPct (↑ / ↓ / ·).
	Trend string `json:"trend"`
	// TrendPct is active-minute vs prior-window average (% change).
	TrendPct float64 `json:"trendPct"`
	// Sparkline is fixed-length per-minute counts (oldest → newest), gaps = 0.
	Sparkline []int64 `json:"sparkline,omitempty"`
	// VolumeHistory is the same chronological minute series used by evidence correlation.
	VolumeHistory []int64     `json:"volumeHistory,omitempty"`
	Severity      Severity    `json:"severity"`
	Pods          []string    `json:"pods"`
	Samples       []LogSample `json:"samples"`
	// Keywords are the highest-scoring terms for this template.
	Keywords []LogWord `json:"keywords,omitempty"`
	Score    float64   `json:"score"`
}

// EvidenceBoardPayload is the Evidence Board dataset: correlated infra→log chains.
type EvidenceBoardPayload struct {
	Cards          []EvidenceCard `json:"cards"`
	WindowMinutes  int            `json:"windowMinutes"`
	Threshold      float64        `json:"threshold"`
	CardCount      int            `json:"cardCount"`
	CorrelatedLogs int            `json:"correlatedLogs"`
}

// EvidenceCard groups one root infrastructure event pattern with time-aligned log patterns.
type EvidenceCard struct {
	EvidenceID    string        `json:"evidenceId"`
	Confidence    float64       `json:"confidence"` // Jaccard similarity in [0,1]
	RootEvent     LogTemplate   `json:"rootEvent"`
	TriggeredLogs []LogTemplate `json:"triggeredLogs"`
}

type LogSample struct {
	Message   string    `json:"message"`
	Pod       string    `json:"pod"`
	Container string    `json:"container"`
	Timestamp time.Time `json:"timestamp"`
	Severity  Severity  `json:"severity"`
}

type LogWord struct {
	Rank  int     `json:"rank"`
	Word  string  `json:"word"`
	Count int     `json:"count"`
	TF    int     `json:"tf"`
	IDF   float64 `json:"idf"`
	Lift  float64 `json:"lift"`
	Score float64 `json:"score"`
}

type LogAttribute struct {
	Rank  int     `json:"rank"`
	Key   string  `json:"key"`
	Count int     `json:"count"`
	TF    int     `json:"tf"`
	IDF   float64 `json:"idf"`
	Lift  float64 `json:"lift"`
	Score float64 `json:"score"`
}

type LogSeverityHist struct {
	Fatal int `json:"fatal"`
	Error int `json:"error"`
	Warn  int `json:"warn"`
	Info  int `json:"info"`
	Debug int `json:"debug"`
	Trace int `json:"trace"`
}

type LogPatternsWindow struct {
	LineCount     int    `json:"lineCount"`
	TotalLogs     int    `json:"totalLogs"`
	PatternCount  int    `json:"patternCount"`
	PodCount      int    `json:"podCount"`
	Scope         string `json:"scope"`
	HistMax int    `json:"histMax"`
	// Deprecated optional internals — omit from product UI; kept for older clients.
	WordModel     string `json:"wordModel,omitempty"`
	TemplateModel string `json:"templateModel,omitempty"`
}
