import sys
sys.path.insert(0, sys.argv[1])
from mcp_client import Client
c = Client(["/opt/homebrew/bin/klew", "mcp"])
_, msg = c.call_raw("tools/list", {})
for t in msg["result"]["tools"]:
    a = t.get("annotations", {})
    props = ", ".join(t["inputSchema"].get("properties", {}).keys())
    print(f'{t["name"]:30} readOnly={a.get("readOnlyHint")} destructive={a.get("destructiveHint")}  params: {props}')
c.close()
