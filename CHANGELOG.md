# Changelog

All notable changes to Klew are documented here. Version tags follow [SemVer](https://semver.org/).

## [Unreleased]

### Desktop

- **Home → Contexts** — full-width table of kubeconfig contexts (context, cluster, user, namespace, tags, history, status, actions). Search, tag filters, sortable columns, favorites, and launch default. Orbital hero and braille wordmark are gone; MCP setup is a header action.
- **Check for updates** — Settings → About and Help → Check for Updates compare this build to the latest GitHub release. After startup, if a newer release exists, a dismissible banner prompts with the Homebrew upgrade command or a download / release link. Klew does not download or install the update itself.
- **Preferred context order** — Home lists launch default first, then favorites, then the rest. On launch Klew tries that order and falls through to the next context if one fails to connect; when none work, Home shows an auth/disconnect callout with one-click Reconnect.

### Security

- **Go toolchain** — builds use Go 1.26.11, which fixes a reachable `net/http` vulnerability in Go 1.26.10 ([GO-2026-6859](https://pkg.go.dev/vuln/GO-2026-6859)).

### Performance

- **Faster investigations of large namespaces** — a whole-namespace snapshot reads Deployments, ReplicaSets and EndpointSlices with one List each instead of one request per Deployment or Service, and works out each workload's latest change from objects it already read. Outside container logs, a snapshot now makes the same number of API requests however many Deployments the namespace has; with 500 Deployments that is about 2,000 fewer requests behind the client rate limit. Nodes, and the pods on them, are read in parallel. An investigation of one Deployment still reads only that Deployment.

### Fixed

- **`klew investigate -context`** — the report header, catalog and RBAC cache keys and the event-history lookup now use the context you pass, not the kubeconfig's current-context. The desktop app, which always passes a context, was affected the same way.
- **Settings → Investigation → Event history** shows again. The Settings dialog sat outside the event-history state, so the section rendered nothing.
- **Terminal pre-fill** — commands opened from the CLI and install buttons are typed once the shell has finished starting, so zsh no longer draws the line garbled.
- **Maximized terminal** fills the window below the top bar instead of only the bottom half.
- **`klew mcp -extensions-dir`** with a folder that does not exist now stops with "is not a directory" (exit status 2), as `klew investigate` does, instead of loading no rules and reporting no problems.

## [2.0.0] — 2026-11-08

### Upgrading to 2.0

- **Go module path** — the module is now `github.com/glnreddy421/klew/v2`, as Go requires for version 2 and later. Install the command with `go install github.com/glnreddy421/klew/v2/cmd/klew@latest`; the path without `/v2` only reaches 1.x releases.
- **New `klew` command** — the command line and the MCP server ship as one `klew` binary (`klew investigate`, `klew mcp`), installed with `brew install glnreddy421/klew/klew-cli`. The desktop app is unchanged in how it installs.
- **Desktop layout** — the left sidebar, the top bar's search box and Focus chain are gone; ⌘K opens Investigate, and Terminal and Live logs open from the corner launcher. Details under Desktop below.

### Security

- **Verified kubectl downloads** — the version-matched kubectl Klew downloads from `dl.k8s.io` is now checked against the SHA-256 published next to it before it is used, and so is the kubectl bundled into releases. A missing checksum or a mismatch stops the download. kubectl binaries cached by earlier versions are downloaded and verified again once.
- **Release provenance** — releases include a `SHA256SUMS` file and one SPDX SBOM per front end (the desktop app; the `klew` command), each built from what ships. Every artifact has signed build provenance and SBOM attestations (`gh attestation verify <file> --repo glnreddy421/klew`).
- **`klew` command release binaries** — signed and notarized macOS binaries and Linux binaries, for arm64 and amd64, are published with each release alongside the desktop app.
- **Hardened workflows** — every GitHub Action is pinned to a commit SHA, workflows get no token permissions by default, and the release job asks only for what it uses.
- **Extension files stay in their folder** — adding a rule file in Settings writes it through a handle confined to Klew's extensions folder, so no file name can place it elsewhere.
- **`golang.org/x/net` 0.62.0** — fixes CVE-2026-78661 (denial of service in `x/net/html`).
- **Go toolchain** — builds use Go 1.26.10, which fixes standard-library vulnerabilities in `net/http`, `crypto/tls`, `crypto/x509` and `encoding/asn1` that earlier toolchains left in release binaries.

### Command line

- **`klew investigate`** — a new `klew` command for macOS and Linux prints an investigation in the terminal: the verdict, problems, causal chain, incidents and warning timeline (Overview); failing pods with container states, exit codes and events (Failures); log and event patterns (Patterns); and event ↔ log correlations, claims, observations, gaps and next checks (Evidence). Read-only; `-only` picks sections, `-all` lifts the top-10 limits, `-o json` prints every field, `-extensions-dir` reads one folder of custom collectors and rules, and the exit status is 0 healthy, 3 warning, 4 critical. Reads the desktop app's recorded events when present. Repeats of one reason on the same objects, such as a rejected pod retried every 15 minutes, are shown as one incident with a count. Output is not redacted. See [docs/cli.md](docs/cli.md).

### MCP server

- **`klew mcp`** — a new read-only MCP server that gives AI assistants Klew's analysis over stdio, run by the new `klew` command (or the desktop app as `Klew mcp`). Tools: `klew_list_contexts`, `klew_investigate` (verdict, causal chain, failing pods, patterns, incidents, related change, next checks), `klew_investigation_details` (paged sections of an investigation), `klew_event_history` (live plus recorded events with coverage), and `klew_workload_changes` (revisions with field-level diffs). Runs without the desktop app, on macOS and Linux; reads the desktop app's recorded events when present. See [docs/mcp.md](docs/mcp.md).
- **Homebrew** — `brew install glnreddy421/klew/klew-cli` installs the `klew` command on macOS and Linux; the release workflow updates the formula alongside the app cask.
- **Set up MCP in the desktop app** — a Home card (and **Tools → Set Up MCP…**) shows ready-to-copy configuration for Claude Code, Claude Desktop, Cursor and other clients, with a default context, `-no-log-samples` and `-no-history`. The app's own binary serves MCP as `Klew mcp`, with the same flags and behaviour as `klew mcp` (both run one implementation), so desktop users need no separate install; the dialog also offers a standalone `klew` command it finds on the machine. It only shows text to copy and changes no client configuration.
- **Lookups without an investigation** — `klew_list_resources` lists any resource type the cluster serves, custom resources included, as compact rows with label and field selectors; `klew_get_resource` returns one object's summary, conditions, pod container states or ConfigMap data, and optionally its YAML; `klew_pod_logs` reads a container's recent or previous log lines. All three call the API directly (no `kubectl`), never return Secret values (only type, key names and sizes), drop `managedFields` and `last-applied-configuration`, and mask credential-named env and ConfigMap values. `klew_pod_logs` is not offered with `-no-log-samples`. Two prompts, `troubleshoot_namespace` and `troubleshoot_workload`, give clients a ready starting point. `klew_event_history` gains `reason` (comma-separated) and `object_kind` filters, applied to recorded events before the row cap so frequent events cannot push matches out.
- **Data handling** — `klew mcp` masks common credential formats (authorization headers, JWTs, `password=`-style pairs, URL credentials, known token prefixes, private keys) and email addresses in every result before it reaches the AI provider. This is best effort and can miss sensitive values. `-no-log-samples` leaves raw container log text out while keeping generalised patterns and counts; patterns can still contain fixed words from log lines. Tool inputs are validated against Kubernetes name rules.
- **Every problem in one call** — `klew_investigate` returns the engine's `problems` list: each unhealthy workload or object with its state, the facts behind it, and the rules that found it. Pods also report probes, ports and mounts, and unschedulable pods their scheduler message and requests.

### Desktop

- **Settings → Extensions** — add a rule file, link a folder of rules (read in place, so edits in your own repository apply on **Reload**), see every collector and rule as loaded or skipped with the reason, and test every rule against a namespace. No restart needed. `klew mcp` reads the same folders, recorded in `extension-sources.json`.
- **Help shortcuts updated** — the list now matches the menu (⌘K Investigate, ⌘` terminal, ⇧⌘L live logs, ⇧⌘` split, ⌘N, ⌘R, ⌘,).
- **No sidebar** — the left navigation rail (Home, Resources, Terminal, Live logs) is gone. Home stays on the top bar's house button, choosing a cluster on Home opens Resources, and **Terminal** and **Live logs** are toggle buttons in the top bar (the Live logs button shows the stream's live dot).
- **Focus chain removed** — the Focus buttons, the inspector's **Focus chain** button and the pinned chain view are gone. Investigate covers the same need: it scopes to an object and what it uses. The inspector's Connection trace stays.
- **Inspector status on the meta line** — status such as *No ready replicas* now reads after kind and namespace ("Deployment · holdout · No ready replicas") instead of a badge that crowded the header buttons.
- **Breadcrumb follows the object** — it shows the object's own namespace, and investigating an object selects it, so the breadcrumb no longer names a different, stale row.
- **Investigable types from collectors** — the collectors alone decide what can be investigated. The inspector's **Investigate** button is now a telescope icon. It is enabled only for types a collector reads; for any other type it is disabled and says Klew has no collector for it, instead of starting an investigation that would report "not found". Investigations of such a type now say "Klew has no collector for …". ReplicaSets can now be picked as targets.
- **Investigate any resource** — select a namespaced resource and click **Investigate** in the inspector. In Investigate mode the **Target** picker is a searchable multi-select, grouped by type, filled from the engine's collectors (custom ones included); types you may not list are named with the reason.
- **Investigate from Resources** — Resources has a **Watch | Investigate** switch. Investigate mode picks one namespace, optionally narrows to one **Target** resource, and has a **Logs** toggle to investigate without reading container logs. Results open as tabs on the same screen (Overview, Patterns, Failures, Evidence, Graph) instead of separate sidebar entries; switching back to Watch keeps the investigation running until Stop. The top bar's search box is gone, and ⌘K opens Investigate mode.

### Investigations

- **Extension mistakes are reported, not hidden** — a rule whose `resource` names nothing it can read is skipped with a suggestion (*Did you mean "certificates"?*); a `when` that returns something other than true or false, a fact that fails, and a rule that stops at 200 findings now each add a warning instead of passing silently.
- **Test extensions** — `klew mcp -check-extensions` lists what loaded and every problem; `klew mcp -dry-run RULE -object FILE` or `-namespace NS` runs one rule and prints each object's result; `klew mcp -dry-run-all -namespace NS` runs every rule in the folder against one namespace, read once, with a summary and a failing exit status for CI. All are read-only.
- **Permissions checked first, gaps stated** — before any collector runs, Klew reviews every permission it needs with the API server and skips collectors you may not use. Each investigation records what every collector did (collected, no permission, not installed, failed, off) and says plainly what was not checked; `klew mcp` returns it as `not_collected`. Fixes permission checks that named no API group, which made Roles granting `apps` Deployments, `batch` Jobs or `networking.k8s.io` Ingresses look like missing permissions to non-admin users.
- **Several targets** — investigate several resources at once (`Deployment/api,Service/web`); targets that do not exist are named.

- **Investigate exactly one object** — an investigation of `Kind/name` now covers that object, the pods it reaches through owners, selectors, Ingress backends or ConfigMap, Secret and PVC references, and the objects related to them. Names are no longer matched by substring, so `deployment/api` no longer pulls in `payment-api`. Any kind can be the target, including custom collectors' kinds. A bare name covers every object with exactly that name. Live events and pods follow the same scope, including pods of a new rollout.

- **Recent live events only** — rules that read events (probe failures, CronJob schedule problems, claim provisioning errors) ignore events recorded from history and events older than an hour, so recovered problems are not reported as current.

- **Collectors and rules** — the engine is now built from collectors (what is read from the cluster) and rules (what is wrong), shared by the desktop app and `klew mcp`. Rules produce a `problems` list: every unhealthy workload or object in scope, grouped by owner, with the facts behind its state.
- **Custom collectors and rules** — add YAML files to the `extensions` folder of Klew's data directory: collectors list one resource type (never Secrets), rules are CEL expressions over one object. See [docs/extensions.md](docs/extensions.md).
- **More resource types** — built-in collectors and rules for PersistentVolumeClaims, StorageClasses and PersistentVolumes; PodDisruptionBudgets; NetworkPolicies; ServiceAccounts, Roles and RoleBindings; Gateway API Gateways, GatewayClasses and HTTPRoutes; ValidatingAdmissionPolicies and bindings; ResourceClaims; and OLM ClusterServiceVersions, Subscriptions, InstallPlans, CatalogSources, OperatorGroups, ClusterExtensions and ClusterCatalogs. They list in parallel, skip APIs the cluster does not have, and report missing list permissions once. A claim's findings appear under the workload that mounts it.
- **Workloads, configuration, Ingress and webhooks** — CronJobs (suspended, failing, missed schedules), StatefulSets (stalled rollouts, missing governing Service), DaemonSets (no matching nodes, unavailable pods), ConfigMaps (missing ones used by unready pods; only key names are read, never values), Ingresses (missing or unset IngressClass, missing or unready backends), and admission webhooks served from the namespace whose Service is down.
- **Headline** — investigations carry a one-line summary of every problem ("7 problems: 4 critical, 3 high. Worst: …"). When the leading signal only says that something was logged, it now names the worst problem instead.
- **Robustness** — a rule or collector that fails, even by crashing, is skipped with a warning instead of stopping the investigation. Grouping problems scales linearly: 5,000 failing pods take milliseconds, not the better part of a minute.
- **New checks** — pods rejected at creation (LimitRange, ResourceQuota, webhooks) even when no pod exists; failed Jobs and CronJobs, with their pods grouped under them; failing init containers; Services whose selector matches no pods, naming the closest pod's mismatched labels; containers restarted by their liveness probe, including pods that look Ready between restarts; and False Deployment conditions such as `ProgressDeadlineExceeded`.

- **Pending pods count as unready** — a pod with no container statuses yet (for example one that cannot be scheduled) was counted as ready. It now counts as unready and its containers' requests are shown.
- **Service endpoint totals include unready pods** — a Service whose pods all fail readiness showed 0/0 endpoints, so endpoint-loss signals never fired. It now shows 0/N.
- **Log lines rated by their own level** — a line that states its level (`INFO`, `WARN:`, `[notice]`, `level=warn`, JSON `"level"`) is rated by it, so `INFO flushed errors=0` is no longer an error and `WARN … (non-fatal)` no longer critical. Lines without a level are rated by their words as before.
- **Correlations stay within a pod** — an event pattern is linked to a log pattern only when both name a pod they share, so lines from another workload that happen to log in the same minutes are no longer offered as evidence.
- **Timeouts are not labelled Redis** — any log line with a timeout was labelled "Redis timeout". It is now "Timeout", and only timeouts (not lines that merely mention redis) count as a dependency failure.
- **Suggested fixes name the failing workload** — commands target the workload and container that own the failing pod instead of the first workload found and a container named `app`. `rollout undo` is suggested only when a Deployment has a previous revision. Lab cleanup scripts and the crash-loop patch that removed the container command are no longer suggested.

### Live logs

- **Stream logs from what you select** — select a pod, workload or Service in Resources and click **Stream logs** in the inspector, or ⌘-click (shift-click for a range) several rows and click **Stream logs** or press ⌘⇧L. Workloads and Services follow their label selector, so pods from a rollout or restart join automatically. Works in Watch mode as well as during an investigation; Failures has **Stream logs** for the selected pod and **Stream failing pods**.
- **One merged stream** — lines from every pod are merged in the order the kubelet logged them, with a colour per pod, `pod/container` on each line, and the date for lines from earlier days. Filter by text or regex to show or hide lines, show errors only, hide individual pods, and toggle timestamps. Pause closes the log connections; Resume replays what was logged meanwhile.
- **Investigations still learn from logs** — lines streamed from pods an investigation covers feed its Patterns and signals, unless the investigation was started without logs.
- **Max concurrent log follows** (Settings) now applies to Live logs streams.
- The Live logs button in the top bar and the pod checklist in the panel are gone; ⌘⇧L and the **Stream logs** actions replace them.
- **One panel at a time in the dock** — the Terminal / Live logs / Split strip at the bottom of the window is gone, along with the split view (⌘⇧\`). ⌘\` opens the terminal and ⌘⇧L Live logs; maximize, hide and close sit in each panel's header.
- **Corner launcher** — while the dock is hidden, a pill in the bottom-left corner opens the terminal or Live logs, with a dot when a shell or stream keeps running in the background. It replaces the terminal button in the top bar. With the dock open, the panel header has a button to switch between the two.

### Settings

- **Settings opens as a dialog** over the page you are on instead of replacing it: a section list with icons on the left, the section on the right. It pops in, and closes with Esc, the close button or a click outside. ⌘, and the links that open a section (for example Kubernetes from a connection error) still go straight there.

### Investigate

- **Target picker refresh** — the Target dropdown has a rounded card, a focused search field, type icons in its headings, rounded rows with clearer checkboxes and an **Apply** button that counts your picks. "Whole namespace" is now **All resources**.
- **Investigate query bar** — Namespace, Target and Logs now sit together in one field group, each with an icon, a small label and its value, and clicking anywhere in a field opens it. Logs is a switch (Included / Off) instead of a checkbox, and Watch / Investigate, the field group and the Investigate button share one height.
- **No terminal or logs dock on Home** — the dock, its top-bar button and ⌘\` / ⌘⇧L only work once you are in a cluster (Resources and the investigation tabs). An open dock reappears when you leave Home.

### Motion

- **Calm, consistent transitions** — page changes, switching resource type, opening the inspector, the dock and menus now fade in over about 180 ms with a small rise, using one shared set of timings. Switching context keeps the current page in place, dimmed under a thin progress line, until the new cluster's page fades in, instead of jumping to empty placeholders. Hover feedback eases instead of snapping. All of it turns off with macOS **Reduce motion**.

### Tooling

- Bump **Wails** to v2.17.1 (CLI, module, release CI).
- Bump the **Kubernetes client libraries** to 0.37, **modernc.org/sqlite** to 1.61, **Vite** to 8 and **React** to 19.3.
- **Pull request lint** compares new code with the base branch in git, so large pull requests are judged on what they add instead of every existing finding.

## [1.5.0] — 2026-10-24

### Workload Changes

- **Changes tab** — Deployments, StatefulSets, and DaemonSets get a revision timeline from the history Kubernetes already keeps (ReplicaSets, ControllerRevisions), newest first: relative time, revision, source (rollout, rollback, restart), who changed it when that can be matched, and a one-line summary such as "image v2.3.1 → v2.4.0, env DB_POOL_SIZE 20 → 5". Expand a row for the field-level diff. Works in browse mode, no investigation needed, and reads only.
- **Field-level diffs** — image, env, envFrom, resources, probes, command/args, ports, volumes and mounts, service account, scheduling, security context, init containers, and template labels/annotations. Secret and ConfigMap references are shown as references, never values; plain values that look like credentials are masked. `kubectl rollout restart` shows as "Restarted".
- **Rollbacks** — labelled "Rollback to rev N template", dated when the rollback happened (not when the reused ReplicaSet was first created).
- **Helm** — for Helm-managed workloads and in the Helm release inspector, user-supplied value changes and chart version changes per Helm revision, attached to the rollout they caused. Loaded only when you open the Changes view.
- **Summary** — "Changed 12m ago (rev 14)" for workloads changed in the last day; Pods show which revision they run and are highlighted when behind ("Running rev 13 · current is rev 14 (image changed 12m ago)"); ReplicaSets show which revision they are and what it changed.
- **Investigations** — Overview pins a recent change of an in-scope workload, e.g. "payment-api changed 12m before the incident started: rev 14 (image, env)", linking to its Changes tab. The change also appears in the timeline and in "What Klew connected".
- **Permissions** — without access to ReplicaSets/ControllerRevisions the tab says so ("No permission to read revision history (replicasets list)"); without access to Secrets only the Helm part is left out.

## [1.4.1] — 2026-10-21

### Event history

- **Fewer false gaps** — after a pause, Klew relists or resumes its watch and recovers the events the cluster still holds, so a pause shorter than the cluster's event TTL leaves no gap; a longer one leaves only the part beyond the TTL. New setting: **Settings → Investigation → Event history → Cluster event TTL** (default 1 hour).
- **Sleep-aware** — a late heartbeat (Mac asleep) now forces a resync instead of leaving a stale connection, so the recovered events are credited.
- **No session churn on launch** — recorders restart only when the proxy or kubeconfig actually changes; the launch-time settings sync no longer splits history.
- **Gaps explain themselves** — one compact line ("Includes recorded events · history since 09:09 · 2 gaps") with the gap list in a popover, each marked *Not recorded* with the reason when known, e.g. "Not recorded · Mac asleep" (sleep is confirmed with macOS's wake time), Klew closed, monitoring paused, cluster unreachable, recording off, RBAC denied, or context not recorded. Same treatment in the inspector and the top-bar recording dot.

### Overview

- **Current state vs. recent incidents** — when nothing fails now but Warnings occurred, the headline reads e.g. "Healthy now · 2 incidents in the last 6h (recovered)". Warnings are grouped into incidents (within 10 minutes of each other), each with its time range, leading reason, and status. An incident is *recovered* only when its objects are healthy now and no Warning arrived for 10 minutes; otherwise it is *ongoing*.
- **What Klew connected** shows the time window it covers and is marked *recovered* when all of its steps predate recovery.
- **Key findings by importance** — ranked by severity, then recency, then count. Routine lifecycle events (Pulled, Scheduled, Started, SuccessfulCreate, …) no longer lead the list; they stay in Patterns and Evidence. "No issues found in this window" when nothing remains.

### Fixes

- Internal identifiers such as `metrics_source` no longer appear in the chain or observations.
- The inspector shows a Pod's namespace instead of "Cluster-scoped" wherever it is opened (Overview, Graph, signals, Resources, cross-links); scope comes from API discovery.
- Overview scope bar reads `docker-desktop / ns: klew-lab`.

## [1.4.0] — 2026-10-20

### Event history

- **Local event recorder** — Kubernetes Events expire after about an hour; Klew now records them while it runs, for the active context and every favorite context, so investigations can show what happened before you started looking. Read-only (List/Watch only), stored locally in `~/Library/Application Support/Klew/history/events.db`, never uploaded. **On by default**; off switch in Settings.
- **Honest coverage** — Klew tracks when it was recording and shows gaps when it was not. Top bar shows a small recording dot beside the cluster cloud (hover for "history since …" and gap count, or why it is not recording: paused, RBAC denied, turned off); Home and the context picker show per-context history depth.
- **Investigations** — recorded events from the last 6 hours merge into Overview, Failures, Evidence, and Patterns (deduplicated by event UID) with a "recorded" marker and an "Includes recorded events…" line listing gaps. The live 15-minute signal window is unchanged.
- **Inspector → Events** — includes recorded events for the object, newest first, with "History since …" and "No history … (Klew not recording)" for gaps during the object's lifetime.
- **Settings → Investigation → Event history** — on/off, retention (default 7 days), size limit (default 500 MB), current size, history start per context, and Clear history.
- **Multiple windows** — one window records each cluster at a time; the others read the shared history and take over when that window closes.
- **Limited RBAC** — when cluster-wide events are denied, records per namespace (up to 50) instead; under very high event volume, keeps Warning events only and says so.

### Settings

- **Refreshed layout** — grouped cards with label-left / control-right rows, switches instead of checkboxes, and a lighter sidebar; long kubectl details fold behind a disclosure.
- **Fix** — "Download kubectl matching cluster version" is now disabled when bundled kubectl is off.

### Look and feel

- **Floating panes** — explorer, workspace, inspector, and console are rounded cards on one canvas with gutters instead of hairlines; works across every dark, black, and light theme.
- **Softer surfaces** — rounded hover/selection rows in tables, filled inspector sections and overview tiles instead of boxes-in-boxes, pill console tabs.
- **Home** — braille-style dot wordmark and a new tagline.

### Layout

- **Adaptive rail** — the activity rail shows icons only while an explorer is beside it and labels elsewhere; the toggle returns to automatic when it matches.
- **Explorer yields to the inspector** — when an open inspector would squeeze the workspace below a usable width, the explorer folds to its icon strip and returns when the inspector closes.
- **Explorer fits its labels** — width fits most resource-kind names; dragging pins your width (double-click the edge to refit).
- **Inspector** — opens automatically when you select a resource, as a side panel rather than a takeover; remembers the size you drag it to (double-click the edge to reset); click the workspace to dismiss. Collapsed edge shows a hover-revealed handle.

### Resources browse

- **Collapsible categories** — each category stays open until you collapse it; selecting a kind opens its category without closing the rest. Collapsed groups no longer render their rows.
- **Resizable columns** — drag a header edge to resize, double-click to fit content, arrow keys to nudge; widths persist per kind with an Auto widths reset.
- **Loading** — progress bar and skeleton rows; slow first loads show cluster API discovery status with Reconnect and Proxy actions.

### Terminal

- **Kubeconfig isolation** — each session uses a private session copy pinned to the selected context and namespace; your kubeconfig file is no longer modified. Stale session copies from crashed processes are cleaned up.
- **Plain shell fallback** — a missing or broken kubeconfig opens an unpinned shell with an explanation instead of failing.
- **Find** — matches are found and highlighted again.
- **Shell picker** — first-run dialog renders as a proper card.

## [1.3.6] — 2026-09-27

### Admission

- **Resources → Admission** — mutating and validating webhook configurations (cluster-scoped) with namespace scope on list rows; moved out of Config.
- **Investigation perimeter** — when a workload expects pods but none exist, optionally list matching admission webhooks and correlate recent events (no metrics-server dependency). Surfaces in Evidence, Failures, Patterns, and Graph; explicit label when RBAC blocks the check. **On by default** in Settings → Investigation.

### Context and Home

- **Favorite contexts** — star multiple kube contexts for quick access; separate **launch default** (home icon) without forcing context switches when leaving Home.

### Terminal

- **Per-context sessions** — terminal tabs restore per kube context (in-memory + session storage).

### Resources browse

- **Namespace picker** — checkbox multi-select with Apply/Cancel; fixes treating manual “all namespaces” selection as the global All checkbox and order-dependent scope compares.

### Focus chain

- **Pin-to-pin stability** — canonical inspect keys, catalog merge without snapping selection back to the pin root on refresh, and consistent row highlighting across key formats.

## [1.3.5] — 2026-09-17

### Home

- **Scroll and layout** — long kubeconfig context lists scroll inside the cluster panel; orbital hero scales down on narrow panes so it is not clipped.
- **Settings** — Kubernetes settings footer control is icon-only.

### Resources browse

- **Workloads first** — overview loads Pod, Deployment, StatefulSet, and DaemonSet before other kinds; workload nav counts fetch eagerly, other kinds stay lazy.
- **Show cached data immediately** — catalog, entity lists, and overview use stale-while-revalidate: last snapshot renders at once, refresh runs in the background with a light “Refreshing…” hint.
- **Overview while discovering** — Workloads overview can render before API discovery finishes; presentation fallback keeps the workloads nav usable.
- **Load on open** — catalog and cluster probes run for the active context only when Resources is opened, not for every context on Home.

### Namespace and performance

- **Namespace switch** — scope changes use a lightweight namespace select instead of a full reconnect spinner.
- **Workload-only counts** — catalog index can request counts for workload resource IDs only, reducing first paint time on large clusters.

## [1.3.4] — 2026-09-17

### Home

- **Landing page** — Klew opens on Home with the orbital hero and local kubeconfig contexts; Resources is chosen explicitly via **Open** or the sidebar.
- **Default context** — pin/unpin with ☆/★ on Home or in the context popover; marks the usual cluster without skipping Home on launch.

### Cluster connection

- **Contexts always listed** — kubeconfig contexts stay visible when unreachable; cloud icon shows online/offline/paused.
- **Loading and reconnect** — spinners while syncing; inline reconnect and proxy settings on fetch panels and loading surfaces.
- **Manual disconnect** — pause monitoring from the context popover or Settings; optional auto-reconnect toggle.
- **Proxy settings** — HTTP/HTTPS/NO_PROXY for API traffic in Settings → Kubernetes; inherited from login shell on launch.
- **Sync resilience** — preserve context picker on refresh errors; load kubeconfig without requiring `current-context`.

### Resources browse

- **Access states** — clearer denied, could-not-load, and not-in-discovery messaging instead of false “API not exposed” on slow clusters.
- **Catalog refresh** — respect monitoring pause; invalidate cache on reconnect.

## [1.3.3] — 2026-09-16

### Focus chain

- **Pin-to-pin navigation** — investigation `Kind/name` and catalog `Kind/namespace/name` keys match consistently; chain selection, root highlight, and inspector links stay in sync.
- **Chain build** — canonical keys, catalog merged into snapshot for browse-mode chains, ingress→service matching fixed.

### Namespace scope

- **Resources picker** — draft multi-select with Apply/Cancel; no flip to “All namespaces” when every box is checked manually.
- **Investigate scope** — decoupled from kubeconfig namespace until an investigation starts; browse scope no longer resets on namespace sync.

### Tooling

- Bump **Wails** to v2.14.0 (CLI, module, release CI).
- Frontend **vitest** suite with pin-to-pin focus chain tests (`npm test`).

## [1.3.2] — 2026-09-16

### Investigation health

- **Finished job pods** (Succeeded/Completed) no longer count as pending or degraded — CronJob completion no longer drags scope health down.
- **Catalog Running pods** derive readiness from phase and container status so the Pods donut matches the cluster.
- **Status strip** ignores config noise and terminal pods; deployment health reconciles against live pod state.
- **Manual snapshot refresh** (`RefreshInvestigation`) and structural auto-refresh on pod failures keep investigation counts current.

### Workloads overview

- **Near real-time overview** — 8s poll, live pod watch while Overview is open, cache bust for stale health data.
- **Completed/failed pods** filtered from browse lists and discovery matches.

### Fixes

- Backend summary, verdict, and correlator agree with frontend on operational vs finished pods.
- Release CI uses Node 24.

## [1.3.1] — 2026-09-14

### Resources browse — instant feel, live updates

- **Stale-while-revalidate cache** across the Resources surface: catalog index, entity lists, workload overview, and cluster status show cached data instantly on revisit; background refresh keeps data current without blank loading states.
- **Tiered TTL by scope** — tighter cache for single-namespace browse; longer intervals for all-namespaces and overview fan-out on large clusters.
- **Live entity watch** — single-namespace (and cluster-scoped) kinds stream changes via Kubernetes watch with debounced relist; wide scopes poll on an interval instead of opening cluster-wide watches.
- **Visibility-aware refresh** — watches and polls pause when the window/tab is hidden to reduce API load.
- **Data age & manual refresh** — “Live” / “Updated X ago” labels plus refresh icons on Overview and entity lists.

### Cluster connection

- **Sync no longer wipes context** when kubeconfig read fails temporarily (e.g. Docker Desktop starting); picker state is preserved.
- **Startup sync sequencing** — `GetCluster` before `SyncCluster` to avoid racing away a good context.
- **Resources explorer on welcome** — ScopeBrowseProvider wraps the Resources tab even before scope is fully selected.

### Fixes

- Fixed catalog hook dependency loops that caused perpetual “Updating…” spinners on Overview.
- Cluster status errors are no longer cached as reachable for five minutes.

## [1.3.0] — 2026-09-14

### Resources browse

- **Workload connection trace** — horizontal Ingress → Service → Deployment → ReplicaSet → Pod path in the inspector when browsing Workloads (no investigation required); click any step to jump.
- **Resource nav headers** — clearer category accordion styling for Network, Storage, Config, and related groups.

### Top bar & branding

- **Top bar size** — zoom logo, scope, search, and Investigate from 85–130% via the header control or Settings → Appearance; preference persists across sessions.
- **KLEW wordmark** — vertical KL / EW stack beside the logo.

### Inspector & tables

- Inspector chrome shows resource name, kind, and status in the header.
- Entity table category tone stripe on column headers; slimmer inspect identity toolbar.

## [1.2.0] — 2026-09-12

### Resources browse

- **Focus chain in browse mode** — follow Deployment → ReplicaSet → Pod → Service without starting an investigation; chain uses catalog data and stable namespace-aware keys.
- **Inspector browse tabs** — Summary, Spec, Relationships, and other tabs show what to expect, loading/empty states, and what to look for per resource kind.
- **Browse metrics** — CPU/memory columns and workload overview metrics when metrics-server is available.
- **Scheduling tables** — tolerations, affinity, and node selector render as proper tables in the entity grid (hover) and inspector.
- **Manifest cache** — LRU stale-while-revalidate for inspector YAML; tiered rendering for large manifests.

### Investigation surfaces (browse without Investigate)

- **Overview, Patterns, Failures, Evidence** — guided empty states explain what each surface provides and how to start an investigation from the top bar.
- Sidebar explorers on investigation tabs show the same guidance when no session is running.

### Inspector & shell

- Inspector panel uses card chrome (accent stripe, header LED, inset padding) and VS Code-style resize sash.
- Terminal / live logs no longer auto-open on focus or when investigation starts — user-driven only.
- Cluster connection banner and reconnect flow improvements.

### Backend

- Workload and batch **relationship** enrichment for inspector graphs.
- Structured **scheduling** fields on catalog rows.
- Node browse metrics enrichment.

## [1.1.0] — 2026-09-09

### Inspector & workloads

- Surface **related Pods** for Deployments, StatefulSets, Jobs, CronJobs, and DaemonSets — inspect and tail logs from the workload inspector.
- **Manifest** tab: LRU cache with stale-while-revalidate fetch; keep view mounted when switching tabs; tiered YAML rendering for large manifests.
- **Scheduling** fields render as proper tables (tolerations, node selector, affinity, taints) with Hide/Show — in the entity table (hover popover) and inspector.
- Fold **Status** and **Scheduling** into the **Summary** tab (no separate tabs).

### Catalog

- Structured scheduling data on catalog rows for table columns and inspector enrichment.

## [1.0.0] — 2026-09-09

Major release: redesigned **Resources** workbench, shell inspector, terminal workspace, and cluster catalog — the “klew-ui” line.

### Resources workbench

- New **Resources** explorer with category navigation, lazy counts, and browse scope (cluster / namespace).
- **Entity tables** with kind-specific columns, column picker, and richer cell renderers (nodes, containers, deployment conditions, service endpoints).
- **Inspector panel** — slide-out detail view with manifest tab, relationship graph, and workload overview; starts collapsed on launch.
- Clickable inspect links (Node, ServiceAccount, etc.) with visible hover/focus states across themes.
- **Container inspect** — full image refs, container/image IDs, ports, command/args (no truncation).
- **Relationship graph** for workloads, services, ingress, and related objects.
- **Resource manifest** view via kubectl-equivalent fetch.

### Helm

- **Helm → Releases** — list releases from cluster Secrets with chart, revision, status, and updated time.
- Read-only release inspector (values summary, backing Secret ref); manifest resolves to the release Secret.
- Remove virtual **Helm Charts** tab (invalid manifest target).

### Terminal

- Per-tab **close** (×) on every tab, including single log-tail sessions.
- Right-click tab → **split** right or down when two or more tabs are open.
- **⌘F find** in terminal output (log tails and shells) with match navigation.

### Cluster connectivity

- Connection status dot and banner when the API is unreachable or credentials expire.
- Clearer connection error messages from the backend.

### Catalog & details (backend)

- Browse API with table fields, node enrichment, and builtin kind providers.
- Endpoints, webhook configurations, Helm release details, and generic unstructured body support.
- Virtual resource dispatch and kubectl manifest resolution for inspect targets.

## [0.1.10] — 2026-09-07

### Investigation UX

- Default to **Resources** when an investigation starts so you can browse the catalog immediately.
- Show a non-blocking “correlating in the background” banner while matches load; Overview still shows full progress if you switch there.

## [0.1.9] — 2026-09-07

### Investigation scope (large clusters)

- Searchable **context** and **namespace** pickers with type-to-filter and type-to-select.
- Namespace picker: Enter or “Use namespace …” when the list is incomplete or RBAC-limited.
- Scope picker: search resources, filter by kind, bulk actions (Workloads / Select visible / Clear).
- Smart default selection — top workload matches instead of checking every resource in huge namespaces.
- Remove “Search all namespaces” — investigations are single-namespace only (cross-namespace planned later).

## [0.1.8] — 2026-09-07

### Cluster connectivity (enterprise / read-only clusters)

- Bootstrap login-shell environment at startup so GUI launches get the same `PATH`, `KUBECONFIG`, and cloud credential vars as Terminal, Lens, and K9s.
- Align kubeconfig loading with kubectl (multi-file `KUBECONFIG`, default loading rules).
- Soft-fail when namespace listing is RBAC-restricted; show a warning instead of a blocking sync error.
- Allow typing a namespace manually when the list is incomplete.
- Fall back to `kubectl get ns` when the in-process API client fails.

### Bundled kubectl

- Ship kubectl v1.31.4 inside `Klew.app/Contents/Resources/` (signed with the app).
- Settings → Kubernetes: choose bundled, system PATH, or a custom kubectl path.
- Conditionally download a cluster-matched kubectl when skew exceeds ± one minor version (cached under `~/Library/Application Support/Klew/binaries/`).
- Terminal prepends the active kubectl directory to `PATH`.

### Release

- Switch Homebrew distribution from a formula to a cask so Klew installs to `/Applications` and upgrades with `brew upgrade --cask klew`.

## [0.1.7] — 2026-09-01

### Investigation engine

- Merge snapshot container logs (including previous-container logs) into pattern mining on every refresh, so Correlated Signals and Log Patterns update without requiring live log tail.
- Spread snapshot log timestamps across recent minutes so burst failures (OOM, crash loops) align with infrastructure event patterns.

### Desktop app

- Streamline Evidence: drop duplicate verdict header, causal notes, and time-window controls; focus on correlated signals, observations, and claims.
- Add bottom console dock for terminal and live logs; fix layout so the log panel stays visible during investigations.
- Refine Overview, Patterns, Resources, and shell navigation; move Settings and Help to the top bar.
- Fix component inspector hook ordering crash and bottom inspector layout.

### Docs

- Add full screenshot set (Overview, Failures, Patterns, Evidence, Resources, Graph) to the README.

### Tooling

- Add `cmd/klew-stress` and engine stress benchmarks for ingestion and correlation load testing.

## [0.1.6] — 2026-09-01

### Desktop app

- Ship the Wails desktop shell as the primary Klew experience, replacing the prior TUI/CLI workflow.
- Add an investigation workbench with scope-aware resource catalog navigation, entity tables, and deep inspection panels.
- Introduce RBAC-aware browsing: catalog entries and secret-backed environment values respect cluster permissions.
- Add lazy resource counts in the sidebar so empty or zero-count kinds no longer show misleading totals.
- Fix resource-kind switching crashes when browsing catalog kinds with no entities (Events, Nodes, and similar).
- Streamline the resources explorer layout and investigation context banner for clearer cross-surface navigation.
- Add terminal workspace tabs, shell selection, and appearance controls integrated into the desktop shell.

### Release

- Fix the Homebrew formula template so installed apps keep their code signature.
- Add a notarization credential preflight in release CI with clearer failure messages.
- Bump `wails.json` product version to 0.1.6.

## [0.1.5] — 2026-08-15

### Desktop app

- Add Klew logo mark to the sidebar and update the Dock icon to match the marketing site.
- Redesign the top bar: refined scope selectors, command-style search, cleaner typography (Plus Jakarta Sans), and a compact connection status rail.
- Split **Settings** and **Help** in the sidebar; Help includes keyboard shortcuts and documentation links.
- Add native macOS menu bar menus: **Edit**, **Tools** (sync, new window, settings, kubeconfig folder), and **Window**.
- Enable the green maximize traffic-light button by configuring Wails macOS options.
- Remove redundant “empty search” helper copy from the search field and welcome screens.

### Release

- Harden macOS CI signing (avoid brittle `--deep --strict` codesign) and pin Node 22 for release builds.

### Other

- Remove demo mode and simulated investigation paths.
- Move release scripts under `.github/scripts/`; package script now stages the app icon from `packaging/macos/appicon.png`.

## [0.1.4] — 2026-07-31

- Sign and notarize macOS releases with Developer ID in CI.
- Automate Homebrew tap bumps on release.
- Fix Homebrew zip install using `cp_r` and buildpath detection.

## [0.1.3] — earlier

- Prior desktop and CLI improvements; see git history and GitHub releases.

[1.1.0]: https://github.com/glnreddy421/klew/releases/tag/v1.1.0
[1.0.0]: https://github.com/glnreddy421/klew/releases/tag/v1.0.0
[0.1.10]: https://github.com/glnreddy421/klew/releases/tag/v0.1.10
[0.1.9]: https://github.com/glnreddy421/klew/releases/tag/v0.1.9
[0.1.8]: https://github.com/glnreddy421/klew/releases/tag/v0.1.8
[0.1.7]: https://github.com/glnreddy421/klew/releases/tag/v0.1.7
[0.1.6]: https://github.com/glnreddy421/klew/releases/tag/v0.1.6
[0.1.5]: https://github.com/glnreddy421/klew/releases/tag/v0.1.5
[0.1.4]: https://github.com/glnreddy421/klew/releases/tag/v0.1.4
[0.1.3]: https://github.com/glnreddy421/klew/releases/tag/v0.1.3