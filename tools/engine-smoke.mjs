/**
 * Validates the decryption core exactly as the web decryptor loads it: build
 * ./cmd/wasm for js/wasm and drive the globals it registers.
 *
 *   GOOS=js GOARCH=wasm go build -trimpath -ldflags "-s -w" \
 *     -o android/app/src/main/assets/pantegnos.wasm ./cmd/wasm
 *   cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" android/app/src/main/assets/
 *   node tools/engine-smoke.mjs
 *
 * The Android app does not use these exports - it calls the core in process
 * through internal/mobile - so this covers the browser build, while
 * tools/mobile-smoke.mjs covers the app.
 */
import { readFileSync, existsSync } from "node:fs";
import { gzipSync } from "node:zlib";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const repo = resolve(here, "..");
// The web build lands in web/ (that is what GitHub Pages publishes); override
// with PANTEGNOS_ASSETS to test a build from somewhere else.
const assets = process.env.PANTEGNOS_ASSETS || resolve(repo, "web");

const wasmPath = resolve(assets, "pantegnos.wasm");
const execPath = resolve(assets, "wasm_exec.js");

for (const path of [wasmPath, execPath]) {
  if (!existsSync(path)) {
    console.error(
      `missing ${path}\n` +
        "build the browser core with:\n" +
        '  GOOS=js GOARCH=wasm go build -trimpath -ldflags "-s -w" -o web/pantegnos.wasm ./cmd/wasm\n' +
        '  cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" web/',
    );
    process.exit(1);
  }
}

// The Go program only needs the globals wasm_exec.js touches.
globalThis.window = globalThis;
globalThis.location = { href: "https://frontiertm.github.io/Pantegnos/" };
if (!globalThis.navigator) {
  Object.defineProperty(globalThis, "navigator", {
    value: { userAgent: "node" },
    configurable: true,
  });
}

const fail = (message) => {
  console.error(`FAIL: ${message}`);
  process.exit(1);
};

// bpfProfile assembles a .bpf container the way the sing-box clients do: a
// message-type byte, a version byte, then a gzip stream of varint-prefixed
// strings with an int32BE profile type. Kept here rather than read from disk so
// the smoke test does not depend on a sample file.
const bpfProfile = (version, { name, type, config, remotePath, autoUpdate, interval, updated }) => {
  const parts = [];
  const uvarint = (n) => {
    const out = [];
    while (n >= 0x80) {
      out.push((n & 0x7f) | 0x80);
      n = Math.floor(n / 128);
    }
    out.push(n & 0xff);
    return out;
  };
  const push = (...groups) => {
    for (const g of groups) parts.push(...g);
  };
  const str = (s) => {
    const bs = Array.from(Buffer.from(s ?? "", "utf8"));
    push(uvarint(bs.length), bs);
  };
  const int32 = (v) => push([(v >>> 24) & 0xff, (v >>> 16) & 0xff, (v >>> 8) & 0xff, v & 0xff]);

  str(name);
  int32(type);
  str(config);
  if (type === 2) {
    str(remotePath);
    push([autoUpdate ? 1 : 0]);
    if (version >= 1) int32(interval ?? 0);
    push([0, 0, 0, 0, 0, 0, 0, 0]);
  }

  return new Uint8Array([3, version, ...gzipSync(Buffer.from(parts))]);
};

(0, eval)(readFileSync(execPath, "utf8"));
if (typeof globalThis.Go !== "function") fail("wasm_exec.js did not define globalThis.Go");

