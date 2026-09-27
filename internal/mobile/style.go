package mobile

// appCSS is served once, before the first paint.
//
// Layout strategy, in the order a phone, a foldable and a tablet each get:
//
//	< 600px          one column, edge to edge
//	600 - 999px      one column, centred, comfortable measure
//	>= 1000px        two panes: intake on the left, the queue on the right
//	short viewport   compact chrome, so the primary action stays above the fold
//
// Type is in rem so Android's font-size accessibility setting scales the whole
// interface; only the wordmark uses vw, and it is clamped at both ends.
const appCSS = `
/* Dark is the default scope so that anything resolving a custom property against
   :root - including body - gets the palette the app ships in. Light is the
   explicit override, mirroring the web decryptor. */
:root, .shell[data-theme="dark"], .shell[data-theme="system"] {
  color-scheme: dark;
  --bg: #120c08; --surface: #1d140c; --surface-2: #291c11; --surface-3: #352415;
  --hairline: #3d2a1a; --ink: #ede4d8; --muted: #a89483;
  --grad-a: #ffd9a0; --grad-b: #ff6b1a; --on-grad: #2a1400;
  --ok: #7adf72; --bad: #ff8a80; --warn: #ff9e3d; --busy: #a89483;
  --shadow: 0 1px 2px rgba(0,0,0,.4), 0 8px 24px rgba(0,0,0,.35);
  --gap: 12px;
}
.shell[data-theme="light"] {
  color-scheme: light;
  --bg: #faf6f0; --surface: #ffffff; --surface-2: #f4ede1; --surface-3: #ede2d1;
  --hairline: #e0d5c8; --ink: #2a2118; --muted: #7a6a58;
  --grad-a: #e0862a; --grad-b: #b45309; --on-grad: #ffffff;
  --ok: #2e9e44; --bad: #c62828; --warn: #b45309; --busy: #7a6a58;
  --shadow: 0 1px 2px rgba(42,33,24,.06), 0 8px 24px rgba(42,33,24,.06);
}
@media (prefers-color-scheme: light) {
  .shell[data-theme="system"] {
    color-scheme: light;
    --bg: #faf6f0; --surface: #ffffff; --surface-2: #f4ede1; --surface-3: #ede2d1;
    --hairline: #e0d5c8; --ink: #2a2118; --muted: #7a6a58;
    --grad-a: #e0862a; --grad-b: #b45309; --on-grad: #ffffff;
    --ok: #2e9e44; --bad: #c62828; --warn: #b45309; --busy: #7a6a58;
    --shadow: 0 1px 2px rgba(42,33,24,.06), 0 8px 24px rgba(42,33,24,.06);
  }
}

* { box-sizing: border-box; -webkit-tap-highlight-color: transparent; }
html { -webkit-text-size-adjust: 100%; }
html, body { margin: 0; padding: 0; background: #120c08; overscroll-behavior: none; }
body {
  font-family: system-ui, -apple-system, "Segoe UI", Roboto, "Vazirmatn", sans-serif;
  color: var(--ink);
}
.shell[dir="rtl"] { font-family: system-ui, "Vazirmatn", sans-serif; }

.shell {
  background: var(--bg);
  /* Must be declared here, not on body: the palette override is scoped to
     .shell, so an inherited colour has to resolve here too. */
  color: var(--ink);
  min-height: 100vh;
  min-height: 100dvh;
  display: flex;
  flex-direction: column;
  gap: var(--gap);
  width: 100%;
  max-width: 720px;
  margin: 0 auto;
  padding-left: max(16px, env(safe-area-inset-left));
  padding-right: max(16px, env(safe-area-inset-right));
  padding-top: max(14px, env(safe-area-inset-top));
  padding-bottom: calc(40px + env(safe-area-inset-bottom));
}
.pane-intake, .pane-queue {
  display: flex;
  flex-direction: column;
  gap: var(--gap);
  min-width: 0;
}

svg { display: block; }

/* header -------------------------------------------------------------- */
.brand { display: flex; align-items: center; gap: 12px; }
/* The mark is shared with the web page, so it carries no classes of its own;
   each surface sizes it. */
.brand > svg { width: 40px; height: 40px; flex: 0 0 40px; }
.brand-text { flex: 1; min-width: 0; }
.brand h1 {
  margin: 0;
  font-size: clamp(1.15rem, 5vw, 1.45rem);
  font-weight: 800;
  line-height: 1.15;
  letter-spacing: 0.2em;
  background: linear-gradient(90deg, var(--grad-a), var(--grad-b));
  -webkit-background-clip: text; background-clip: text; color: transparent;
}
.brand p {
  margin: 2px 0 0; font-size: 0.78rem; color: var(--muted);
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}
.icon {
  border: 0; background: transparent; color: var(--muted);
  width: 42px; height: 42px; border-radius: 50%;
  display: grid; place-items: center; cursor: pointer; padding: 0; flex: 0 0 auto;
  transition: color .15s, background .15s;
}
.icon svg { width: 21px; height: 21px; }
.icon:hover, .icon:focus-visible { color: var(--ink); background: var(--surface-2); }
.icon.sm { width: 34px; height: 34px; }
.icon.sm svg { width: 17px; height: 17px; }

/* pills and strips ----------------------------------------------------- */
.pill {
  display: flex; align-items: center; gap: 8px; align-self: flex-start;
  max-width: 100%; flex-wrap: wrap;
  background: var(--surface-2); border: 1px solid var(--hairline);
  border-radius: 999px; padding: 6px 12px; font-size: 0.72rem; color: var(--muted);
}
.pill .shield { width: 14px; height: 14px; flex: 0 0 14px; }
.pill .version {
  background: var(--surface-3); color: var(--ink);
  border-radius: 999px; padding: 2px 8px; font-size: 0.7rem;
  white-space: nowrap;
}
.strip {
  display: flex; align-items: center; gap: 10px;
  background: var(--surface); border: 1px solid var(--hairline);
  border-radius: 14px; padding: 12px 14px; font-size: 0.875rem;
}
.dot { width: 9px; height: 9px; border-radius: 50%; background: var(--busy); flex: 0 0 9px; }
.dot.ok { background: var(--ok); }
.dot.bad { background: var(--bad); }
.dot.warn { background: var(--warn); }
.dot.busy { background: var(--grad-a); }
.pulse { animation: breathe 1.5s ease-in-out infinite; }
@keyframes breathe { 0%,100% { transform: scale(1); opacity: .5 } 50% { transform: scale(1.5); opacity: 1 } }

/* dropzone ------------------------------------------------------------- */
.drop {
  position: relative; border: 2px dashed var(--hairline); border-radius: 20px;
  padding: 22px 20px; text-align: center; cursor: pointer;
  display: flex; flex-direction: column; align-items: center; gap: 5px;
  background:
    radial-gradient(120% 70% at 50% 0%, color-mix(in srgb, var(--grad-a) 14%, transparent), transparent 70%),
    color-mix(in srgb, var(--surface) 60%, transparent);
  transition: border-color .15s, transform .12s;
}
.drop:active { transform: scale(.99); }
.drop:focus-visible { outline: 2px solid var(--grad-a); outline-offset: 3px; }
.drop-icon {
  width: 52px; height: 52px; border-radius: 50%;
  background: color-mix(in srgb, var(--grad-a) 22%, transparent);
  display: grid; place-items: center; margin-bottom: 8px; color: var(--grad-a);
}
.drop-icon svg { width: 24px; height: 24px; }
.drop strong { font-size: 0.94rem; }
.drop .hint { font-size: 0.78rem; color: var(--muted); max-width: 34ch; }
.drop-actions { display: flex; align-items: center; justify-content: center; gap: 2px; margin-top: 12px; flex-wrap: wrap; }

/* buttons -------------------------------------------------------------- */
.btn {
  border: 1px solid transparent; border-radius: 999px; cursor: pointer;
  font: inherit; font-size: 0.84rem; font-weight: 600; padding: 9px 16px;
  display: inline-flex; align-items: center; gap: 6px; color: var(--ink);
  background: transparent; white-space: nowrap;
  transition: opacity .15s, background .15s;
}
.btn svg { width: 16px; height: 16px; flex: 0 0 16px; }
.btn.primary {
  background: linear-gradient(90deg, var(--grad-a), var(--grad-b)); color: var(--on-grad);
}
.btn.ghost { color: var(--grad-a); }
.btn.ghost:hover { background: color-mix(in srgb, var(--grad-a) 12%, transparent); }
.btn.sm { font-size: 0.78rem; padding: 7px 11px; }
.btn:disabled { opacity: .45; cursor: default; }
.label { margin: 0; font-size: 0.72rem; letter-spacing: .03em; color: var(--muted); }
.chips { display: flex; flex-wrap: wrap; gap: 6px; }
.chip {
  background: var(--surface-2); border: 1px solid var(--hairline);
  border-radius: 8px; padding: 3px 8px; font-size: 0.7rem; color: var(--muted);
  white-space: nowrap;
}

/* queue ---------------------------------------------------------------- */
.queue-head { display: flex; align-items: center; gap: 8px; }
.queue-head strong { flex: 1; font-size: 0.875rem; min-width: 0; }
.card {
  background: var(--surface); border: 1px solid var(--hairline);
  border-radius: 18px; padding: 14px; box-shadow: var(--shadow);
}
.card.tone-bad { border-color: color-mix(in srgb, var(--bad) 45%, transparent); }
.card-head { display: flex; align-items: center; gap: 12px; }
.ext {
  width: 44px; height: 44px; flex: 0 0 44px; border-radius: 12px;
  display: grid; place-items: center; font-size: 0.75rem; font-weight: 700;
  letter-spacing: .03em;
  background: color-mix(in srgb, var(--ok) 16%, transparent);
  border: 1px solid color-mix(in srgb, var(--ok) 32%, transparent);
  color: var(--ok);
}
.card.tone-bad .ext { background: color-mix(in srgb, var(--bad) 14%, transparent);
  border-color: color-mix(in srgb, var(--bad) 30%, transparent); color: var(--bad); }
.card.tone-warn .ext, .card.tone-busy .ext { background: color-mix(in srgb, var(--grad-a) 16%, transparent);
  border-color: color-mix(in srgb, var(--grad-a) 32%, transparent); color: var(--grad-a); }
.card-title { flex: 1; min-width: 0; }
.card-title strong {
  display: block; font-size: 0.94rem; overflow: hidden;
  text-overflow: ellipsis; white-space: nowrap;
}
.card-title .sub {
  display: block; font-size: 0.75rem; color: var(--muted); margin-top: 3px;
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}
.status {
  display: inline-flex; align-items: center; gap: 6px; font-size: 0.7rem;
  padding: 4px 10px; border-radius: 999px; white-space: nowrap; flex: 0 0 auto;
  color: var(--busy); background: color-mix(in srgb, var(--busy) 12%, transparent);
  border: 1px solid color-mix(in srgb, var(--busy) 30%, transparent);
}
.status.ok { color: var(--ok); background: color-mix(in srgb, var(--ok) 12%, transparent);
  border-color: color-mix(in srgb, var(--ok) 30%, transparent); }
.status.bad { color: var(--bad); background: color-mix(in srgb, var(--bad) 12%, transparent);
  border-color: color-mix(in srgb, var(--bad) 30%, transparent); }
.status.warn { color: var(--warn); background: color-mix(in srgb, var(--warn) 12%, transparent);
  border-color: color-mix(in srgb, var(--warn) 30%, transparent); }
.alert {
  display: flex; gap: 8px; align-items: flex-start; margin-top: 10px;
  background: color-mix(in srgb, var(--bad) 14%, transparent);
  border-radius: 11px; padding: 9px 11px; font-size: 0.78rem;
  overflow-wrap: anywhere;
}
.alert svg { width: 15px; height: 15px; flex: 0 0 15px; margin-top: 1px; color: var(--bad); }
.card-actions { display: flex; align-items: center; margin-top: 6px; gap: 2px; }
.spacer { flex: 1; }

/* result --------------------------------------------------------------- */
.result { margin-top: 8px; }
.mono {
  margin: 0; background: var(--bg); border: 1px solid var(--hairline);
  border-radius: 14px; padding: 14px; max-height: 340px; overflow: auto;
  font-family: ui-monospace, "SFMono-Regular", Menlo, Consolas, monospace;
  font-size: 0.75rem; line-height: 1.55; white-space: pre-wrap; overflow-wrap: anywhere;
  color: var(--ink); -webkit-user-select: text; user-select: text;
  overscroll-behavior: contain;
}
.mono.nowrap { white-space: pre; overflow-wrap: normal; }
.tools { display: flex; flex-wrap: wrap; gap: 2px; margin-top: 8px; }
.muted { color: var(--muted); font-size: 0.78rem; margin: 8px 0 0; }
.tiny { font-size: 0.7rem; color: var(--muted); overflow-wrap: anywhere; }

.empty {
  text-align: center; border: 1px solid var(--hairline); border-radius: 18px;
  padding: 26px 20px; background: color-mix(in srgb, var(--surface) 60%, transparent);
  display: flex; flex-direction: column; gap: 6px; align-items: center;
}
.empty strong { font-size: 0.94rem; }
.empty span { font-size: 0.78rem; color: var(--muted); max-width: 44ch; }

.page-foot { display: flex; flex-direction: column; gap: 10px; margin-top: 2px; }
/* Keep the prose readable instead of letting it run the full width of a
   tablet, where a single line would be well over a hundred characters. */
.page-foot .note, .page-foot > .tiny { max-width: 88ch; }
.note {
  display: flex; gap: 9px; align-items: flex-start; margin: 0;
  background: color-mix(in srgb, var(--surface) 60%, transparent);
  border: 1px solid var(--hairline); border-radius: 12px; padding: 10px 12px;
  font-size: 0.75rem; color: var(--muted);
}
.note svg { width: 15px; height: 15px; flex: 0 0 15px; margin-top: 1px; color: var(--grad-a); }

/* sheets --------------------------------------------------------------- */
.scrim { position: fixed; inset: 0; background: rgba(0,0,0,.5); z-index: 10; }
.sheet {
  position: fixed; z-index: 11; left: 0; right: 0; bottom: 0;
  background: var(--surface); border-radius: 22px 22px 0 0;
  padding: 10px 20px calc(16px + env(safe-area-inset-bottom));
  padding-left: max(20px, env(safe-area-inset-left));
  padding-right: max(20px, env(safe-area-inset-right));
  box-shadow: 0 -8px 32px rgba(0,0,0,.35);
  max-width: 720px; margin: 0 auto;
  max-height: 88vh; max-height: 88dvh; overflow-y: auto;
  overscroll-behavior: contain;
  animation: rise .22s ease-out;
}
.settings { max-height: 82vh; max-height: 82dvh; }
@keyframes rise { from { transform: translateY(18px); opacity: 0 } to { transform: none; opacity: 1 } }
.grip { width: 40px; height: 4px; border-radius: 999px; background: var(--hairline); margin: 0 auto 14px; }
.sheet-head { display: flex; align-items: center; gap: 11px; }
.mini-emblem { width: 30px; height: 30px; flex: 0 0 30px; }
.mini-emblem svg { width: 30px; height: 30px; }
.sheet-head strong { display: block; font-size: 1.125rem; }
.sheet-head .accent { font-size: 0.75rem; color: var(--grad-a); }
.field { display: block; margin-top: 14px; position: relative; }
.field > span { display: block; font-size: 0.75rem; color: var(--muted); margin-bottom: 6px; }
.field input {
  width: 100%; font: inherit; font-size: 0.94rem; color: var(--ink);
  background: var(--bg); border: 1.5px solid var(--hairline);
  border-radius: 12px; padding: 12px 44px 12px 14px; outline: none;
  transition: border-color .15s;
}
.shell[dir="rtl"] .field input, [dir="rtl"] .field input { padding: 12px 14px 12px 44px; }
.field input:focus { border-color: var(--grad-a); }
.field.has-error input { border-color: var(--bad); }
.icon.in-field {
  position: absolute; bottom: 3px; inset-inline-end: 3px; width: 38px; height: 38px;
}
.field-error { margin: 9px 0 0; font-size: 0.78rem; color: var(--bad); }
.field-error .tiny { display: block; }
.sheet-actions { display: flex; align-items: center; gap: 10px; margin-top: 18px; flex-wrap: wrap; }
.sheet-actions .spacer { flex: 1; }

/* settings sheet ------------------------------------------------------- */
.settings h3 { font-size: 0.875rem; margin: 20px 0 8px; }
.settings ul { list-style: none; margin: 0; padding: 0; }
.settings li {
  display: flex; gap: 10px; align-items: baseline; padding: 5px 0;
  font-size: 0.78rem; color: var(--muted);
}
.seg { display: flex; gap: 8px; flex-wrap: wrap; }
.seg button {
  font: inherit; font-size: 0.81rem; padding: 7px 14px; border-radius: 999px;
  cursor: pointer; border: 1px solid var(--hairline); background: transparent;
  color: var(--ink);
}
.seg button[aria-pressed="true"] {
  background: color-mix(in srgb, var(--grad-a) 22%, transparent);
  border-color: color-mix(in srgb, var(--grad-a) 45%, transparent);
  color: var(--ink);
}

/* toast ---------------------------------------------------------------- */
.toast {
  position: fixed; left: 50%; bottom: calc(22px + env(safe-area-inset-bottom));
  transform: translateX(-50%); z-index: 20;
  background: var(--surface-3); color: var(--ink);
  border: 1px solid var(--hairline); border-radius: 999px;
  padding: 10px 18px; font-size: 0.81rem; box-shadow: var(--shadow);
  animation: rise .2s ease-out; max-width: min(88vw, 420px); text-align: center;
}

/* ---------------------------------------------------------------------- */
/* Breakpoints                                                            */
/* ---------------------------------------------------------------------- */

/* Comfortable measure once there is room for it. */
@media (min-width: 600px) {
  .shell { max-width: 680px; --gap: 14px; }
}

/* Small phones: keep the chrome, shrink the furniture. */
@media (max-width: 360px) {
  .shell {
    padding-left: max(12px, env(safe-area-inset-left));
    padding-right: max(12px, env(safe-area-inset-right));
    --gap: 10px;
  }
  .brand { gap: 9px; }
  .brand h1 { letter-spacing: 0.14em; }
  .card { padding: 12px; }
  .card-head { gap: 9px; }
  .ext { width: 38px; height: 38px; flex-basis: 38px; }
  .status { padding: 3px 8px; gap: 5px; }
  .card-title .sub { white-space: normal; }
  .drop { padding: 18px 14px; }
  .sheet { padding-left: 16px; padding-right: 16px; }
}

/* Landscape phones and any other short viewport: the dropzone is the whole
   screen, so everything above it gets tighter. */
@media (max-height: 560px) {
  .shell { --gap: 8px; padding-top: max(8px, env(safe-area-inset-top)); }
  .brand { gap: 9px; }
  .brand > svg { width: 30px; height: 30px; flex-basis: 30px; }
  .brand h1 { font-size: 1.1rem; }
  .brand p, .pill { display: none; }
  .icon { width: 38px; height: 38px; }
  .strip { padding: 9px 12px; }
  .drop { padding: 14px 16px; }
  .drop-icon { width: 38px; height: 38px; margin-bottom: 6px; }
  .drop-icon svg { width: 19px; height: 19px; }
  .drop-actions { margin-top: 8px; }
  .label { display: none; }
  .mono { max-height: 200px; }
  .card { padding: 11px 12px; }
  .empty { padding: 16px; }
}

/* Tablets, foldables opened, and landscape desktops: two panes. The intake
   column sticks so the picker stays reachable while the queue scrolls. */
@media (min-width: 1000px) {
  .shell {
    max-width: 1180px;
    display: grid;
    grid-template-columns: minmax(300px, 400px) minmax(0, 1fr);
    grid-template-areas:
      "brand  brand"
      "intake queue"
      "foot   foot";
    align-items: start;
    /* normal behaves as stretch for grid rows, which would push the panes apart
       on a tall screen. Keep every row its natural height. */
    align-content: start;
    column-gap: 28px;
    row-gap: 16px;
    padding-top: max(20px, env(safe-area-inset-top));
  }
  .brand { grid-area: brand; }
  .pane-intake {
    grid-area: intake;
    position: sticky;
    top: calc(20px + env(safe-area-inset-top));
  }
  .pane-queue { grid-area: queue; }
  .page-foot { grid-area: foot; }
  .brand p { white-space: normal; }
  .mono { max-height: 460px; }
}

/* Very wide: the content stops growing so lines stay readable. */
@media (min-width: 1500px) {
  .shell { max-width: 1280px; }
}

/* Touch targets: 44dp is the platform guidance, so coarse pointers get it. */
@media (pointer: coarse) {
  .icon { width: 44px; height: 44px; }
  .icon.sm { width: 40px; height: 40px; }
  .btn { padding: 11px 18px; }
  .btn.sm { padding: 9px 13px; }
}

@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after { animation: none !important; transition: none !important; }
  .pulse { opacity: 1; }
}
`
