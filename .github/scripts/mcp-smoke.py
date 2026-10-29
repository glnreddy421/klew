#!/usr/bin/env python3
"""Smoke-test a built klew-mcp binary over stdio.

Speaks MCP JSON-RPC to the binary and checks that it initializes, lists the
expected tools, and marks every tool read-only and non-destructive. Needs no
cluster: a tool call without a kubeconfig must come back as a tool error, not
crash the server.

Usage: mcp-smoke.py path/to/klew-mcp
"""

import json
import subprocess
import sys
import threading

EXPECTED_TOOLS = {
    "klew_list_contexts",
    "klew_investigate",
    "klew_investigation_details",
    "klew_event_history",
    "klew_workload_changes",
}


def main() -> int:
    if len(sys.argv) != 2:
        print(__doc__.strip(), file=sys.stderr)
        return 2
    requests = [
        {"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {
            "protocolVersion": "2025-06-18", "capabilities": {},
            "clientInfo": {"name": "ci-smoke", "version": "0"}}},
        {"jsonrpc": "2.0", "method": "notifications/initialized"},
        {"jsonrpc": "2.0", "id": 2, "method": "tools/list"},
        {"jsonrpc": "2.0", "id": 3, "method": "tools/call", "params": {
            "name": "klew_investigation_details",
            "arguments": {"investigation_id": "inv_000000000000", "section": "pods"}}},
    ]
    proc = subprocess.Popen(
        [sys.argv[1], "-no-history"], stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
        text=True, env={"PATH": "/usr/bin:/bin", "HOME": "/nonexistent", "KUBECONFIG": "/nonexistent/kubeconfig"},
    )
    timer = threading.Timer(60, proc.kill)
    timer.start()
    replies = {}
    try:
        for r in requests:
            proc.stdin.write(json.dumps(r) + "\n")
            proc.stdin.flush()
            if "id" not in r:
                continue
            line = proc.stdout.readline()
            if not line:
                break
            try:
                msg = json.loads(line)
            except json.JSONDecodeError:
                return fail(f"non-JSON on stdout (logs must go to stderr): {line!r}", proc)
            replies[msg.get("id")] = msg
        # Closing stdin ends the session; the server must exit cleanly.
        proc.stdin.close()
        code = proc.wait()
    finally:
        timer.cancel()
    if code != 0:
        return fail(f"server exited with {code}", proc)

    errors = []
    init = replies.get(1, {}).get("result")
    if not init or init.get("serverInfo", {}).get("name") != "klew":
        errors.append(f"initialize failed: {replies.get(1)}")

    tools = replies.get(2, {}).get("result", {}).get("tools", [])
    names = {t["name"] for t in tools}
    if names != EXPECTED_TOOLS:
        errors.append(f"tools mismatch: missing {sorted(EXPECTED_TOOLS - names)}, unexpected {sorted(names - EXPECTED_TOOLS)}")
    for t in tools:
        a = t.get("annotations") or {}
        if a.get("readOnlyHint") is not True or a.get("destructiveHint") is not False:
            errors.append(f"{t['name']} is not annotated read-only and non-destructive: {a}")
        if not t.get("description"):
            errors.append(f"{t['name']} has no description")

    call = replies.get(3, {}).get("result", {})
    if call.get("isError") is not True:
        errors.append(f"unknown investigation_id should return a tool error, got {replies.get(3)}")

    if errors:
        return fail("\n".join(errors), proc)
    print(f"klew-mcp smoke OK: {len(tools)} read-only tools, version {init['serverInfo'].get('version')}")
    return 0


def fail(msg: str, proc: subprocess.Popen) -> int:
    print(f"klew-mcp smoke FAILED:\n{msg}", file=sys.stderr)
    proc.kill()
    stderr = proc.stderr.read() if proc.stderr else ""
    if stderr:
        print("--- server stderr ---\n" + stderr[-4000:], file=sys.stderr)
    return 1


if __name__ == "__main__":
    sys.exit(main())
