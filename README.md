# Klew

**Kubernetes-native live incident investigation.**

Klew continuously combines Kubernetes workload state, events, and multi-pod logs into an evolving incident timeline — read-only, no in-cluster agent.

> Not a log viewer. Not a dashboard. A live investigation engine.
>
> **Klew** (from *clew*, the ball of thread that leads you out of a labyrinth): follow the thread from symptom to root cause.

## Install

Requires macOS and a kubeconfig for live investigations (`~/.kube/config`).

### Download

Get the latest macOS build from **[GitHub Releases](https://github.com/glnreddy421/klew/releases)**:

1. Download `Klew-x.y.z-macos-arm64.dmg` (or `.zip`)
2. Open the DMG and drag **Klew** to Applications
3. Launch Klew — signed releases open without a Gatekeeper warning

> **Older unsigned builds:** right-click the app → **Open**, or allow it in **System Settings → Privacy & Security**.

### Homebrew

```bash
brew tap glnreddy421/klew
brew trust glnreddy421/klew
brew install klew
open "$(brew --prefix)/opt/klew/Klew.app"
```

## Build from source

Requires Go 1.22+, Node.js 18+, and [Wails v2](https://wails.io).

```bash
make install-desktop-tools   # once — installs wails to $(go env GOPATH)/bin
export PATH="$(go env GOPATH)/bin:$PATH"   # add to ~/.zshrc if needed

make desktop-dev   # hot reload
make desktop       # Klew.app production build
```

See [cmd/klew-desktop/README.md](cmd/klew-desktop/README.md) and **[DEVELOPMENT.md](DEVELOPMENT.md)** for the full local workflow.

## Using Klew

Open Klew and connect to a cluster via your kubeconfig. Pick a namespace, search for a workload, and start a live investigation.

**Demo mode** (no cluster required): use the built-in demo scenarios to explore a fully simulated live investigation — payment (OOMKilled), vault (FailedMount), or checkout (ImagePullBackOff).

Namespace scope: **namespace is the investigation boundary**. Select or pin a namespace before searching; the query targets workloads inside that boundary only.

## Investigation views

Each view answers a specific question during an incident:

1. **Incident** — What broke? Status, leading signal, hypothesis, confidence, impact, and ranked evidence.
2. **Timeline** — How did it unfold? Correlated story with phase bookmarks and causal connectors.
3. **Graph** — What is the blast radius? Propagation chain over a health-grouped investigation scope.
4. **Failures** — Which runtime is failing? Severity-ranked pods and per-pod detail.
5. **Resources** — Did CPU/memory contribute? Pressure, top consumers, nodes, and recommendations.
6. **Evidence** — Why this verdict? Summary, grouped evidence, claims, cross-correlation, and gaps.

## Architecture

```
cmd/klew-desktop/     Wails + React desktop app
internal/service/     live investigation backend
internal/api/         JSON view for React
internal/kube/        client-go (no kubectl shell-out)
internal/investigation/  workload discovery + InvestigationScope + relationship graph
internal/model/       InvestigationState, EvidenceEvent, verdict
internal/engine/      evidence bus, reducer, snapshot, live session
internal/bundle/      JSON bundle I/O
internal/render/      charts, graph layout
```

Live flow: collectors publish `EvidenceEvent` → bus → reducer updates `InvestigationState` → desktop UI redraws.

### Workload discovery & investigation scope

Klew does **not** investigate namespaces — it investigates workloads; the
namespace is only the search boundary. The `internal/investigation` package owns
this foundation:

- **Discovery** searches every namespace-scoped object (exact / prefix / contains
  / label match) and groups results by workload (`WorkloadGroup`).
- **Scope builder** expands a selected workload into an `InvestigationScope` — a
  traversable relationship graph (`owns`, `selects`, `routes`, `mounts`,
  `targets`, `binds`, `uses`, `schedules`, `related`) across resource tiers:
  - **Tier 1** (always): Deployment/StatefulSet/DaemonSet/ReplicaSet/Pod,
    Service/Endpoints/EndpointSlice/Ingress, Event, Node.
  - **Tier 2** (only if referenced): ConfigMap, Secret (metadata only), PVC,
    HPA, ServiceAccount, Role, RoleBinding, NetworkPolicy, PDB.
  - **Tier 3** (operator detection): Istio, cert-manager, External Secrets, Argo
    Rollouts, KEDA, Prometheus Operator, Knative, Tekton, Argo CD, Cilium — any
    detected CRD instance related to the workload becomes a `RelatedCRD`.

Unrelated namespace objects are never pulled in. The resulting scope is attached
to `InvestigationState.Scope` so every downstream collector, correlation step and
panel operates from one workload-centric source of truth — panels never talk to
Kubernetes directly.

## v1 scope

- Deterministic signal scoring (OOMKilled, CrashLoopBackOff, zero endpoints, etc.)
- Graceful degradation when RBAC permissions are missing
- Metrics API optional (warning if unavailable)
- No AI, no backend, no cloud dependencies

## Naming

Formerly **Solid** / **solid-k8s**; renamed to **Klew**. The Go module path is
`github.com/glnreddy421/klew`.
