// Sign a built klew.mcpb with the Developer ID Application certificate, so
// Claude Desktop shows a verified publisher instead of an unknown one.
//
// Usage: node sign.mjs path/to/klew.mcpb path/to/identity.pem
//
// identity.pem is the release's .p12 converted by openssl (certificates and
// private keys, unencrypted, in a private temporary folder). This script picks
// the Developer ID Application certificate and its key, adds the issuing Apple
// intermediate, signs with the mcpb library, then checks the signature
// matches the bundle and the certificate chains to a root macOS trusts.
// With REQUIRE_SIGNING=1 anything short of that is an error.
import { execFileSync } from "node:child_process";
import { readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import forge from "node-forge";
import { extractSignatureBlock, signMcpbFile } from "@anthropic-ai/mcpb";

// Apple's Developer ID intermediates, fetched only when the .p12 lacks them.
const APPLE_INTERMEDIATES = [
  "https://www.apple.com/certificateauthority/DeveloperIDG2CA.cer",
  "https://www.apple.com/certificateauthority/DeveloperIDCA.cer",
];

const [bundle, identityPem] = process.argv.slice(2);
const required = process.env.REQUIRE_SIGNING === "1";

function fail(msg) {
  console.error(`mcpb sign: ${msg}`);
  process.exit(1);
}

if (!bundle || !identityPem) fail("usage: node sign.mjs path/to/klew.mcpb path/to/identity.pem");

const pem = readFileSync(identityPem, "utf8");
const blocks = (label) => pem.match(new RegExp(`-----BEGIN ${label}-----[\\s\\S]+?-----END ${label}-----`, "g")) || [];
const certs = blocks("CERTIFICATE").map((p) => forge.pki.certificateFromPem(p));
const keys = [...blocks("PRIVATE KEY"), ...blocks("RSA PRIVATE KEY")].map((p) => forge.pki.privateKeyFromPem(p));

const cn = (name) => name.getField("CN")?.value || "";
const leaf = certs.find((c) => cn(c.subject).startsWith("Developer ID Application"));
if (!leaf) fail(`no Developer ID Application certificate among: ${certs.map((c) => cn(c.subject)).join(", ") || "none"}`);
const key = keys.find((k) => k.n && k.n.equals(leaf.publicKey.n));
if (!key) fail("no private key matches the Developer ID Application certificate");

async function intermediateFor(cert) {
  const bundled = certs.find((c) => c !== cert && c.subject.hash === cert.issuer.hash);
  if (bundled) return bundled;
  for (const url of APPLE_INTERMEDIATES) {
    const res = await fetch(url);
    if (!res.ok) continue;
    const der = Buffer.from(await res.arrayBuffer()).toString("binary");
    const ca = forge.pki.certificateFromAsn1(forge.asn1.fromDer(der));
    try {
      if (ca.verify(cert)) return ca;
    } catch {
      // Not this issuer.
    }
  }
  return null;
}

// A self-signed certificate (local testing only) has no intermediate.
const selfSigned = leaf.issuer.hash === leaf.subject.hash;
const intermediate = selfSigned ? null : await intermediateFor(leaf);
if (!selfSigned && !intermediate) fail(`could not find the intermediate that issued ${cn(leaf.subject)}`);

const dir = dirname(identityPem);
const certPath = join(dir, "mcpb-cert.pem");
const keyPath = join(dir, "mcpb-key.pem");
const interPath = join(dir, "mcpb-intermediate.pem");
writeFileSync(certPath, forge.pki.certificateToPem(leaf), { mode: 0o600 });
writeFileSync(keyPath, forge.pki.privateKeyToPem(key), { mode: 0o600 });
if (intermediate) writeFileSync(interPath, forge.pki.certificateToPem(intermediate), { mode: 0o600 });

signMcpbFile(bundle, certPath, keyPath, intermediate ? [interPath] : []);

// Check the result ourselves: the mcpb library's verifyMcpbFile reports every
// bundle as unsigned, because node-forge cannot verify PKCS#7 signatures.
// 1. The detached signature matches the bundle's content (openssl).
const { originalContent, pkcs7Signature } = extractSignatureBlock(readFileSync(bundle));
if (!pkcs7Signature) fail("no signature block was appended");
const contentPath = join(dir, "mcpb-content.zip");
const sigPath = join(dir, "mcpb-signature.der");
writeFileSync(contentPath, originalContent, { mode: 0o600 });
writeFileSync(sigPath, pkcs7Signature, { mode: 0o600 });
try {
  execFileSync(process.env.OPENSSL || "openssl", [
    "smime", "-verify", "-binary", "-inform", "DER", "-in", sigPath, "-content", contentPath, "-noverify", "-out", "/dev/null",
  ], { stdio: "pipe" });
} catch (e) {
  fail(`the signature does not match the bundle: ${e.stderr?.toString().trim()}`);
}

// 2. The certificate chains to a root macOS trusts for code signing, the
// check Claude Desktop's extension verifier runs.
const chainPath = join(dir, "mcpb-chain.pem");
writeFileSync(chainPath, [leaf, intermediate].filter(Boolean).map((c) => forge.pki.certificateToPem(c)).join("\n"), { mode: 0o600 });
let trusted = true;
try {
  execFileSync("security", ["verify-cert", "-c", chainPath, "-p", "codeSign"], { stdio: "pipe" });
} catch {
  trusted = false;
}

console.log(`mcpb sign: signed by ${cn(leaf.subject)}; ${trusted ? "trusted by macOS" : "NOT trusted by macOS"}`);
if (!trusted && required) fail("the certificate does not chain to a trusted root");
