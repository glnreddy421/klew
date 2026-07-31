package investigation

// Extension is a known operator/CRD family Klew can pull into a scope (Tier 3).
type Extension struct {
	Name  string
	Group string
	Kinds []string
}

// KnownExtensions is the catalog of operator extensions Klew recognises. When a
// cluster has the corresponding CRDs installed and instances relate to the
// workload, they are attached to the scope as RelatedCRDs.
var KnownExtensions = []Extension{
	{Name: "Istio", Group: "networking.istio.io", Kinds: []string{
		"VirtualService", "DestinationRule", "Sidecar", "AuthorizationPolicy",
		"PeerAuthentication", "RequestAuthentication", "ServiceEntry", "Telemetry", "WasmPlugin"}},
	{Name: "cert-manager", Group: "cert-manager.io", Kinds: []string{
		"Certificate", "CertificateRequest", "Issuer"}},
	{Name: "External Secrets", Group: "external-secrets.io", Kinds: []string{
		"ExternalSecret", "SecretStore"}},
	{Name: "Argo Rollouts", Group: "argoproj.io", Kinds: []string{
		"Rollout", "AnalysisRun", "Experiment", "AnalysisTemplate"}},
	{Name: "KEDA", Group: "keda.sh", Kinds: []string{
		"ScaledObject", "TriggerAuthentication"}},
	{Name: "Prometheus Operator", Group: "monitoring.coreos.com", Kinds: []string{
		"ServiceMonitor", "PodMonitor", "Probe", "PrometheusRule", "Alertmanager", "Prometheus"}},
	{Name: "Knative", Group: "serving.knative.dev", Kinds: []string{
		"Service", "Revision", "Configuration", "Route"}},
	{Name: "Tekton", Group: "tekton.dev", Kinds: []string{
		"Task", "TaskRun", "Pipeline", "PipelineRun"}},
	{Name: "Argo CD", Group: "argoproj.io", Kinds: []string{
		"Application", "AppProject"}},
	{Name: "Cilium", Group: "cilium.io", Kinds: []string{
		"CiliumNetworkPolicy", "CiliumEndpoint"}},
}

// ExtensionForKind returns the catalog extension owning a CRD kind, if known.
func ExtensionForKind(kind string) (Extension, bool) {
	for _, e := range KnownExtensions {
		for _, k := range e.Kinds {
			if k == kind {
				return e, true
			}
		}
	}
	return Extension{}, false
}
