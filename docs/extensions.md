# Extensions: custom collectors and rules

Klew's investigation engine is built from **collectors**, which read evidence from the cluster, and **rules**, which turn that evidence into signals and findings. The desktop app, `klew investigate` and `klew mcp` run the same engine, so a collector or rule you add shows up in all three.

Built-in collectors read workloads, ReplicaSets, pods (including init containers), Jobs, Services and endpoints, ingresses, events, logs, HPAs, nodes and metrics, plus these resource types:

| Area | Resources | Built-in rules report |
|---|---|---|
| Workloads | CronJobs, StatefulSets, DaemonSets | Suspended CronJobs, CronJobs whose last run failed or that missed their schedule, StatefulSet rollouts that have stalled or lack their governing Service, DaemonSets that match no node or have unavailable pods |
| Configuration | ConfigMaps (key names only) | Unready pods using a ConfigMap that does not exist |
| Ingress | IngressClasses | Missing IngressClass, no default class, backend Services that do not exist or have no ready endpoints |
| Admission webhooks | Mutating and validating webhook configurations | Webhooks served from the namespace whose Service is missing or down, critical when `failurePolicy: Fail` blocks requests cluster-wide |
| Storage | PersistentVolumeClaims, StorageClasses, PersistentVolumes | Claims Pending or Lost (with a missing StorageClass named), pods mounting a claim that does not exist, volumes Failed or Released |
| Availability | PodDisruptionBudgets | Budgets with too few healthy pods, budgets that block every eviction, budgets that select no pods |
| Traffic | NetworkPolicies | Policies that deny all ingress or egress (egress includes DNS), policies that select no pods |
| Security | ServiceAccounts, Roles, RoleBindings | Pods running as a deleted ServiceAccount, bindings to a missing Role or ServiceAccount |
| Gateway API | Gateways, GatewayClasses, HTTPRoutes | Gateways not accepted or programmed or with a missing class, routes not accepted, unresolved refs, missing backend Services |
| Admission | ValidatingAdmissionPolicies and bindings | Policies with type-checking warnings, bindings to a missing policy |
| Dynamic resource allocation | ResourceClaims | Claims not allocated |
| OLM | ClusterServiceVersions, Subscriptions, InstallPlans, CatalogSources, OperatorGroups, ClusterExtensions, ClusterCatalogs | Failed or pending operators, resolution failures, plans awaiting approval, unhealthy catalogs, too many or no OperatorGroups, extensions not installed, catalogs not serving |

Before any collector runs, Klew asks the API server whether your kubeconfig user may read what it needs (a SelfSubjectAccessReview per permission, with the right API group). A collector you may not use is not called. Every investigation records each collector's outcome: **collected**, **no permission**, **not installed** (Gateway API, OLM and resource claims on clusters without them; these are not warned about), **failed**, or **off** (Logs unticked). Gaps are stated in the investigation's warnings, for example "Not checked, no permission: nodes (needs list nodes)", and `klew mcp` returns them as `not_collected`. If a permission check itself cannot run, the collector is tried anyway and the warning says so. Read anything Klew could not check as unknown, not healthy.

Built-in rules score the verdict and list **problems**: every unhealthy workload or object in scope, with the facts behind its state. They also cover pods failing to start, pods rejected at creation (LimitRange, ResourceQuota, webhook), failed Jobs and CronJobs, failing init containers, Services whose selector matches no pods, and liveness probes restarting containers.

You add your own as YAML files. No rebuild is needed.

To start quickly, copy from [extensions-examples](../extensions-examples): six commented files with saved objects to try them without a cluster.

## Where extensions live

In the desktop app, open **Settings → Extensions**:

