# Klew MCP server

`klew-mcp` gives AI assistants — Claude Code, Claude Desktop, Cursor, and any other [Model Context Protocol](https://modelcontextprotocol.io) client — Klew's incident analysis instead of raw `kubectl` output. It runs the same engine as the desktop app and is **read-only**: it lists, gets and watches cluster objects and never changes them.

It does not need the desktop app. When the app is installed, `klew-mcp` also reads the events it recorded, so an assistant can see events Kubernetes has already expired.

## Install

`klew-mcp` needs a working kubeconfig, the same as `kubectl`.

**Download a release** — every Klew release has `klew-mcp` archives for macOS and Linux on arm64 and amd64. The macOS binaries are signed with Klew's Developer ID and notarized by Apple.

```bash
VERSION=2.0.0  # the release you want
OS=darwin      # or linux
ARCH=arm64     # or amd64
curl -fsSLO "https://github.com/glnreddy421/klew/releases/download/v${VERSION}/klew-mcp-${VERSION}-${OS}-${ARCH}.tar.gz"

# Optional: check where it came from before installing.
gh attestation verify "klew-mcp-${VERSION}-${OS}-${ARCH}.tar.gz" --repo glnreddy421/klew
# Or, without gh, against the release checksums (Linux: sha256sum -c --ignore-missing):
curl -fsSLO "https://github.com/glnreddy421/klew/releases/download/v${VERSION}/Klew-${VERSION}-SHA256SUMS.txt"
shasum -a 256 -c --ignore-missing "Klew-${VERSION}-SHA256SUMS.txt"

tar -xzf "klew-mcp-${VERSION}-${OS}-${ARCH}.tar.gz" klew-mcp
sudo install -m 0755 klew-mcp /usr/local/bin/klew-mcp
```

**Or build from source** (Go 1.26+):

```bash
go install github.com/glnreddy421/klew/cmd/klew-mcp@latest
```

The binary lands in `$(go env GOPATH)/bin`.

## Add it to a client

**Claude Code**

```bash
claude mcp add klew -- klew-mcp
```

**Claude Desktop, Cursor and other JSON-configured clients**

```json
{
  "mcpServers": {
    "klew": {
      "command": "/Users/you/go/bin/klew-mcp",
      "args": []
    }
  }
}
```

Use the absolute path: desktop clients start servers without your shell's `PATH`. `klew-mcp` loads `PATH`, `KUBECONFIG` and cloud CLI variables from your login shell itself, so exec auth plugins (`aws`, `gke-gcloud-auth-plugin`, `kubelogin`) keep working.

### Flags

| Flag | Default | Purpose |
| --- | --- | --- |
| `-kubeconfig` | `KUBECONFIG` or `~/.kube/config` | Kubeconfig to read |
| `-context` | kubeconfig current-context | Default context when a tool call names none |
| `-history-dir` | `~/Library/Application Support/Klew/history` | Where the desktop app keeps recorded events |
| `-no-history` | off | Ignore recorded events |
| `-no-log-samples` | off | Never return raw container log lines (see below) |
| `-version` | | Print the version and exit |

## Tools

| Tool | What it returns |
| --- | --- |
| `klew_list_contexts` | Contexts in your kubeconfig. Reads the file only. |
| `klew_investigate` | A verdict for a namespace or workload: status, leading signal, likely trigger, confidence, causal chain, failing pods (exit codes, OOMKilled, limits), top log and event patterns, warning incidents, a related workload change, and next checks. Collects a snapshot, then watches live for `observe_seconds` (default 10, max 60). |
| `klew_investigation_details` | One section of an earlier investigation, paged: `pods`, `signals`, `log_patterns`, `event_patterns`, `evidence_board`, `timeline`, `events`, `live_evidence`, `matched_objects`. No new cluster calls. Investigations are kept for 30 minutes. |
| `klew_event_history` | Events for a namespace or object, newest first, merging live events with recorded ones, plus recording coverage and gaps. |
| `klew_workload_changes` | Revision history of a Deployment, StatefulSet, DaemonSet or Helm release with field-level diffs. |

Every tool is annotated `readOnlyHint: true` and `destructiveHint: false`. Next checks and suggested actions are text for a person to review; `klew-mcp` never runs them.

Example prompts:

- "Why is payment-api failing in klew-lab?"
- "What changed in payment-api before it started crashing?"
- "Show warning events for redis over the last two days."

## What the assistant sees

`klew-mcp` runs locally and has no network endpoint, telemetry, or call-home. **Tool results, however, become part of the conversation, and your MCP client sends the conversation to its AI provider.** Plan for that before pointing it at a cluster.

Results can include:

- pod, container, workload and namespace names, images, resource limits, exit codes;
- Kubernetes event messages;
- **samples of container log lines** (pattern samples, log evidence, the opening words of a log line in the causal chain);
- revision diffs, which show Secret and ConfigMap references, never their values.

Secret objects are never read. Permissions are your kubeconfig's: anything your credentials cannot list is left out, and recorded events are withheld when you cannot list events.

### Redaction is best effort

Before any free text leaves `klew-mcp`, it masks common sensitive shapes with `‹redacted›` or `‹email›`:

- `Authorization: Bearer|Basic|Token …` values and JWTs
- `password=`, `secret:`, `"api_key": …`, `client_secret`, `session_id`, `cookie` and similar key/value pairs
- credentials in URLs (`postgres://user:pass@host`)
- well-known token formats (GitHub, GitLab, Slack, Stripe, AWS access keys, Google API keys) and PEM private keys
- email addresses
- long mixed-case tokens that look like base64 API keys

IP addresses, hostnames, UUIDs and hex digests are left as they are, because incident diagnosis depends on them.

**This is not a guarantee.** Pattern matching cannot recognise every secret or every piece of personal data an application might log — customer names, account numbers, internal identifiers, hex-only keys, or credentials in formats it doesn't know. Treat redaction as a safety net, not a control you can rely on for compliance.

### `-no-log-samples`

With `-no-log-samples`, raw log text is left out of every result: pattern samples, log evidence messages, log-derived signal evidence and timeline messages, and causal-chain steps worded from a log line. Log **templates**, counts, trends and severity are still returned, as is everything that isn't a log (pod status, events, exit codes, revision diffs), so investigations still work.

Templates replace the parts of a line that vary with `<*>`. A value that never varies — a single hard-coded hostname or account — can still appear in a template, and is still passed through redaction.

### Choosing a setup

- **Clusters whose logs may hold customer or regulated data:** use `-no-log-samples`, and an AI provider plan whose data terms your organisation has approved.
- **Everything else:** the default (redacted samples) gives the assistant the most to work with.
- In every case, connect `klew-mcp` only to clusters whose data you are allowed to share with your AI provider.
