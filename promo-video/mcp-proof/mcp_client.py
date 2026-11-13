"""Minimal MCP stdio client: sends JSON-RPC to `klew mcp` and returns raw replies."""
import json, subprocess

class Client:
    def __init__(self, cmd):
        self.p = subprocess.Popen(cmd, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, text=True)
        self.n = 0
        self.call_raw("initialize", {"protocolVersion": "2025-06-18", "capabilities": {}, "clientInfo": {"name": "proof", "version": "0"}})
        self.send({"jsonrpc": "2.0", "method": "notifications/initialized"})

    def send(self, msg):
        self.p.stdin.write(json.dumps(msg) + "\n"); self.p.stdin.flush()

    def call_raw(self, method, params):
        self.n += 1
        self.send({"jsonrpc": "2.0", "id": self.n, "method": method, "params": params})
        while True:
            line = self.p.stdout.readline()
            msg = json.loads(line)
            if msg.get("id") == self.n:
                return line, msg

    def tool(self, name, args):
        return self.call_raw("tools/call", {"name": name, "arguments": args})

    def close(self):
        self.p.stdin.close(); self.p.wait(timeout=10)