- **Add file…** checks a YAML file and copies it into Klew's own extensions folder. A file in which nothing loads is refused, with the reasons.
- **Link folder…** reads a folder of your own in place, such as a git repository of rules. Klew never copies or changes files in a linked folder. **Unlink** stops reading it and leaves its files alone.
- **Reload** reads every folder again. New investigations, and the next refresh of a running one, use the result; no restart is needed.
- **Loaded** lists every collector and rule, file by file, as *loaded* or *skipped* with the reason.
- **Test** runs every rule against a namespace and shows what each one matches (see [Testing a rule](#testing-a-rule)).

Klew's own folder is `~/Library/Application Support/Klew/extensions/` (Windows: `%USERPROFILE%\AppData\Roaming\Klew\extensions\`). Linked folders are listed in `extension-sources.json` next to it. The `klew` command reads both, so the app, the command line and your AI assistant always use the same extensions. To use one folder instead, such as a git repository of rules, pass it to either command: `klew investigate -extensions-dir ./team-rules` or `klew mcp -extensions-dir ./team-rules`. A folder that does not exist is an error, so a mistyped path cannot silently load no rules.

You can also put files in Klew's folder by hand; restart `klew mcp` (or Reload in the app) after editing.

A file can hold several documents separated by `---`. A document that fails to load is skipped, and the reason appears in Settings and in the warnings of every investigation (and on stderr for the `klew` command). Names must be unique across all folders, and built-in names cannot be redefined.

## Collectors

A collector lists one resource type in the investigation's namespace (or across the cluster with `clusterScoped: true`) and keeps the objects for rules.

```yaml
apiVersion: klew.dev/v1alpha1
kind: Collector
metadata:
  name: certificates          # rules refer to this name
spec:
  group: cert-manager.io      # omit for the core group
  version: v1                 # optional; omit to use the first version that serves the resource
  resource: certificates      # plural resource name
  clusterScoped: false        # default: the investigation namespace only
  limit: 200                  # objects kept; default 200, max 1000
```

Collectors can only **list**. They cannot collect Secrets or subresources such as `pods/log`, nor ConfigMaps: Klew's built-in `configmaps` collector already lists them with their **key names only** (`object.keys`), never their values, and rules can use it. The objects they list are given to rules but are not sent to the desktop UI or to MCP clients; only the findings rules produce from them are. `managedFields` and the `last-applied-configuration` annotation are dropped. If your kubeconfig user cannot list the resource, the investigation reports a warning and carries on.

When `version` is left out, Klew uses the group's preferred version if it serves the resource, and otherwise the first other version that does. A group's preferred version does not always serve every resource in it: `kyverno.io` prefers `v2`, but ClusterPolicies are served by `v1`.

### Across namespaces

With `clusterScoped: true` on a namespaced type, a collector lists that type in every namespace, not just the investigated one. Its rules then see objects from all namespaces, so compare each object with `ns` to keep what relates to the investigation. This rule finds Services in other namespaces that point into the investigated one, which tells you who else is affected when it fails:

```yaml
apiVersion: klew.dev/v1alpha1
kind: Collector
metadata:
  name: all-services
spec:
  resource: services
  clusterScoped: true
  limit: 1000
---
apiVersion: klew.dev/v1alpha1
kind: Rule
metadata:
  name: external-dependents
spec:
  resource: all-services
  when: >-
    object.metadata.namespace != ns && has(object.spec.externalName) &&
    object.spec.externalName.contains('.' + ns + '.svc')
  severity: info
  state: DependsOnThisNamespace
  facts:
    - "object.metadata.namespace + '/' + object.metadata.name + ' -> ' + object.spec.externalName"
```

A cluster-wide list needs permission to list the type across the cluster, and keeps at most `limit` objects (1000 at most). Findings join the investigation's problems like any other, so give rules about other namespaces a severity that matches how much they matter here: `high` and `critical` findings count toward the verdict.

## Rules

A rule checks one object at a time with a [CEL](https://cel.dev) expression, the language Kubernetes uses for validating admission policies. When the expression is true, the rule reports a finding.

```yaml
apiVersion: klew.dev/v1alpha1
kind: Rule
metadata:
  name: certificate-not-ready
spec:
  resource: certificates      # a collector name, or a built-in resource (below)
  when: "object.status.conditions.exists(c, c.type == 'Ready' && c.status != 'True')"
  severity: high              # critical, high, warning (default) or info
  state: CertificateNotReady  # a short name for the problem
  facts:                      # optional CEL expressions that return strings
    - "object.status.conditions.filter(c, c.type == 'Ready')[0].message"
```

Expressions see two variables:

- `object`: the object as JSON. For a collector this is the Kubernetes object (`object.metadata.name`, `object.status…`). For a built-in resource it is Klew's summary, with camelCase fields as in the desktop app's data (for example `object.restartCount`, `object.containers`, `object.phase`).
- `ns`: the investigation namespace. (`namespace` is a reserved word in CEL, so a rule that uses it is refused with a pointer to `ns`; the object's own `object.metadata.namespace` works as usual.)

Built-in resources: `pods`, `services`, `workloads`, `replicasets`, `jobs`, `events`, `nodes`, and the name of any built-in resource collector above, such as `persistentvolumeclaims`, `networkpolicies` or `gateways`, which give rules the full Kubernetes object.

A rule or collector that fails, even by crashing, is skipped with a warning; the rest of the investigation carries on.

### When an expression goes wrong

Nothing a rule does can stop an investigation or change the cluster, and mistakes are reported rather than read as "nothing wrong":

| Mistake | What happens |
|---|---|
| YAML, `apiVersion`, `kind`, an unknown field, severity or state is wrong | The document is skipped when it loads, with the reason |
| CEL syntax error, or `when` does not return true/false, or a fact does not return a string | The rule is skipped when it loads |
| `resource` names no collector or built-in resource (`certifcates`) | The rule is skipped when it loads, with a suggestion: *Did you mean "certificates"?* |
| A field is missing on some objects | Those objects are skipped; the warning counts them and quotes the first error |
| `when` returns something other than true or false for an object | That object is skipped and counted, as above |
| A fact fails for an object | The finding is kept without that fact; the warning counts failed facts |
| More than 200 objects match | The rule stops at 200 findings and says how many objects it did not check |
| An expression is too expensive | It stops at the CEL cost limit and is reported as an evaluation error |

Use `has()` for fields that may be missing, such as `has(object.status.phase) && object.status.phase != 'Bound'`.

A rule can still be wrong without failing: `c.status == 'True'` where you meant `!=` runs fine and reports the opposite. Test a rule against an object you know is broken before relying on it.

### Testing a rule

`klew mcp` can check your extensions and dry-run one rule without starting an investigation. Both are read-only.

```sh
# What loaded, and every problem (exit status 1 if anything was skipped)
klew mcp -check-extensions

# One rule against objects you saved
kubectl get certificates -n shop -o yaml > certs.yaml
klew mcp -dry-run certificate-not-ready -object certs.yaml

# Saved objects from every namespace, checked as if investigating holdout (ns)
kubectl get services -A -o yaml > services.yaml
klew mcp -dry-run external-dependents -object services.yaml -namespace holdout

# One rule against a namespace, read live
klew mcp -dry-run certificate-not-ready -namespace shop

# Every rule in a folder against a namespace, read once
klew mcp -extensions-dir ./my-rules -dry-run-all -namespace shop
```

`-dry-run-all` prints each rule's matches and errors (objects that did not match are counted, not listed), then a summary line per rule. It exits with status 1 if any extension failed to load or any object could not be evaluated, so it can check a folder of rules in CI against a test namespace. It needs `-namespace`: a saved-objects file holds one type, so test those rules one at a time with `-object`.

A dry run lists every object the rule saw, with `MATCH` and its facts, `no match`, or `ERROR` and the reason, then the warnings the rule would add to an investigation. Rules over Klew's summaries (`pods`, `services`, `workloads`, `replicasets`, `jobs`, `events`, `nodes`) need `-namespace`, because a saved Kubernetes object does not have the summary's shape. Use `-context` and `-kubeconfig` to pick the cluster, and `-extensions-dir` to test a folder other than the default.

### Investigating custom objects

In the desktop app's Investigate mode, the **Target** list comes from the engine's collectors, grouped by type, so a custom collector's objects appear there under their own heading (marked *custom*) as soon as it lists any in the namespace. Picking one of them, or passing `Certificate/web-tls` as `workload` to `klew_investigate`, investigates exactly that object: its own events and the findings your rules report about it.

What can be investigated is decided in one place: the collectors. Any namespaced type a collector lists, built-in or custom, is investigable. In the desktop app, the inspector's telescope button investigates the selected object when its type is investigable. For other types, such as Leases or ResourceQuotas, the button is disabled and says why: Klew has no collector for them. Adding a collector for a type makes it investigable everywhere, with no other change. An investigation asked for such a type says so ("Klew has no collector for Lease") instead of reporting the object as not found.

### How findings are used

- **Problems.** Findings are grouped by the workload that owns the object (a pod's Deployment, StatefulSet, Job or CronJob), so each workload appears once, at its worst severity. Objects with no owner appear under their own kind and name.
- **Verdict.** Findings with severity `critical` or `high` about objects other than pods also become signals, so they count toward the verdict, and a namespace with one is not reported as healthy.
- **Read-only.** Rules only read what collectors gathered. Nothing an extension does can change the cluster.

## Example: a rule over a built-in collector

Rules can read what the built-in collectors list, with no collector of their own. This one flags PersistentVolumeClaims asking for more than 100Gi:

```yaml
apiVersion: klew.dev/v1alpha1
kind: Rule
metadata: {name: large-claim}
spec:
  resource: persistentvolumeclaims
  when: "object.spec.resources.requests.storage.endsWith('Ti') || (object.spec.resources.requests.storage.endsWith('Gi') && int(object.spec.resources.requests.storage.replace('Gi', '')) > 100)"
  severity: info
  state: LargeClaim
  facts:
    - "'claim ' + object.metadata.name + ' requests ' + object.spec.resources.requests.storage"
```

## Data handling

Facts from extensions are treated like any other cluster text. `klew mcp` passes them through the same best-effort redaction and length cap before returning them. Redaction can miss sensitive values, so avoid writing facts that copy whole fields which may hold credentials.
