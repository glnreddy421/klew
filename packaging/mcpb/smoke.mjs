// Smoke-test a built klew.mcpb the way Claude Desktop runs it: unpack the
// bundle, resolve its mcp_config with the mcpb library (user_config defaults,
// ${__dirname}), start the server and speak MCP over stdio. Needs no cluster.
//
// Usage: node smoke.mjs path/to/klew-<version>.mcpb
import { spawn } from "node:child_process";
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { getMcpConfigForManifest, unpackExtension, validateManifest } from "@anthropic-ai/mcpb";

const EXPECTED = [
  "klew_list_contexts",
  "klew_investigate",
  "klew_investigation_details",
  "klew_event_history",
  "klew_workload_changes",
  "klew_list_resources",
  "klew_get_resource",
  "klew_pod_logs",
  "klew_list_extensions",
];

function fail(msg) {
  console.error(`mcpb smoke: ${msg}`);
  process.exit(1);
}

// Starts the configured server, lists its tools and stops it.
function listTools(config) {
  return new Promise((resolve, reject) => {
    const proc = spawn(config.command, config.args, {
      env: { ...process.env, ...(config.env || {}), KUBECONFIG: "/nonexistent" },
      stdio: ["pipe", "pipe", "pipe"],
    });
    let out = "";
    let err = "";
    const timer = setTimeout(() => {
      proc.kill();
      reject(new Error(`no tools/list reply within 20s; stderr: ${err}`));
    }, 20000);
    proc.stderr.on("data", (d) => (err += d));
    proc.stdout.on("data", (d) => {
      out += d;
      for (const line of out.split("\n")) {
        if (!line.trim()) continue;
        let msg;
        try {
          msg = JSON.parse(line);
        } catch {
          continue;
        }
        if (msg.id === 2) {
          clearTimeout(timer);
          proc.kill();
          if (msg.error) reject(new Error(JSON.stringify(msg.error)));
          else resolve(msg.result.tools);
        }
      }
    });
    proc.on("error", (e) => {
      clearTimeout(timer);
      reject(e);
    });
    const send = (m) => proc.stdin.write(JSON.stringify(m) + "\n");
    send({
      jsonrpc: "2.0", id: 1, method: "initialize",
      params: { protocolVersion: "2025-06-18", capabilities: {}, clientInfo: { name: "mcpb-smoke", version: "0" } },
    });
    send({ jsonrpc: "2.0", method: "notifications/initialized" });
    send({ jsonrpc: "2.0", id: 2, method: "tools/list" });
  });
}

async function resolveConfig(manifest, dir, userConfig) {
  const config = await getMcpConfigForManifest({
    manifest, extensionPath: dir, systemDirs: {}, userConfig, pathSeparator: "/",
  });
  if (!config) fail("mcp_config did not resolve");
  const unresolved = [config.command, ...config.args].filter((a) => a.includes("${"));
  if (unresolved.length) fail(`unresolved variables: ${unresolved.join(" ")}`);
  return config;
}

const bundle = process.argv[2];
if (!bundle) fail("usage: node smoke.mjs path/to/klew.mcpb");

const dir = mkdtempSync(join(tmpdir(), "klew-mcpb-"));
try {
  if (!(await unpackExtension({ mcpbPath: bundle, outputDir: dir, silent: true }))) fail("unpack failed");
  if (!validateManifest(join(dir, "manifest.json"))) fail("manifest is invalid");
  const manifest = JSON.parse(readFileSync(join(dir, "manifest.json"), "utf8"));

  const listed = manifest.tools.map((t) => t.name).sort();
  if (JSON.stringify(listed) !== JSON.stringify([...EXPECTED].sort())) fail(`manifest tools: ${listed}`);

  // Defaults, as a fresh install runs it.
  const tools = await listTools(await resolveConfig(manifest, dir, {}));
  const names = tools.map((t) => t.name).sort();
  if (JSON.stringify(names) !== JSON.stringify([...EXPECTED].sort())) fail(`server tools: ${names}`);
  for (const t of tools) {
    const a = t.annotations || {};
    if (a.readOnlyHint !== true || a.destructiveHint !== false) fail(`${t.name} is not marked read-only`);
  }

  // "Leave out log samples" must reach the server: klew_pod_logs goes away.
  const quiet = await listTools(await resolveConfig(manifest, dir, { no_log_samples: true }));
  if (quiet.some((t) => t.name === "klew_pod_logs")) fail("no_log_samples did not reach the server");

  console.log(`mcpb smoke: ok (${manifest.name} ${manifest.version}, ${tools.length} read-only tools)`);
} finally {
  rmSync(dir, { recursive: true, force: true });
}
