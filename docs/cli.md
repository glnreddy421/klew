# Klew command line

The `klew` command runs Klew's engine from a terminal. It has two commands:

- `klew investigate` prints an investigation for a person: what the desktop app shows in Overview, Failures, Patterns and Evidence, without the graph.
- `klew mcp` serves the same engine to AI assistants. See [mcp.md](mcp.md).

Both are **read-only**: they list, get and watch cluster objects and never change them. They use your kubeconfig and its RBAC, like `kubectl`.

## Install

Homebrew (macOS or Linux):

```bash
brew install glnreddy421/klew/klew-cli
```

Or with Go 1.26+:

```bash
go install github.com/glnreddy421/klew/v2/cmd/klew@latest
```

Both install the same `klew` binary. Release tarballs are described in [mcp.md](mcp.md#other-ways).

## klew investigate

```bash
klew investigate -n shop                 # the whole namespace
klew investigate -n shop checkout        # one workload in it
klew investigate -n shop -only failures  # just the pod triage
```

It collects pods, events, logs and workload changes, watches for new events and logs for `-observe` (10 seconds by default), then prints:

| Section | What it shows |
|---|---|
| Header | Verdict (healthy, warning, critical), leading signal, confidence, summary, pod and endpoint counts |
| Overview | A related workload change, every problem with the facts behind it, the causal chain, the hypothesis and alternatives, incidents, and a timeline of warnings |
| Failures | Failing pods, worst first: each container's state, exit codes and restarts, and the pod's recent events. Admission webhooks when workloads expect pods but none exist |
| Patterns | Log and event patterns: count, share, trend (↑ ↓), pods and an example line |
| Evidence | Event ↔ log correlations, scored claims, events and changes, collectors that could not run, gaps, next checks and suggested actions |

Each list shows its top 10; `-all` shows everything. When the desktop app has recorded events, they are included and marked `(recorded)`, so you can see events Kubernetes has already expired.

Suggested actions are text. Klew never runs them.

### Options

| Option | |
|---|---|
| `-n`, `-namespace` | Namespace. Default: the context's namespace, else `default` |
| `WORKLOAD` | Narrows the investigation: `api`, `deploy/api`, `pod/api-7f9c` |
| `-context`, `-kubeconfig` | Which cluster, as for `kubectl` |
| `-observe 30s` | How long to watch before reporting, up to 5m. `0s` reports right after collecting |
| `-only a,b` | Print only these sections: `overview`, `failures`, `patterns`, `evidence` |
| `-all` | Print every item instead of the top 10 |
| `-o json` | Print the report as JSON, with every field |
| `-no-logs` | Do not read container logs |
| `-history-dir`, `-no-history` | Where the desktop app's recorded events are, or do not read them |
| `-extensions-dir DIR` | Use only this folder of [custom collectors and rules](extensions.md), instead of Klew's extensions folder and the folders linked in the desktop app |

`klew investigate -h` lists them all.

### Exit status

| Status | Meaning |
|---|---|
| 0 | Healthy (or not enough data to say otherwise) |
| 1 | Error, such as no cluster access or a missing namespace |
| 2 | Usage error |
| 3 | Warning |
| 4 | Critical |

So it works in scripts and CI:

```bash
klew investigate -n shop -observe 0s -only overview || echo "shop needs attention"
klew investigate -n shop -o json | jq -r '.overview.problems[] | "\(.severity) \(.object) \(.state)"'
```

### What it prints

Output is **not redacted**: log lines and event messages appear as they are, like `kubectl logs`. That differs from `klew mcp`, whose results go to an AI provider and are masked first (best effort). Review terminal or JSON output before sharing it. Secret values are never read or shown; only Secret names appear.

Color is used on a terminal, and turned off when output is piped, when `NO_COLOR` is set, or when `TERM=dumb`.
