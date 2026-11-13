"""Calls klew mcp like an AI client would, saves every raw reply, and checks
the replies for Secret values and YAML. Secret values are compared in memory
and never printed."""
import base64, json, os, re, subprocess, sys
sys.path.insert(0, sys.argv[1])
from mcp_client import Client

OUT = sys.argv[2]
CTX, NS = "docker-desktop", "holdout"
os.makedirs(OUT, exist_ok=True)

calls = [
    ("investigate", "klew_investigate", {"kube_context": CTX, "namespace": NS, "observe_seconds": 5}),
    ("list-secrets", "klew_list_resources", {"kube_context": CTX, "namespace": NS, "resource_type": "secrets"}),
    ("get-secret", "klew_get_resource", {"kube_context": CTX, "namespace": NS, "resource_type": "secrets", "name": "ledger-db"}),
    ("get-secret-with-manifest", "klew_get_resource", {"kube_context": CTX, "namespace": NS, "resource_type": "secrets", "name": "ledger-db", "include_manifest": True}),
    ("get-deployment", "klew_get_resource", {"kube_context": CTX, "namespace": NS, "resource_type": "deployments", "name": "ledger"}),
    ("events", "klew_event_history", {"kube_context": CTX, "namespace": NS, "limit": 20}),
]

c = Client(["/opt/homebrew/bin/klew", "mcp"])
replies = {}
for label, tool, args in calls:
    raw, msg = c.tool(tool, args)
    replies[label] = raw
    with open(os.path.join(OUT, f"{label}.json"), "w") as f:
        f.write(raw)
    err = msg.get("result", {}).get("isError")
    print(f"{label:26} {tool:22} {len(raw):7} bytes  error={bool(err)}")
c.close()

# Every Secret value in the namespace, straight from the cluster.
secrets = json.loads(subprocess.check_output(["kubectl", "--context", CTX, "-n", NS, "get", "secrets", "-o", "json"]))
values = []
for s in secrets["items"]:
    for k, b64 in (s.get("data") or {}).items():
        plain = base64.b64decode(b64).decode("utf-8", "replace")
        values.append((s["metadata"]["name"], k, b64, plain))

print("\nSecret values in namespace (values not shown):")
leaks = 0
for name, key, b64, plain in values:
    hits = [l for l, raw in replies.items() if (b64 in raw) or (len(plain) >= 4 and plain in raw)]
    leaks += len(hits)
    print(f"  {name}/{key}: {len(plain)} chars -> found in replies: {', '.join(hits) or 'none'}")

print("\nYAML manifest in reply:")
for label, raw in replies.items():
    text = json.loads(raw)["result"]["content"][0]["text"]
    has = bool(re.search(r"(^|\\n)apiVersion: ", text))
    print(f"  {label:26} {'YES' if has else 'no'}")
print(f"\nSecret values leaked: {leaks}")
