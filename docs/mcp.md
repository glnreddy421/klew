# Klew MCP server

`klew mcp`, a subcommand of the [`klew` command](cli.md), gives AI assistants — Claude Code, Claude Desktop, Cursor, and any other [Model Context Protocol](https://modelcontextprotocol.io) client — Klew's incident analysis instead of raw `kubectl` output. It runs the same engine as the desktop app and is **read-only**: it lists, gets and watches cluster objects and never changes them.

It does not need the desktop app. When the app is installed, `klew mcp` also reads the events it recorded, so an assistant can see events Kubernetes has already expired.

## Install

The server needs a working kubeconfig, the same as `kubectl`.

### Homebrew (macOS and Linux)

```bash
brew install glnreddy421/klew/klew-cli
claude mcp add --scope user klew -- klew mcp
```

The formula installs the `klew` command. Upgrade with `brew upgrade klew-cli`. Add flags after `klew mcp` (see [Flags](#flags)), for example `claude mcp add --scope user klew -- klew mcp -no-log-samples`. Options for `claude` go before `klew`: anything after `--` is passed to the server. For other clients, see [Add it to a client](#add-it-to-a-client).

### From the desktop app (macOS)

The app has the same server built in, so desktop users need nothing else. Open **Set up MCP** on the Home screen (or **Tools → Set Up MCP…**). Pick the server (built into the app, or a standalone `klew` command it found), choose a default context and flags, and copy the ready-made line for Claude Code, Claude Desktop, Cursor or another client. The built-in server runs as `<path to Klew.app>/Contents/MacOS/Klew mcp`, takes the same flags as `klew mcp`, and opens no window. The dialog only shows text to copy; it changes no client configuration.

### Other ways

For machines without Homebrew, or to pin a version.

**Download a release.** Every Klew release has `klew-cli` archives of the `klew` command for macOS and Linux on arm64 and amd64. The macOS binaries are signed with Klew's Developer ID and notarized by Apple.

```bash
VERSION=2.0.0  # the release you want
OS=darwin      # or linux
ARCH=arm64     # or amd64
curl -fsSLO "https://github.com/glnreddy421/klew/releases/download/v${VERSION}/klew-cli-${VERSION}-${OS}-${ARCH}.tar.gz"

# Optional: check where it came from before installing.
gh attestation verify "klew-cli-${VERSION}-${OS}-${ARCH}.tar.gz" --repo glnreddy421/klew
# Or, without gh, against the release checksums (Linux: sha256sum -c --ignore-missing):
curl -fsSLO "https://github.com/glnreddy421/klew/releases/download/v${VERSION}/Klew-${VERSION}-SHA256SUMS.txt"
shasum -a 256 -c --ignore-missing "Klew-${VERSION}-SHA256SUMS.txt"

tar -xzf "klew-cli-${VERSION}-${OS}-${ARCH}.tar.gz" klew
sudo install -m 0755 klew /usr/local/bin/klew
```

**Build from source** (Go 1.26+):

```bash
go install github.com/glnreddy421/klew/cmd/klew@latest
```

The binary lands in `$(go env GOPATH)/bin`.

## Add it to a client

**Claude Code**

```bash
claude mcp add --scope user klew -- klew mcp
```

`--scope user` adds it for every project; without it, the server is only available in the folder you ran the command in. Start a new Claude Code session afterwards, since one already open does not pick up the change. If it says `klew` already exists, `claude mcp get klew` shows which server and scope it uses; remove it with `claude mcp remove klew -s <scope>` and add it again.

**Claude Desktop, Cursor and other JSON-configured clients**

```json
{
  "mcpServers": {
    "klew": {
      "command": "klew",
      "args": ["mcp"]
    }
  }
}
```

If the client cannot find `klew` (apps started from the Dock often lack Homebrew's folder on `PATH`), use the full path that `command -v klew` prints. With the desktop app, use `"command": "/Applications/Klew.app/Contents/MacOS/Klew", "args": ["mcp"]`.

Use the absolute path: desktop clients start servers without your shell's `PATH`. `klew mcp` loads `PATH`, `KUBECONFIG` and cloud CLI variables from your login shell itself, so exec auth plugins (`aws`, `gke-gcloud-auth-plugin`, `kubelogin`) keep working.

### Flags

`klew mcp -h` (or `Klew mcp -h` for the app's server) lists these with examples for each client.

| Flag                | Default                                                             | Purpose                                                                                                                                                                |
| ------------------- | ------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `-kubeconfig`       | `KUBECONFIG` or `~/.kube/config`                                    | Kubeconfig to read                                                                                                                                                     |
| `-context`          | kubeconfig current-context                                          | Default context when a tool call names none                                                                                                                            |
| `-history-dir`      | `~/Library/Application Support/Klew/history`                        | Where the desktop app keeps recorded events                                                                                                                            |
| `-no-history`       | off                                                                 | Ignore recorded events                                                                                                                                                 |
| `-no-log-samples`   | off                                                                 | Leave raw container log text out of results (see below)                                                                                                                |
| `-extensions-dir`   | Klew's extensions folder plus the folders linked in the desktop app | Read only this folder of custom collectors and rules instead (see [extensions.md](extensions.md))                                                                      |
| `-check-extensions` | off                                                                 | Load the extensions folder, print what loaded and every problem, and exit (exit status 1 if anything was skipped)                                                      |
| `-dry-run-all`      | off                                                                 | Run every user rule in the extensions folder against `-namespace NS`, read once, and print a summary; exit status 1 on any load problem or evaluation error; read-only |
| `-dry-run RULE`     | —                                                                   | Run one user rule with `-object FILE` or `-namespace NS` and print each object's result, then exit; read-only (see [extensions.md](extensions.md#testing-a-rule))      |
| `-version`          |                                                                     | Print the version and exit                                                                                                                                             |


## Tools


| Tool                         | What it returns                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
| ---------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `klew_list_contexts`         | Contexts in your kubeconfig. Reads the file only.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
| `klew_investigate`           | A verdict for a namespace or workload: status, leading signal, likely trigger, confidence, causal chain, `problems` (every unhealthy workload or Service with its state and key facts such as exit reason, memory limit, readiness probe, declared ports, CPU and memory requests, image and mounts), failing pods, top log and event patterns, warning incidents, a related workload change, and next checks. The leading signal and suggested actions follow the strongest problem only. `problems` comes from the engine's rules, the same list the desktop app uses, including any [custom rules](extensions.md), and `headline` sums it up in one line ("7 problems: 4 critical, 3 high. Worst: …"). Collects a snapshot, then watches live for `observe_seconds` (default 10, max 60; 0 skips the watch). Without `workload` it covers every pod in the namespace. With `workload` set to `Kind/name` (`Deployment/api`, `Service/web`, `ConfigMap/settings`, `PersistentVolumeClaim/data`, or any kind a collector lists, custom ones included) it covers exactly that object: the pods it reaches through owners, selectors, Ingress backends or references, and the objects related to them, never other objects whose names merely contain it. Several targets can be given comma-separated, and the investigation covers all of them; any that do not exist are named in the warnings. A bare name covers every object with exactly that name; other text is matched loosely. `not_collected` lists collectors that were skipped (no permission, logs off) or failed, so what they cover is unchecked rather than healthy; Klew checks each permission with the API server before running a collector. One namespace per call: to check several, call once for each. Sends progress notifications when the client requests them. |
| `klew_investigation_details` | One section of an earlier investigation, paged: `pods`, `signals`, `log_patterns`, `event_patterns`, `evidence_board`, `timeline`, `events`, `live_evidence`, `matched_objects`. No new cluster calls. Investigations are kept for 30 minutes.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| `klew_event_history`         | Events for a namespace or object, newest first, merging live events with recorded ones, plus recording coverage and gaps.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| `klew_workload_changes`      | Revision history of a Deployment, StatefulSet, DaemonSet or Helm release with field-level diffs.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| `klew_list_resources`        | Objects of one type in a namespace, or cluster-wide for cluster-scoped types, like `kubectl get`. Takes any type the cluster serves, custom resources included: a kind (`Deployment`), plural (`deployments`), short name (`deploy`) or `resource.group` (`certificates.cert-manager.io`). Each row has name, creation time, status, owner and the type's key columns (ready, restarts, node, replicas, ports, …). Filters with `label_selector` and `field_selector`; returns 50 rows by default, up to 200, and says when more exist.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| `klew_get_resource`          | One object: its summary row, labels, annotations, owners and conditions; for Pods each container's state, restarts, image, limits and probes; for ConfigMaps keys and values; for Secrets key names and sizes only. `include_manifest` adds the YAML, capped in length (see below for what is removed or masked).                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
| `klew_pod_logs`              | Recent log lines of one container, oldest first, with timestamps (`tail_lines` default 100, max 1000; optional `since_minutes`; `previous` for the last terminated instance). Does not follow. Each line is redacted and capped, and the result is capped in size, keeping the newest lines. Not offered when `-no-log-samples` is set.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |


Every tool is annotated `readOnlyHint: true` and `destructiveHint: false`. Next checks and suggested actions are text for a person to review; `klew mcp` never runs them.

None of these tools runs `kubectl` or a shell: every call goes straight to the Kubernetes API, and only lists, gets, watches or reads logs.

Two prompts are also offered, which clients may show as slash commands: `troubleshoot_namespace` (namespace) and `troubleshoot_workload` (namespace and workload). Each tells the assistant which of these tools to call and to treat suggested commands as text to review.

Example prompts:

- "Why is payment-api failing in klew-lab?"
- "What changed in payment-api before it started crashing?"
- "Show warning events for redis over the last two days."

## What the assistant sees

`klew mcp` runs locally and has no network endpoint, telemetry, or call-home. **Tool results, however, become part of the conversation, and your MCP client sends the conversation to its AI provider.** Plan for that before pointing it at a cluster.

Results can include:

- pod, container, workload and namespace names, images, resource limits, exit codes;
- Kubernetes event messages;
- **samples of container log lines** (pattern samples, log evidence, the opening words of a log line in the causal chain);
- revision diffs, which show Secret and ConfigMap references, never their values;
- with `klew_list_resources` and `klew_get_resource`, labels, annotations, conditions and ConfigMap values of the objects you ask about;
- with `klew_get_resource` and `include_manifest`, an object's YAML;
- with `klew_pod_logs`, container log lines.

**Secret values are never returned.** `klew_list_resources` and `klew_get_resource` can read Secret objects when your credentials allow it, but return only their type, key names and sizes; manifests show each value as `‹redacted› (N bytes)`. Manifests also drop `managedFields` and the `last-applied-configuration` annotation (which can repeat a Secret as it was applied), and mask env and ConfigMap values whose name or shape looks like a credential. Permissions are your kubeconfig's: anything your credentials cannot list is left out, and recorded events are withheld when you cannot list events.

### Redaction is best effort

Before any free text leaves `klew mcp`, it masks common sensitive shapes with `‹redacted›` or `‹email›`:

- `Authorization: Bearer|Basic|Token …` values and JWTs
- `password=`, `secret:`, `"api_key": …`, `client_secret`, `session_id`, `cookie` and similar key/value pairs
- credentials in URLs (`postgres://user:pass@host`)
- well-known token formats (GitHub, GitLab, Slack, Stripe, AWS access keys, Google API keys) and PEM private keys
- email addresses
- long mixed-case tokens that look like base64 API keys

IP addresses, hostnames, UUIDs and hex digests are left as they are, because incident diagnosis depends on them.

**This is not a guarantee.** Pattern matching cannot recognise every secret or every piece of personal data an application might log — customer names, account numbers, internal identifiers, hex-only keys, or credentials in formats it doesn't know. Treat redaction as a safety net, not a control you can rely on for compliance.

### `-no-log-samples`

With `-no-log-samples`, `klew_pod_logs` is not offered and raw log text is left out of every result: pattern samples, log evidence messages, terminated containers' messages (in manifests too), log-derived signal evidence and timeline messages, and causal-chain steps worded from a log line. Generalised log **templates**, counts, trends and severity are still returned, as is everything that isn't a log (pod status, events, exit codes, revision diffs), so investigations still work.

**Templates are derived from log text, so this flag does not remove all of it.** A template replaces the parts of a line that vary with `<*>`, and the fixed words stay: a value that never varies across the lines it covers — a hard-coded hostname, tenant or account — can still appear, and is still passed through redaction. Templates that merged no differing lines are replaced with `‹log text withheld›`, because such a template would be the log line itself; their counts and severity are kept.

### Choosing a setup

- **Clusters whose logs may hold customer or regulated data:** use `-no-log-samples`, and an AI provider plan whose data terms your organisation has approved.
- **Everything else:** the default (redacted samples) gives the assistant the most to work with.
- In every case, connect `klew mcp` only to clusters whose data you are allowed to share with your AI provider.

