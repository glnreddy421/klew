# Klew

Klew is a read-only Kubernetes investigator for Claude. Instead of pasting raw `kubectl` output into the conversation, Claude gets answers Klew has already worked out on your machine: what failed, why, what changed, and what to check next.

This plugin starts the `klew` command (`klew mcp`), looking on your `PATH` and in the usual Homebrew and local install folders. Install that binary first; a session-start check reminds you if it is missing.

## Install the plugin

In Claude Code:

```
/plugin install klew --marketplace glnreddy421/klew
```

On older Claude Code versions, add the marketplace first: `/plugin marketplace add glnreddy421/klew`, then `/plugin install klew@klew`.

## Install the klew command

```bash
brew install glnreddy421/klew/klew-cli
```

Or download a release binary from https://github.com/glnreddy421/klew/releases (macOS and Linux, arm64 and amd64).

## What you get

- Investigate a namespace or workload and get a verdict with evidence
- Dig into pods, signals, log and event patterns, timeline, and live evidence
- See field-level diffs across Deployment, StatefulSet, DaemonSet, and Helm revisions
- Read event history (including events the Klew desktop app recorded after Kubernetes expired them)
- List and get any cluster object type, custom resources included

Every tool is read-only: Klew only lists, gets, and watches. Access follows the RBAC of your kubeconfig. Secret values are never returned; credential-shaped text is masked best-effort.

## Configure

Optional plugin settings (leave empty for defaults):

- **Kubeconfig** — path to a kubeconfig file (`KUBECONFIG` or `~/.kube/config` otherwise)
- **Default kube context** — used when Claude does not name a context
- **Leave out log samples** — keep raw container log text out of results
- **Extensions folder** — your Collector and Rule YAML files

## Surfaces

A local stdio MCP server runs in Claude Code and in on-machine Cowork sessions. It does not reach claude.ai chat; that needs a remote MCP connector.

## Docs

- Product: https://klew.dev
- MCP guide: https://github.com/glnreddy421/klew/blob/main/docs/mcp.md
- Privacy: https://klew.dev/privacy
