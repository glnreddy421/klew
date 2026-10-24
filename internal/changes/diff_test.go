package changes

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/glnreddy421/klew/internal/model"
)

func template(c ...corev1.Container) *corev1.PodTemplateSpec {
	return &corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: c}}
}

func findDiff(t *testing.T, diffs []model.FieldDiff, path string) model.FieldDiff {
	t.Helper()
	for _, d := range diffs {
		if d.Path == path {
			return d
		}
	}
	t.Fatalf("no diff for %q in %+v", path, diffs)
	return model.FieldDiff{}
}

func TestDiffImage(t *testing.T) {
	d := DiffPodTemplates(
		template(corev1.Container{Name: "app", Image: "repo/payment-api:v2.3.1"}),
		template(corev1.Container{Name: "app", Image: "repo/payment-api:v2.4.0"}),
	)
	got := findDiff(t, d, "containers[app].image")
	if got.Kind != model.DiffChanged || got.Area != AreaImage || got.After != "repo/payment-api:v2.4.0" {
		t.Fatalf("image diff = %+v", got)
	}
	if s := Summarize(d); s != "image v2.3.1 → v2.4.0" {
		t.Fatalf("summary = %q", s)
	}
}

func TestDiffEnvAddRemoveChange(t *testing.T) {
	d := DiffPodTemplates(
		template(corev1.Container{Name: "app", Env: []corev1.EnvVar{
			{Name: "DB_POOL_SIZE", Value: "20"}, {Name: "OLD_FLAG", Value: "on"},
		}}),
		template(corev1.Container{Name: "app", Env: []corev1.EnvVar{
			{Name: "DB_POOL_SIZE", Value: "5"}, {Name: "NEW_FLAG", Value: "off"},
		}}),
	)
	if c := findDiff(t, d, "containers[app].env.DB_POOL_SIZE"); c.Kind != model.DiffChanged || c.Before != "20" || c.After != "5" {
		t.Fatalf("changed = %+v", c)
	}
	if c := findDiff(t, d, "containers[app].env.OLD_FLAG"); c.Kind != model.DiffRemoved {
		t.Fatalf("removed = %+v", c)
	}
	if c := findDiff(t, d, "containers[app].env.NEW_FLAG"); c.Kind != model.DiffAdded || c.After != "off" {
		t.Fatalf("added = %+v", c)
	}
	if s := Summarize(d); !strings.Contains(s, "env DB_POOL_SIZE 20 → 5") {
		t.Fatalf("summary = %q", s)
	}
}

func TestDiffSecretKeyRefShownAsRefNeverValue(t *testing.T) {
	ref := func(name, key string) *corev1.EnvVarSource {
		return &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: name}, Key: key}}
	}
	d := DiffPodTemplates(
		template(corev1.Container{Name: "app", Env: []corev1.EnvVar{{Name: "DB_PASS", ValueFrom: ref("db-v1", "password")}}}),
		template(corev1.Container{Name: "app", Env: []corev1.EnvVar{{Name: "DB_PASS", ValueFrom: ref("db-v2", "password")}}}),
	)
	c := findDiff(t, d, "containers[app].env.DB_PASS")
	if c.Before != "secret/db-v1:password" || c.After != "secret/db-v2:password" {
		t.Fatalf("secret ref diff = %+v", c)
	}
}

func TestDiffRedactsSecretLikePlainValues(t *testing.T) {
	d := DiffPodTemplates(
		template(corev1.Container{Name: "app", Env: []corev1.EnvVar{
			{Name: "API_TOKEN", Value: "old-token-value"},
			{Name: "DATABASE_URL", Value: "postgres://app:s3cr3t@db:5432/pay"},
			{Name: "LOG_LEVEL", Value: "info"},
		}}),
		template(corev1.Container{Name: "app", Env: []corev1.EnvVar{
			{Name: "API_TOKEN", Value: "new-token-value"},
			{Name: "DATABASE_URL", Value: "postgres://app:n3w@db:5432/pay"},
			{Name: "LOG_LEVEL", Value: "debug"},
		}}),
	)
	tok := findDiff(t, d, "containers[app].env.API_TOKEN")
	if tok.Before != Redacted || tok.After != Redacted || tok.Kind != model.DiffChanged {
		t.Fatalf("token must be redacted yet reported as changed: %+v", tok)
	}
	dsn := findDiff(t, d, "containers[app].env.DATABASE_URL")
	if dsn.Before != Redacted || dsn.After != Redacted {
		t.Fatalf("connection string with credentials must be redacted: %+v", dsn)
	}
	if lvl := findDiff(t, d, "containers[app].env.LOG_LEVEL"); lvl.Before != "info" || lvl.After != "debug" {
		t.Fatalf("plain values are shown: %+v", lvl)
	}
	for _, x := range d {
		if strings.Contains(x.Before+x.After, "s3cr3t") || strings.Contains(x.Before+x.After, "token-value") {
			t.Fatalf("secret leaked: %+v", x)
		}
	}
}

func TestRedactValue(t *testing.T) {
	cases := map[[2]string]bool{
		{"PASSWORD", "hunter2"}:        true,
		{"CACHE_KEY_PREFIX", "orders"}: true, // broad on purpose
		{"GREETING", "hello world"}:    false,
		{"GREETING", "eyJhbGciOiJIUzI1.eyJzdWIiOiIxMjM0NTY.SflKxwRJSMeKKF2"}: true,
		{"ENDPOINT", "https://user:pw@api.example.com"}:                      true,
		{"ENDPOINT", "https://api.example.com/v1"}:                           false,
		{"IMAGE", "registry.example.com/team/app:v1.2.3"}:                    false,
		{"CLIENT", "AKIAIOSFODNN7EXAMPLE"}:                                   true,
		{"RANDOM", "Zm9vYmFyYmF6cXV4MTIzNDU2Nzg5MGFiY2RlZg"}:                 true,
		{"COUNT", "12345678901234567890123456789012345"}:                     false,
	}
	for in, masked := range cases {
		got := RedactValue(in[0], in[1]) == Redacted
		if got != masked {
			t.Errorf("RedactValue(%q, %q) masked=%v, want %v", in[0], in[1], got, masked)
		}
	}
}

