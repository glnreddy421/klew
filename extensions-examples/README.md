# Klew extension examples

Ready-to-use extensions that teach Klew about your cluster. Each file is plain YAML. Copy the ones you want into your own folder, change them to fit, and Klew picks them up in the desktop app, `klew investigate` and `klew mcp` alike.

Extensions only read. A collector lists one resource type, and a rule checks the objects it gets with a [CEL](https://cel.dev) expression. Nothing an extension does can change the cluster. The full reference is in [docs/extensions.md](../docs/extensions.md).

## How an extension works

```
Collector ──lists──► objects ──► Rule (CEL `when`) ──match──► Finding ──► Problems and verdict
 one resource type              one object at a time            severity, state, facts
```

- A **collector** lists one resource type: in the namespace you investigate, or across the cluster with `clusterScoped: true`.
- A **rule** names a collector, or one of Klew's built-in resources (`pods`, `workloads`, `services`, `jobs`, `events`, `nodes`, …), in `resource:`. Its `when` expression runs once per object. When it is true, the rule reports a finding with a `severity`, a short `state` name and the `facts` expressions as evidence.
- Findings appear under **Problems**, grouped by the workload that owns the object. `high` and `critical` findings about objects other than pods also count toward the **verdict**, so a namespace with one is never reported healthy.

Expressions see two variables:

| Variable | What it is |
|---|---|
| `object` | One object. For a collector, the Kubernetes object (`object.metadata.name`, `object.status…`). For a built-in resource, Klew's summary with camelCase fields (`object.restartCount`, `object.containers`, `object.replicas`). |
| `ns` | The namespace being investigated. (`namespace` is a reserved word in CEL.) |

## The examples

| File | What it checks | Needs | On the Klew lab cluster |
|---|---|---|---|
| [01-pods-hygiene.yaml](01-pods-hygiene.yaml) | Containers without a memory limit, unpinned images, restart storms | Nothing: built-in `pods` | `restart-storm` matches the crash-looping billing and ratelimiter pods |
| [02-workload-standards.yaml](02-workload-standards.yaml) | Single-replica Deployments, workloads without a `team` label | Nothing: built-in `workloads` Every holdout workload runs one replica with no team label |
| [03-resource-quotas.yaml](03-resource-quotas.yaml) | ResourceQuotas at their limit | A collector for a core type | No quotas there; try the sample |
| [04-cert-manager.yaml](04-cert-manager.yaml) | Certificates not ready, renewals failing | cert-manager | Not installed; try the sample |
| [05-kyverno-cluster-policies.yaml](05-kyverno-cluster-policies.yaml) | Kyverno ClusterPolicies that are not ready | Kyverno, cluster-scoped | No policies; try the sample |
| [06-cross-namespace-dependents.yaml](06-cross-namespace-dependents.yaml) | Services in other namespaces that point into this one | List Services cluster-wide | Checks all 14 Services; none point in |

### 01: pod hygiene (built-in pods)

No collector: Klew reads pods for every investigation already, so a rule can use `resource: pods` directly. The fields are Klew's pod summary, for example `object.containers[].limitsMem`, which is absent when no limit is set, hence `!has(c.limitsMem)`. Facts use `filter`, `map` and `join` to name the containers at fault:

```
MATCH    Pod/kube-proxy-5bq2q
           fact: no memory limit: kube-proxy
```

### 02: workload standards (built-in workloads)

Team conventions as rules. `single-replica` is a `warning`, so it shows in Problems without changing the verdict. `missing-team-label` is `info`. Change `team` to the label your organisation uses.

### 03: a collector for a core type

ResourceQuotas are not read by default, so the file adds a collector. Core types need no `group`. The rule compares `status.used` with `status.hard`, key by key, and is `high`: a full quota rejects new pods, so it should count toward the verdict.

### 04 and 05: custom resources

Set `group` to the custom resource's API group. `version` is optional: Klew uses the first version that serves the resource. That matters for Kyverno, whose preferred `v2` does not serve ClusterPolicies. ClusterPolicies have no namespace, so 05 sets `clusterScoped: true`. On a cluster without the CRD, the collector reports "not installed" and the rule simply has nothing to check.

### 06: across namespaces

`clusterScoped: true` on a namespaced type lists it in every namespace. The rule then compares each object with `ns`, the namespace being investigated, and keeps Services elsewhere that point into it. This tells you who else is affected when the namespace fails. Its severity is `info`, so these findings inform without changing the verdict. A cluster-wide list needs permission to list Services across the cluster and keeps at most `limit` objects (1000 at most).

## Try them

**Without a cluster.** The [samples](samples) folder holds saved objects for the rules that need a CRD or a full quota. A dry run prints every object the rule saw and why it matched:

```bash
cd extensions-examples
klew mcp -extensions-dir . -check-extensions
klew mcp -extensions-dir . -dry-run quota-exhausted -object samples/resourcequotas.yaml
klew mcp -extensions-dir . -dry-run certificate-not-ready -object samples/certificates.yaml
klew mcp -extensions-dir . -dry-run kyverno-policy-not-ready -object samples/clusterpolicies.yaml
# -namespace with -object sets ns; nothing is read from the cluster
klew mcp -extensions-dir . -dry-run external-dependents -object samples/services.yaml -namespace holdout
```

**Against your cluster, read-only:**

```bash
klew mcp -extensions-dir . -dry-run-all -namespace my-namespace
klew investigate -n my-namespace -extensions-dir .
```

`-dry-run-all` exits with status 1 if anything fails to load or evaluate, so you can run it in CI against a test namespace.

**In the desktop app.** Open **Settings → Extensions → Link folder…** and pick your folder. **Loaded** lists every collector and rule, and **Test** runs every rule against a namespace. Findings then appear in every investigation, next to the built-in ones.

## Writing your own

1. Start from the closest example and rename it. Names must be unique, and built-in names (`pods`, `services`, `configmaps`, …) cannot be reused for a collector.
2. Use `has()` for any field that may be missing. Objects that fail to evaluate are counted in a warning, never read as "nothing wrong".
3. Pick the severity by what it should do: `critical` and `high` change the verdict, `warning` and `info` only list.
4. Dry-run against an object you know is broken, and one you know is fine, before you rely on the rule.

Collectors cannot read Secrets or subresources such as `pods/log`. ConfigMaps come from Klew's built-in `configmaps` collector, with key names only (`object.keys`), never values.