(async () => {
  const go = new Go();
  const { instance } = await WebAssembly.instantiate(readFileSync(wasmPath), go.importObject);
  go.run(instance);

  // go.run() is asynchronous, so wait for the exports to be registered.
  for (let i = 0; i < 250 && typeof globalThis.pantegnosDecrypt !== "function"; i++) {
    await new Promise((r) => setTimeout(r, 20));
  }
  for (const name of ["pantegnosDecrypt", "pantegnosNeedsPassword", "pantegnosInspect"]) {
    if (typeof globalThis[name] !== "function") fail(`the core did not register ${name}`);
  }
  console.log("core registered its exports");

  // The exported functions take a Uint8Array, exactly as web/index.html passes
  // one: cmd/wasm ignores anything that is not a typed array.
  const data = (s) => Uint8Array.from(Buffer.from(s, "utf8"));

  // 1. A SlipNet URI is recognised and does not demand a password.
  const slipUri = "slipnet-enc://00000000000000000000000000000000";
  const slip = globalThis.pantegnosInspect("sample.slip", data(slipUri));
  console.log("inspect(.slip) ->", JSON.stringify(slip));
  if (slip.ok !== true || !String(slip.module).includes("SlipNet")) {
    fail(`expected the SlipNet module to claim the file, got ${JSON.stringify(slip)}`);
  }
  if (slip.needsPassword !== false) fail("a plain slipnet:// URI must not demand a password");
  if (globalThis.pantegnosNeedsPassword("sample.slip", data(slipUri)) !== false) {
    fail("needsPassword() disagreed with inspect()");
  }

  // 2. Garbage never decrypts. inspect() may still claim it - the .ehi/.hat
  //    modules register an empty scheme on purpose as a last resort, exactly as
  //    the CLI does - but decrypt() has to report a failure.
  const claimed = globalThis.pantegnosInspect("notes.txt", data("just some text"));
  if (claimed.ok === true) console.log("fallback module for unsupported input:", claimed.module);
  const rejected = globalThis.pantegnosDecrypt("notes.txt", data("just some text"), "");
  if (rejected.ok === true) fail("decrypt() must report failure for unsupported input");
  if (!("error" in rejected)) fail("a failed decrypt() does not report an error");
  console.log("unsupported input rejected:", String(rejected.error).split("\n")[0]);

  // 3. A passphrase-protected payload is flagged, and a wrong passphrase must
  //    never look like a success.
  const npvs = data("npvs://" + "A".repeat(96));
  const npvsInspect = globalThis.pantegnosInspect("protected.npvs", npvs);
  console.log("inspect(.npvs) ->", JSON.stringify(npvsInspect));
  if (npvsInspect.ok !== true) fail("the NPVS module should claim a npvs:// payload");
  const wrong = globalThis.pantegnosDecrypt("protected.npvs", npvs, "wrong-password");
  if (wrong.ok === true) fail("a bogus passphrase must not decrypt");
  console.log("wrong passphrase rejected:", String(wrong.error).split("\n")[0]);

  // 4. Every module the README advertises must be reachable, which also proves
  //    the staged WASM is not a stale build.
  for (const [expected, name, uri] of [
    ["SlipNet", "x.slip", "slipnet://abcd"],
    ["HTTP Injector", "x.ehi", "ehi://abcd"],
    ["DarkTunnel", "x.dark", "darktunnel://abcd"],
    ["Happ", "x.happ", "happ://abcd"],
    ["NetMod", "x.nm", "nm-v3://abcd"],
  ]) {
    const got = globalThis.pantegnosInspect(name, data(uri));
    if (got.ok !== true || !String(got.module).includes(expected)) {
      fail(`${expected} was not claimed, got ${JSON.stringify(got)}`);
    }
    console.log(`inspect -> ${got.module}`);
  }

  // 4b. .bpf is a container, not a cipher, so it has no URI to inspect. Build
  //     one and drive the whole round trip through the exported globals.
  const bpf = bpfProfile(1, {
    name: "🎨@oneclickvpnkeys",
    type: 0,
    config: '{"outbounds":[{"type":"vless","tag":"de","server":"198.51.100.7","server_port":443}]}',
  });
  const bpfInspect = globalThis.pantegnosInspect("sample.bpf", bpf);
  console.log("inspect(.bpf) ->", JSON.stringify(bpfInspect));
  if (bpfInspect.ok !== true || !String(bpfInspect.module).includes("sing-box")) {
    fail(`expected the sing-box module to claim the file, got ${JSON.stringify(bpfInspect)}`);
  }
  if (bpfInspect.needsPassword !== false) fail("a .bpf container is not passphrase protected");
  const bpfOut = globalThis.pantegnosDecrypt("sample.bpf", bpf, "");
  if (bpfOut.ok !== true) fail(`a valid .bpf should decode, got ${JSON.stringify(bpfOut)}`);
  if (bpfOut.fileName !== "sample.txt") fail(`fileName = ${bpfOut.fileName}, want sample.txt`);
  if (!String(bpfOut.text).includes('"server": "198.51.100.7"')) {
    fail(`decoded .bpf lost the config: ${JSON.stringify(bpfOut.text)}`);
  }
  console.log("decrypt() recovers a .bpf profile");

  // 5. The result shape the web front-end relies on, on a real payload. The
  //    profile has to carry a full schema worth of fields or the core rejects
  //    the blob as too short.
  const fields = [
    "20", "ssh", "Node", "example.com", "1.1.1.1,8.8.8.8", "password", "30", "JP",
    "443", "edge.example.com", "1", "", "socksuser", "s3cret", "1", "root", "sshpw",
    "22", "0", "10.0.0.5", "1", "https://dns.example.com/dns-query", "tls", "password",
    "", "", "", "0", "", "", "0", "", "", "0", "", "1232",
  ];
  const profile = Buffer.from(fields.join("|"), "utf8").toString("base64");
  const shape = globalThis.pantegnosDecrypt("x.slip", data("slipnet://" + profile), "");
  if (shape.ok !== true) fail(`a valid profile should decrypt, got ${JSON.stringify(shape)}`);
  for (const key of ["ok", "text", "fileName", "module", "apkAuthor"]) {
    if (!(key in shape)) fail(`a successful decrypt() is missing "${key}"`);
  }
  if (!String(shape.text).includes("edge.example.com")) {
    fail(`decrypted text looks wrong: ${JSON.stringify(shape.text)}`);
  }
  console.log("decrypt() returns the expected shape");

  console.log("\nAll engine smoke checks passed.");
  process.exit(0);
})().catch((err) => {
  console.error("engine smoke test crashed:", err);
  process.exit(1);
});