func TestDiffResourcesAndProbes(t *testing.T) {
	prev := corev1.Container{Name: "app",
		Resources:      corev1.ResourceRequirements{Limits: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("256Mi")}},
		ReadinessProbe: &corev1.Probe{ProbeHandler: corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{Path: "/ready", Port: intstr.FromInt32(8080)}}, PeriodSeconds: 10},
	}
	next := prev
	next.Resources = corev1.ResourceRequirements{
		Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("128Mi")},
		Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")},
	}
	next.ReadinessProbe = &corev1.Probe{ProbeHandler: corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{Path: "/healthz", Port: intstr.FromInt32(8080)}}, PeriodSeconds: 10}
	d := DiffPodTemplates(template(prev), template(next))
	if m := findDiff(t, d, "containers[app].resources.limits.memory"); m.Before != "256Mi" || m.After != "128Mi" || m.Area != AreaResources {
		t.Fatalf("memory = %+v", m)
	}
	if c := findDiff(t, d, "containers[app].resources.requests.cpu"); c.Kind != model.DiffAdded || c.After != "100m" {
		t.Fatalf("cpu = %+v", c)
	}
	if p := findDiff(t, d, "containers[app].readinessProbe"); p.Area != AreaProbes || !strings.Contains(p.After, "/healthz") {
		t.Fatalf("probe = %+v", p)
	}
}

func TestDiffContainerAddRemove(t *testing.T) {
	d := DiffPodTemplates(
		template(corev1.Container{Name: "app", Image: "a:1"}, corev1.Container{Name: "sidecar", Image: "s:1"}),
		template(corev1.Container{Name: "app", Image: "a:1"}, corev1.Container{Name: "proxy", Image: "p:2"}),
	)
	if r := findDiff(t, d, "containers[sidecar]"); r.Kind != model.DiffRemoved || r.Before != "s:1" {
		t.Fatalf("removed = %+v", r)
	}
	if a := findDiff(t, d, "containers[proxy]"); a.Kind != model.DiffAdded || a.After != "p:2" {
		t.Fatalf("added = %+v", a)
	}
	if s := Summarize(d); !strings.Contains(s, "container +proxy") || !strings.Contains(s, "container −sidecar") {
		t.Fatalf("summary = %q", s)
	}
}

func TestDiffRestartedAtAndIgnoredHashes(t *testing.T) {
	prev := template(corev1.Container{Name: "app", Image: "a:1"})
	prev.Labels = map[string]string{"app": "pay", "pod-template-hash": "aaa"}
	next := template(corev1.Container{Name: "app", Image: "a:1"})
	next.Labels = map[string]string{"app": "pay", "pod-template-hash": "bbb"}
	next.Annotations = map[string]string{"kubectl.kubernetes.io/restartedAt": "2026-10-21T20:00:00Z"}

	d := DiffPodTemplates(prev, next)
	if len(d) != 1 || d[0].Area != AreaRestart || d[0].Path != "Restarted (rollout restart)" {
		t.Fatalf("want only the restart, got %+v", d)
	}
	if s := Summarize(d); s != "restarted (rollout restart)" {
		t.Fatalf("summary = %q", s)
	}
}

func TestDiffPodLevelFields(t *testing.T) {
	prev := template(corev1.Container{Name: "app"})
	prev.Spec.ServiceAccountName = "default"
	prev.Spec.NodeSelector = map[string]string{"pool": "general"}
	prev.Spec.Volumes = []corev1.Volume{{Name: "cfg", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "cfg-v1"}}}}}
	next := template(corev1.Container{Name: "app"})
	next.Spec.ServiceAccountName = "payments"
	next.Spec.NodeSelector = map[string]string{"pool": "memory"}
	next.Spec.Tolerations = []corev1.Toleration{{Key: "dedicated", Operator: corev1.TolerationOpEqual, Value: "pay", Effect: corev1.TaintEffectNoSchedule}}
	next.Spec.Volumes = []corev1.Volume{{Name: "cfg", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "cfg-v2"}}}}}

	d := DiffPodTemplates(prev, next)
	findDiff(t, d, "serviceAccountName")
	findDiff(t, d, "nodeSelector.pool")
	findDiff(t, d, "tolerations")
	if v := findDiff(t, d, "volumes.cfg"); v.Before != "configmap/cfg-v1" || v.After != "configmap/cfg-v2" {
		t.Fatalf("volume = %+v", v)
	}
}

func TestFirstRevisionHasNoDiffs(t *testing.T) {
	if d := DiffPodTemplates(nil, template(corev1.Container{Name: "app"})); d != nil {
		t.Fatalf("first revision diffs = %+v", d)
	}
}

func TestSummaryStaysShort(t *testing.T) {
	var diffs []model.FieldDiff
	for i := 0; i < 30; i++ {
		diffs = append(diffs, model.FieldDiff{Path: "containers[app].env.VERY_LONG_VARIABLE_NAME_" + string(rune('A'+i%26)),
			Before: "something-rather-long", After: "something-else-long", Kind: model.DiffChanged, Area: AreaEnv})
	}
	s := Summarize(diffs)
	if len([]rune(s)) > MaxSummaryLen || !strings.Contains(s, "more") {
		t.Fatalf("summary too long or missing remainder (%d): %q", len([]rune(s)), s)
	}
}
