# Klew MCP server

`klew-mcp` gives AI assistants — Claude Code, Claude Desktop, Cursor, and any other [Model Context Protocol](https://modelcontextprotocol.io) client — Klew's incident analysis instead of raw `kubectl` output. It runs the same engine as the desktop app and is **read-only**: it lists, gets and watches cluster objects and never changes them.

It does not need the desktop app. When the app is installed, `klew-mcp` also reads the events it recorded, so an assistant can see events Kubernetes has already expired.

## Install

Build from source (Go 1.26+):

```bash
go install github.com/glnreddy421/klew/cmd/klew-mcp@latest
```

The binary lands in `$(go env GOPATH)/bin`. It needs a working kubeconfig, the same as `kubectl`.

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

Tool results go into the assistant's context, and from there to its model provider. They can include pod and container names, images, event messages, resource limits and **samples of container log lines** from the investigated namespace. Secret values are never read; revision diffs show Secret and ConfigMap references, not values, and mask plain values that look like credentials. Point `klew-mcp` only at clusters whose data you are allowed to share with your assistant.

Permissions are your kubeconfig's: anything your credentials cannot list is left out, and recorded events are withheld when you cannot list events.
