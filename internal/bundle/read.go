package bundle

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/glnreddy421/klew/internal/model"
)

const formatVersion = "klew.bundle.v1"
const legacyFormatVersion = "solid.bundle.v1"

type envelope struct {
	Format       string                    `json:"format"`
	Bundle       model.EvidenceBundle      `json:"bundle"`
	LiveEvidence []model.EvidenceEvent     `json:"liveEvidence,omitempty"`
	Timeline     []model.TimelineEvent     `json:"timeline,omitempty"`
	Graph        model.WorkloadGraph       `json:"graph,omitempty"`
	Verdict      model.Verdict             `json:"verdict,omitempty"`
}

// Write serializes an evidence bundle to a JSON file.
func Write(path string, b model.EvidenceBundle) error {
	return WriteState(path, model.InvestigationState{Snapshot: b, Query: b.Query, KubeContext: b.KubeContext})
}

// WriteState persists snapshot plus optional live investigation artifacts.
func WriteState(path string, st model.InvestigationState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		return fmt.Errorf("create bundle dir: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create bundle: %w", err)
	}
	defer f.Close()
	b := st.ToBundle()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	env := envelope{
		Format: formatVersion, Bundle: b,
		LiveEvidence: st.LiveEvidence,
		Timeline:     st.Timeline,
		Graph:        st.WorkloadGraph,
		Verdict:      st.Verdict,
	}
	if err := enc.Encode(env); err != nil {
		return fmt.Errorf("encode bundle: %w", err)
	}
	return nil
}

// Read loads an evidence bundle from JSON.
func Read(path string) (model.EvidenceBundle, error) {
	st, err := ReadState(path)
	if err != nil {
		return model.EvidenceBundle{}, err
	}
	return st.Snapshot, nil
}

// ReadState loads a full investigation state from JSON when available.
func ReadState(path string) (model.InvestigationState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return model.InvestigationState{}, fmt.Errorf("read bundle: %w", err)
	}
	var env envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return model.InvestigationState{}, fmt.Errorf("parse bundle: %w", err)
	}
	if env.Format != "" && env.Format != formatVersion && env.Format != legacyFormatVersion {
		return model.InvestigationState{}, fmt.Errorf("unsupported bundle format %q (expected %s)", env.Format, formatVersion)
	}
	if env.Bundle.Namespace == "" && len(env.Bundle.Workloads) == 0 && len(env.Bundle.Pods) == 0 {
		var bare model.EvidenceBundle
		if err := json.Unmarshal(data, &bare); err != nil {
			return model.InvestigationState{}, fmt.Errorf("parse bare bundle: %w", err)
		}
		return engineBootstrapFromBundle(bare), nil
	}
	st := engineBootstrapFromBundle(env.Bundle)
	st.LiveEvidence = env.LiveEvidence
	if len(env.Timeline) > 0 {
		st.Timeline = env.Timeline
	}
	if len(env.Graph.Nodes) > 0 {
		st.WorkloadGraph = env.Graph
	}
	if env.Verdict.Status != "" {
		st.Verdict = env.Verdict
	}
	st.Mode = model.ModeBundle
	return st, nil
}

func engineBootstrapFromBundle(b model.EvidenceBundle) model.InvestigationState {
	st := model.NewInvestigationState(b.Query, model.ModeBundle)
	st.KubeContext = b.KubeContext
	st.NamespaceScope = model.NamespaceScope{Primary: b.Namespace}
	st.MatchedObjects = b.MatchedObjects
	st.Snapshot = b
	st.Permissions = b.Permissions
	st.Warnings = append([]string(nil), b.Warnings...)
	return st
}
