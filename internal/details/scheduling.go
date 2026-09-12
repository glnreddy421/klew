package details

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

func schedulingSummaryPairs(spec corev1.PodSpec) []string {
	var pairs []string
	if sel := selectorString(spec.NodeSelector); sel != "" {
		pairs = append(pairs, "Node Selector", sel)
	}
	if s := tolerationsSummaryString(spec.Tolerations); s != "" {
		pairs = append(pairs, "Tolerations", s)
	}
	if spec.Affinity != nil {
		if s := affinityDetailSummary(spec.Affinity); s != "" {
			pairs = append(pairs, "Affinity", s)
		}
	}
	return pairs
}

func appendSchedulingSummaryPairs(pairs []string, spec corev1.PodSpec) []string {
	return append(pairs, schedulingSummaryPairs(spec)...)
}

func schedulingSections(spec corev1.PodSpec) []Section {
	var sections []Section
	if rows := nodeSelectorRows(spec.NodeSelector); len(rows) > 0 {
		sections = append(sections, sectionTable("nodeSelector", "Node Selector", GroupSummary,
			[]string{"Key", "Value"}, rows))
	}
	if rows := podTolerationRows(spec.Tolerations); len(rows) > 0 {
		sections = append(sections, sectionTable("tolerations", "Tolerations", GroupSummary,
			[]string{"Key", "Operator", "Value", "Effect", "Seconds"}, rows))
	}
	if spec.Affinity != nil {
		if rows := nodeAffinityRows(spec.Affinity.NodeAffinity); len(rows) > 0 {
			sections = append(sections, sectionTable("nodeAffinity", "Node Affinity", GroupSummary,
				[]string{"Type", "Weight", "Topology", "Match"}, rows))
		}
		if rows := podAffinityRows(spec.Affinity.PodAffinity); len(rows) > 0 {
			sections = append(sections, sectionTable("podAffinity", "Pod Affinity", GroupSummary,
				[]string{"Type", "Weight", "Topology", "Namespaces", "Match"}, rows))
		}
		if rows := podAntiAffinityRows(spec.Affinity.PodAntiAffinity); len(rows) > 0 {
			sections = append(sections, sectionTable("podAntiAffinity", "Pod Anti-Affinity", GroupSummary,
				[]string{"Type", "Weight", "Topology", "Namespaces", "Match"}, rows))
		}
	}
	return sections
}

func podTolerationRows(tols []corev1.Toleration) [][]string {
	var rows [][]string
	for _, t := range tols {
		sec := ""
		if t.TolerationSeconds != nil {
			sec = fmt.Sprintf("%d", *t.TolerationSeconds)
		}
		rows = append(rows, []string{
			t.Key,
			string(t.Operator),
			t.Value,
			string(t.Effect),
			sec,
		})
	}
	return rows
}

func nodeSelectorRows(selector map[string]string) [][]string {
	if len(selector) == 0 {
		return nil
	}
	kv := kvMap(selector)
	rows := make([][]string, 0, len(kv))
	for _, item := range kv {
		rows = append(rows, []string{item.Key, item.Value})
	}
	return rows
}

func tolerationsSummaryString(tols []corev1.Toleration) string {
	if len(tols) == 0 {
		return ""
	}
	parts := make([]string, 0, len(tols))
	for _, t := range tols {
		part := strings.TrimSpace(t.Key)
		if t.Value != "" {
			part += "=" + t.Value
		}
		if t.Operator != "" && t.Operator != corev1.TolerationOpEqual {
			part += " (" + string(t.Operator) + ")"
		}
		if t.Effect != "" {
			part += ":" + string(t.Effect)
		}
		if t.TolerationSeconds != nil {
			part += fmt.Sprintf(" %ds", *t.TolerationSeconds)
		}
		if part != "" {
			parts = append(parts, part)
		}
	}
	return truncateSummary(strings.Join(parts, ", "))
}

func taintsSummaryString(taints []corev1.Taint) string {
	if len(taints) == 0 {
		return ""
	}
	parts := make([]string, 0, len(taints))
	for _, t := range taints {
		part := strings.TrimSpace(t.Key)
		if t.Value != "" {
			part += "=" + t.Value
		}
		if t.Effect != "" {
			part += ":" + string(t.Effect)
		}
		if part != "" {
			parts = append(parts, part)
		}
	}
	return truncateSummary(strings.Join(parts, ", "))
}

func truncateSummary(s string) string {
	if len(s) <= 160 {
		return s
	}
	return s[:157] + "…"
}

