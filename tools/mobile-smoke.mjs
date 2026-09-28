/**
 * Boots the real Android application - the same cmd/mobile wasm the APK ships -
 * under a small DOM shim, and checks that Go renders the interface on its own.
 *
 *   node tools/mobile-smoke.mjs
 *
 * This is the counterpart to tools/engine-smoke.mjs: that one proves the
 * decryption core, this one proves the whole Go UI, without an emulator.
 */
import { readFileSync, existsSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const repo = resolve(here, "..");
const assets = resolve(repo, "android/app/src/main/assets");

for (const name of ["pantegnos.wasm", "wasm_exec.js", "index.html"]) {
  if (!existsSync(resolve(assets, name))) {
    console.error(`missing assets/${name} - run: ./gradlew buildWasm stageShell stageWasmRuntime`);
    process.exit(1);
  }
}

const fail = (message) => {
  console.error(`FAIL: ${message}`);
  process.exit(1);
};

// --- a DOM just large enough for the renderer -------------------------------

const body = { innerHTML: "" };
const listeners = [];
const styleEl = { textContent: "" };

function element(tag) {
  if (String(tag).toLowerCase() === "style") return styleEl;
  return {
    tagName: String(tag).toUpperCase(),
    id: "",
    style: { cssText: "" },
    textContent: "",
    innerHTML: "",
    hidden: false,
    dataset: {},
    classList: { add() {}, remove() {} },
    focus() {},
    appendChild() {},
    setAttribute() {},
    getAttribute: () => null,
    value: "",
    type: "",
    disabled: false,
    selectionStart: 0,
  };
}

globalThis.window = globalThis;
globalThis.document = {
  body,
  head: { appendChild() {} },
  documentElement: { dir: "ltr" },
  createElement: element,
  getElementById: () => null,
  addEventListener(type, fn) { listeners.push([type, fn]); },
};
globalThis.location = { href: "https://appassets.androidplatform.net/assets/index.html" };
globalThis.scrollX = 0;
globalThis.scrollY = 0;
globalThis.scrollTo = () => {};
// The real setTimeout has to stay: it is how the Go runtime schedules its own
// goroutines, and replacing it starves the program.
globalThis.matchMedia = () => ({ matches: false });
globalThis.atob = (s) => Buffer.from(s, "base64").toString("binary");
globalThis.addEventListener = () => {};

// The Android shell, stubbed: this is the whole contract Go sees.
const hostCalls = [];
globalThis.PantegnosHost = {
  pickFiles: () => { hostCalls.push("pickFiles"); return "[]"; },
  readClipboard: () => "",
  copyToClipboard: (t) => { hostCalls.push("copy:" + t.length); return true; },
  share: () => hostCalls.push("share"),
  save: () => hostCalls.push("save"),
  openExternal: (t) => hostCalls.push("external:" + t),
  loadPrefs: () => "",
  savePrefs: (v) => hostCalls.push("prefs:" + v),
  setDark: (d) => hostCalls.push("dark:" + d),
  haptic: () => hostCalls.push("haptic"),
  locale: () => "en",
};

(0, eval)(readFileSync(resolve(assets, "wasm_exec.js"), "utf8"));

(async () => {
  const bytes = readFileSync(resolve(assets, "pantegnos.wasm"));
  const go = new Go();
  const { instance } = await WebAssembly.instantiate(bytes, go.importObject);
  go.run(instance); // registers pantegnosBoot asynchronously, then parks

  // go.run() yields before the Go main runs, so wait for the export exactly
  // like the shell page does.
  for (let i = 0; i < 250 && typeof globalThis.pantegnosBoot !== "function"; i++) {
    await new Promise((r) => setTimeout(r, 20));
  }
  if (typeof globalThis.pantegnosBoot !== "function") {
    fail("the module did not register pantegnosBoot");
  }
  globalThis.pantegnosBoot();
  // Boot defers its first paint through setTimeout, like every later repaint.
  await new Promise((r) => setTimeout(r, 60));

  const html = body.innerHTML;
  if (!html) fail("Go painted nothing into the body");

  console.log(`rendered ${html.length} bytes of markup`);
  if (!styleEl.textContent) fail("Go did not install the stylesheet");
  console.log(`stylesheet ${styleEl.textContent.length} bytes`);

  for (const needle of [
    "PANTEGNOS",
    'data-theme="system"',
    "No network permission",
    "Drop a config file here",
    "Add files",
    "Recognised extensions",
    ".slip",
    ".happ",
    ".bpf",
    "Nothing to decrypt yet",
  ]) {
    if (!html.includes(needle)) fail(`markup is missing ${JSON.stringify(needle)}`);
  }

  // The mark has to reach the interface from internal/brand, not an inline copy.
  if (!html.includes("M54,27 L77.38,40.5")) {
    fail("the canonical logo geometry is not in the markup");
  }
  if (!html.includes('viewBox="0 0 108 108"')) {
    fail("the logo is missing its viewBox");
  }
  if (!html.includes("var(--grad-a")) {
    fail("the logo is not themed by the page palette");
  }
  console.log("canonical logo is inlined and themed");

  if (!listeners.some(([type]) => type === "click")) {
    fail("Go did not register a click listener");
  }
  console.log(`listeners: ${listeners.map(([t]) => t).join(", ")}`);

  // Drive a real action through the delegated listener and confirm it reaches
  // the host, which is the only thing the Android shell has to get right.
  const click = listeners.find(([type]) => type === "click")[1];
  const target = { tagName: "BUTTON", dataset: { act: "theme" }, getAttribute: () => null };
  let prevented = false;
  click(
    { target, preventDefault: () => { prevented = true; } },
    [],
  );
  if (!prevented) fail("the click listener did not consume the event");

  await new Promise((r) => setTimeout(r, 60));

  // The shim reports a light system, so the toggle must go to dark and tell the
  // shell to use light-on-dark system bars.
  if (!hostCalls.includes("dark:true")) {
    fail(`theme toggle did not reach the host: ${JSON.stringify(hostCalls)}`);
  }
  console.log("theme toggle reached the host and resolved to dark");

  const repainted = body.innerHTML;
  if (!repainted.includes('data-theme="dark"')) {
    fail("the repaint did not switch to the dark theme");
  }
  console.log("repaint switched the theme to dark");

  console.log("\nAll mobile smoke checks passed.");
  process.exit(0);
})().catch((err) => {
  console.error("mobile smoke test crashed:", err);
  process.exit(1);
});
