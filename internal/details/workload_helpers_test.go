package details

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestEnvVarSourceValue_SecretNeverResolvedInEnvironmentTable(t *testing.T) {
	resolver := func(sourceKind, name, key string) (string, bool) {
		if sourceKind == "Secret" {
			return "leaked", true
		}
		return "", false
	}
	env := corev1.EnvVar{
		Name: "DB_PASSWORD",
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "db-secret"},
				Key:                  "password",
			},
		},
	}
	source, value := envVarSourceValue(env, resolver)
	if source != "secret:db-secret/password" {
		t.Fatalf("source = %q", source)
	}
	if value != secretEnvPlaceholder {
		t.Fatalf("value = %q, want masked placeholder", value)
	}
}

func TestEnvVarSourceValue_ConfigMapResolvedWhenPermitted(t *testing.T) {
	resolver := func(sourceKind, name, key string) (string, bool) {
		if sourceKind == "ConfigMap" && name == "app-config" && key == "mode" {
			return "production", true
		}
		return "", false
	}
	env := corev1.EnvVar{
		Name: "MODE",
		ValueFrom: &corev1.EnvVarSource{
			ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "app-config"},
				Key:                  "mode",
			},
		},
	}
	source, value := envVarSourceValue(env, resolver)
	if source != "configMap:app-config/mode" {
		t.Fatalf("source = %q", source)
	}
	if value != "production" {
		t.Fatalf("value = %q", value)
	}
}

func TestResolvedSecretEnvRows_OnlyIncludesPermittedValues(t *testing.T) {
	containers := []corev1.Container{{
		Name: "app",
		Env: []corev1.EnvVar{
			{
				Name: "ALLOWED",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "ok"},
						Key:                  "key",
					},
				},
			},
			{
				Name: "DENIED",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "denied"},
						Key:                  "key",
					},
				},
			},
		},
	}}
	resolver := func(name, key string) secretResolveResult {
		switch name {
		case "ok":
			return secretResolveResult{value: "c2VjcmV0", found: true}
		case "denied":
			return secretResolveResult{forbidden: true}
		default:
			return secretResolveResult{}
		}
	}

	rows, notes := resolvedSecretEnvRows(containers, resolver)
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1 resolved row", len(rows))
	}
	if rows[0][1] != "ALLOWED" {
		t.Fatalf("resolved row = %v", rows[0])
	}
	if len(notes) != 1 {
		t.Fatalf("notes = %v, want permission note", notes)
	}
}