func affinityDetailSummary(affinity *corev1.Affinity) string {
	if affinity == nil {
		return ""
	}
	var parts []string
	if affinity.NodeAffinity != nil {
		for _, term := range affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms {
			if s := nodeSelectorTermString(term); s != "" {
				parts = append(parts, "node required: "+s)
			}
		}
		for _, pref := range affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
			if s := nodeSelectorTermString(pref.Preference); s != "" {
				parts = append(parts, fmt.Sprintf("node preferred(%d): %s", pref.Weight, s))
			}
		}
	}
	if affinity.PodAffinity != nil {
		for _, term := range affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution {
			parts = append(parts, podAffinitySummaryLine("pod required", term))
		}
		for _, pref := range affinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
			parts = append(parts, podAffinitySummaryLine(fmt.Sprintf("pod preferred(%d)", pref.Weight), pref.PodAffinityTerm))
		}
	}
	if affinity.PodAntiAffinity != nil {
		for _, term := range affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution {
			parts = append(parts, podAffinitySummaryLine("pod anti required", term))
		}
		for _, pref := range affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
			parts = append(parts, podAffinitySummaryLine(fmt.Sprintf("pod anti preferred(%d)", pref.Weight), pref.PodAffinityTerm))
		}
	}
	return truncateSummary(strings.Join(parts, "; "))
}

func podAffinitySummaryLine(prefix string, term corev1.PodAffinityTerm) string {
	var bits []string
	if term.TopologyKey != "" {
		bits = append(bits, term.TopologyKey)
	}
	if sel := labelSelectorSummary(term.LabelSelector); sel != "" {
		bits = append(bits, sel)
	}
	if len(bits) == 0 {
		return prefix
	}
	return prefix + ": " + strings.Join(bits, " ")
}

func nodeAffinityRows(na *corev1.NodeAffinity) [][]string {
	if na == nil {
		return nil
	}
	var rows [][]string
	for _, term := range na.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms {
		rows = append(rows, []string{
			"Required",
			"",
			"",
			nodeSelectorTermString(term),
		})
	}
	for _, pref := range na.PreferredDuringSchedulingIgnoredDuringExecution {
		weight := ""
		if pref.Weight != 0 {
			weight = fmt.Sprintf("%d", pref.Weight)
		}
		rows = append(rows, []string{
			"Preferred",
			weight,
			"",
			nodeSelectorTermString(pref.Preference),
		})
	}
	return rows
}

func nodeSelectorTermString(term corev1.NodeSelectorTerm) string {
	var parts []string
	for _, expr := range term.MatchExpressions {
		parts = append(parts, labelRequirementString(expr.Key, string(expr.Operator), expr.Values))
	}
	for _, expr := range term.MatchFields {
		parts = append(parts, labelRequirementString(expr.Key, string(expr.Operator), expr.Values))
	}
	return strings.Join(parts, ", ")
}

func podAffinityRows(pa *corev1.PodAffinity) [][]string {
	if pa == nil {
		return nil
	}
	var rows [][]string
	for _, term := range pa.RequiredDuringSchedulingIgnoredDuringExecution {
		rows = append(rows, podAffinityTermRow("Required", 0, term))
	}
	for _, pref := range pa.PreferredDuringSchedulingIgnoredDuringExecution {
		rows = append(rows, podAffinityTermRow("Preferred", pref.Weight, pref.PodAffinityTerm))
	}
	return rows
}

func podAntiAffinityRows(paa *corev1.PodAntiAffinity) [][]string {
	if paa == nil {
		return nil
	}
	var rows [][]string
	for _, term := range paa.RequiredDuringSchedulingIgnoredDuringExecution {
		rows = append(rows, podAffinityTermRow("Required", 0, term))
	}
	for _, pref := range paa.PreferredDuringSchedulingIgnoredDuringExecution {
		rows = append(rows, podAffinityTermRow("Preferred", pref.Weight, pref.PodAffinityTerm))
	}
	return rows
}

func podAffinityTermRow(kind string, weight int32, term corev1.PodAffinityTerm) []string {
	w := ""
	if weight != 0 {
		w = fmt.Sprintf("%d", weight)
	}
	ns := strings.Join(term.Namespaces, ",")
	if term.NamespaceSelector != nil {
		if sel := labelSelectorSummary(term.NamespaceSelector); sel != "" {
			if ns != "" {
				ns += "; "
			}
			ns += "ns:" + sel
		}
	}
	return []string{
		kind,
		w,
		term.TopologyKey,
		ns,
		labelSelectorSummary(term.LabelSelector),
	}
}

func labelRequirementString(key, op string, values []string) string {
	return fmt.Sprintf("%s %s [%s]", key, op, strings.Join(values, ","))
}
