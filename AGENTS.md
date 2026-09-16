# Agent instructions

## Git and commits

All work in this repository must appear solely under the repository owner's identity.

- **Never** add `Co-authored-by`, `Signed-off-by`, or any other trailer that names Cursor, Composer, Copilot, or other AI/agent tools.
- Cursor may inject `Co-authored-by: Cursor` via `--trailer` when the agent runs `git commit`; repo `.githooks/commit-msg` strips it. Enable with `git config core.hooksPath .githooks` (local, per clone).
- **Never** mention Cursor, agents, or AI assistants in commit messages, PR descriptions, or release notes unless the user explicitly asks for that attribution.
- Use the owner's name and email only (as configured in git). Do not change `user.name` or `user.email`.
- **Do not create commits or push** unless the user explicitly asks.
- When the user asks to commit: write clear, professional messages with no tool attribution.

## History

Do not rewrite git history unless the user explicitly requests it.

## Resources browse — read-only

The Resources surface (catalog, entity lists, overview, inspector, manifest YAML) must stay **read-only** against the cluster:

- Allowed: `List`, `Get`, `Watch`, discovery, metrics read, manifest fetch.
- **Never** add Create/Update/Patch/Delete/Apply/Scale (or equivalent) APIs or UI for browse mode.
- Live watch only **observes** changes made elsewhere; it does not mutate resources.
- Investigation mode may suggest kubectl commands as text; it must not execute mutating cluster calls unless the user explicitly asks for that capability in a future feature.
- The embedded terminal is user-driven shell access — not part of the Resources read model.
