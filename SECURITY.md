# Security

## Reporting a vulnerability

Please report security issues privately through [GitHub security advisories](https://github.com/glnreddy421/klew/security/advisories/new) rather than a public issue. Include the version, how to reproduce, and the impact you expect. You'll get an acknowledgement as soon as the report is read, and a fix or a plan before anything is disclosed.

## Security model

### Read-only against the cluster

Klew — the desktop app and `klew-mcp` — only lists, gets and watches cluster objects, and reads metrics and manifests. It has no code path that creates, updates, patches, applies, scales, execs into, or deletes anything. Suggested next steps are shown as text; Klew never runs them. Every `klew-mcp` tool is annotated `readOnlyHint: true` and `destructiveHint: false`, and a test enforces it.

Klew acts with the RBAC of your kubeconfig context. It cannot see more than your credentials can.

The desktop app's embedded terminal is an ordinary shell you drive; it is not part of how Klew reads the cluster.

### Secrets

Klew never reads Secret objects' values. Revision diffs show Secret and ConfigMap references, not values, and mask plain values whose name or shape looks like a credential.

### `klew-mcp` and AI providers

`klew-mcp` runs locally over stdio. It has no network listener, no telemetry, and makes no calls of its own except to your Kubernetes API server.

Its tool results — object names, event messages, and samples of container log lines — become part of the conversation, which your MCP client sends to its AI provider. Before returning them, `klew-mcp` masks common credential formats and email addresses. **That redaction is best effort and can miss sensitive data**: pattern matching cannot recognise every secret or piece of personal data an application might log. `-no-log-samples` leaves raw log lines out entirely. See [docs/mcp.md](docs/mcp.md#what-the-assistant-sees).

Treat tool results as untrusted input: logs and events can contain text written to manipulate an AI assistant. `klew-mcp`'s server instructions tell clients so, but the client and model are responsible for not acting on it.

### Local data

Recorded event history lives in a local SQLite database (`~/Library/Application Support/Klew/history/`, mode 0600) and is never uploaded.

## Supply chain

- Every pull request runs tests (including the race detector), `govulncheck`, Trivy (dependencies, secrets, misconfiguration), CodeQL, dependency review, `npm audit` and `golangci-lint` with `gosec`.
- Every GitHub Action is pinned to a commit SHA; workflows start with no token permissions.
- Builds use a pinned Go toolchain.
- The kubectl Klew downloads at runtime, and the one bundled into releases, are verified against the SHA-256 Kubernetes publishes; a mismatch is never run.
- Each release builds both front ends of the same engine from one tag: the desktop app (macOS) and `klew-mcp` (macOS and Linux, arm64 and amd64). The app and the macOS `klew-mcp` binaries are signed with Klew's Developer ID and notarized by Apple.
- Each front end gets its own SPDX SBOM, built from what actually ships: the desktop SBOM lists the Go modules in the app binary (including the bundled kubectl) and the npm runtime packages bundled into its UI; the `klew-mcp` SBOM lists only Go modules, read from the binaries' embedded build information.
- Every artifact has a Sigstore-signed build provenance attestation and an SBOM attestation, and is listed in the release's `SHA256SUMS` file:

  ```bash
  gh attestation verify Klew-x.y.z-macos-arm64.dmg --repo glnreddy421/klew
  gh attestation verify klew-mcp-x.y.z-linux-amd64.tar.gz --repo glnreddy421/klew
  shasum -a 256 -c --ignore-missing Klew-x.y.z-SHA256SUMS.txt   # Linux: sha256sum -c --ignore-missing
  ```

## Supported versions

Security fixes go into the latest release. Please upgrade before reporting an issue in an older version.
