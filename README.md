<h1 align="left">
  <img src="internal/brand/logo-solid.svg" width="48" height="48" alt="" style="vertical-align:middle">
  Pantegnos
</h1>

[![Stars](https://img.shields.io/github/stars/FrontierTM/Pantegnos?style=flat-square)](https://github.com/FrontierTM/Pantegnos/stargazers)
[![Forks](https://img.shields.io/github/forks/FrontierTM/Pantegnos?style=flat-square)](https://github.com/FrontierTM/Pantegnos/network/members)
[![Issues](https://img.shields.io/github/issues/FrontierTM/Pantegnos?style=flat-square)](https://github.com/FrontierTM/Pantegnos/issues)
[![Go](https://img.shields.io/badge/Go-1.26.3+-00ADD8?style=flat-square&logo=go)](https://go.dev/)
[![Site](https://img.shields.io/badge/Web%20Decryptor-frontiertm.github.io%2FPantegnos-ff9e3d?style=flat-square)](https://frontiertm.github.io/Pantegnos/)
[![Android](https://img.shields.io/badge/Android%20APK-Actions-3DDC84?style=flat-square&logo=github-actions)](https://github.com/FrontierTM/Pantegnos/actions/workflows/android.yml)

A multi-platform decryptor for VPN and proxy configuration files used by various Android and desktop clients. Pantegnos extracts readable server metadata from encrypted proprietary formats, making it useful for security researchers analyzing these tools.

One decryption core, three front-ends: a **CLI**, an **in-browser decryptor** at [frontiertm.github.io/Pantegnos](https://frontiertm.github.io/Pantegnos/), and an **Android app** written in Go. Everything runs locally — no telemetry, no uploads.

## Supported Formats

| Format                                            | Extension | Protocol                | Module                                                                           |
|---------------------------------------------------|-----------|-------------------------|----------------------------------------------------------------------------------|
| SlipNet (Encrypted)                               | `.slip`   | `slipnet-enc://`        | AES-256-GCM with hardcoded key                                                   |
| SlipNet (Plaintext)                               | `.slip`   | `slipnet://`            | Base64 decode + profile parse                                                    |
| HTTP Injector (SSH/V2ray) - Native                | `.ehi`    | *(extension-based)*     | Argon2id-KDF + XChaCha20-Poly1305 (custom-obfuscated envelope)                   |
| DarkTunnel - SSH DNSTT V2Ray                      | `.dark`   | `darktunnel://`         | AES‑CFB‑256 -> MessagePack + AES‑CFB‑192                                         |
| SlipNet Bundle (Password)                         | `.slip`   | `slipnet-bundle-enc://` | PBKDF2 (600k iter) + AES-GCM                                                     |
| HA Tunnel Plus                                    | `.hat`    | *(extension-based)*     | AES-ECB (SHA1-derived key)                                                       |
| NpvTunnel (NapsternetV) (NPVT)                    | `.npvt`   | `NPVT1`                 | Custom whitebox AES CTR                                                          |
| NpvTunnel (NapsternetV) (NPVS) (PASSKEY PROTECTED) | `.npvs`   | `NPVS`                  | Custom PBKDF2-HMAC-SHA256 + ChaCha20-Poly1305                                    |
| NpvTunnel (NapsternetV) (NPVS, AppKey embedded)   | `.npvs`   | `NPVS`                  | Custodian whitebox AES-CTR-SHA256 KDF + ChaCha20-Poly1305 (offline, no password) |
| NetMod (Both OLD & NEW)                           | `.nm`     | `nm-*://`               | AES-ECB (fixed key)                                                              |
| Happ Proxy                                        | `.happ`   | `happ://crypt[1-4]/`    | RSA-1024/4096 private key                                                        |
| sing-box profile export (SFA/SFI/SFM)              | `.bpf`    | *(extension-based)*     | gzip container, no cipher (see [Notes](#notes))                                   |

SlipNet profiles support schema versions 1 through 28, covering fields like VLESS, SSH tunneling, SOCKS5, DoH, SNI fragmentation, and more.

## Notes

`.bpf` is the one supported format that is not encrypted. It is the binary
profile export the sing-box clients write when a profile is shared, and
recovering it is a decompress plus a walk through a few length-prefixed fields:

| Offset | Field |
| --- | --- |
| 0 | message type, always `0x03` (profile content) |
| 1 | container version, 0 or 1 |
| 2.. | gzip stream of: varint-prefixed profile name, `int32BE` profile type (0 local, 1 iCloud, 2 remote), varint-prefixed sing-box config, and for remote profiles the source URL, auto-update flag, interval (v1+) and last-updated timestamp |

The output is the sing-box config re-indented, with the profile metadata in a
`//` comment header, so it is a file `sing-box check` accepts. iCloud profiles
are reported as unrecoverable: the clients stream their config from the
account rather than storing it in the export.

## Web Decryptor

The full decryptor also runs in your browser — no install, files never leave your machine:

**[frontiertm.github.io/Pantegnos](https://frontiertm.github.io/Pantegnos/)**

The page hosts the same Go decryption core compiled to WebAssembly (`GOOS=js GOARCH=wasm ./cmd/wasm`), built and published automatically by GitHub Actions from the `web/` directory.

To build and serve it locally:

```bash
# Build the WASM bundle
GOOS=js GOARCH=wasm go build -o web/pantegnos.wasm ./cmd/wasm

# Copy the Go WASM runtime glue (path is GOROOT-dependent)
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" web/wasm_exec.js

# Serve with any static file server, e.g.
python -m http.server -d web 8000
```

`web/pantegnos.wasm` and `web/wasm_exec.js` are build artifacts and are not committed; CI regenerates them on every push to `main`.

## Android App

The same decryption core ships as an Android app in [`android/`](android) — and it is **written in Go**. `cmd/mobile` is the entire application: the queue, the passphrase handling, the translations and every pixel of the interface, compiled to `js/wasm` and rendered by the system WebView. The Kotlin side is a single ~350-line `MainActivity` that provides only what the platform reserves for the app process — the document picker, the share sheet, the clipboard, the system bars and persisted preferences.

The app declares **zero permissions**: no `INTERNET`, no storage. The WebView serves its page from a virtual `https` origin through `WebViewAssetLoader`, the client refuses every other URL, and without `INTERNET` the runtime physically cannot ship a config anywhere.

**Features**

- Pick one or many config files through the system file picker, or paste a `…://` config URI from the clipboard
- Sequential queue with per-file module identification (`.slip`, `.ehi`, `.dark`, `.hat`, `.npvt`, `.npvs`, `.nm`, `.happ`, `.bpf`)
- Passphrase prompt with automatic retry, for `.npvs` bundles and SlipNet bundle files
- Monospaced viewer with selectable text and a wrap/scroll toggle
- Copy, share (as a real `.txt` file via `FileProvider`, so a VPN client can import it) or save anywhere
- Light / dark / system themes, English and Persian (RTL) localisations, vector launcher icon with an Android 13+ themed variant

**Responsive layout**

| Viewport | Layout |
| --- | --- |
| Phone, portrait | Single column, edge to edge, comfortable measure |
| Small phones (≤360dp) | Tighter gutters and furniture, subtitles allowed to wrap |
| Landscape phones (≤560dp tall) | Compact chrome: the tagline and privacy pill drop out so the picker stays above the fold |
| Tablets / unfolded foldables (≥1000px) | Two panes, intake left and results right, with the intake column sticky |
| Very wide (≥1500px) | Growth stops at 1280px so lines stay readable |

Type is set in `rem`, so Android's font-size accessibility setting scales the whole interface, and every edge respects `env(safe-area-inset-*)` for notches in landscape.

**Layout**

| Path | What it is |
| --- | --- |
| `cmd/mobile` | The `js/wasm` entry point; a handful of lines |
| `internal/mobile` | The application: state machine, i18n, markup, stylesheet |
| `internal/mobile/dom_js.go` | The only file that knows it runs in a browser (`//go:build js && wasm`) |
| `internal/mobile/index.html` | A ~30-line shell that loads the Go module and nothing else |
| `internal/brand` | The logo, and the tests that keep every surface showing it |
| `android/.../MainActivity.kt` | The WebView plus the platform bridges |
| `tools/engine-smoke.mjs` | Boots the shipped WASM and exercises every module under Node |
| `tools/mobile-smoke.mjs` | Boots the whole Go interface against a DOM shim and drives a real action |
| `tools/dumppage` | Writes the rendered page and stylesheet to one HTML file, for iterating on the layout in a desktop browser |
| `tools/icon` | Rasterises the logo into the `.ico` the Windows binary embeds |

Because everything except the shell is ordinary Go, the application is tested with `go test`:

```bash
go test ./internal/...             # queue, i18n, rendering, passphrase flow, brand drift
node tools/engine-smoke.mjs        # the decryption core, as shipped
node tools/mobile-smoke.mjs        # the interface, as shipped
```

**Building**

Needs **Go 1.26+**, **JDK 17+** and an Android SDK with platform 35. A Gradle task compiles the Go application, so there is no separate manual step:

```bash
cd android
./gradlew assembleDebug      # app/build/outputs/apk/debug/
./gradlew assembleRelease    # minified, resource-shrunk, signed
```

Signing uses `-PPANTEGNOS_KEYSTORE=… -PPANTEGNOS_STORE_PASSWORD=… -PPANTEGNOS_KEY_ALIAS=… -PPANTEGNOS_KEY_PASSWORD=…`. Without those properties the release variant falls back to the debug key so `assembleRelease` still produces an installable APK.

**Releases**

[`.github/workflows/android.yml`](.github/workflows/android.yml) runs `go vet` and `go test`, both smoke tests, then builds and signs the APKs on every push to `main`, and attaches them to the GitHub release for `v*` tags. Add the repository secrets `KEYSTORE_BASE64`, `KEYSTORE_PASSWORD`, `KEY_ALIAS` and `KEY_PASSWORD` to publish with your own upload key; otherwise CI generates an ephemeral one for that run.

## Versioning

No surface hard-codes a version. Each one receives it from the build, and anything missing, empty, or left as an unresolved template collapses to `dev`:

| Surface | Source | Fallback |
| --- | --- | --- |
| CLI banner | goreleaser `-X main.version={{ .Tag }}` | `dev` |
| Web decryptor | Pages workflow writes `web/version.txt` from the tag, or the commit | `dev` |
| Android app | `android.yml` passes `-PversionName`, Gradle forwards it to `-X main.version` | `dev` |

```bash
go run ./cmd/pantegnos                       # dev
go build -ldflags "-X main.version=1.4.2" .  # v1.4.2
cd android && ./gradlew assembleDebug        # dev
cd android && ./gradlew assembleDebug -PversionName=1.4.2   # v1.4.2
```

`internal/buildinfo` owns the rule, and `go test ./internal/...` covers it, including the awkward inputs a template engine can produce: `""`, whitespace, `null`, `nil`, `<nil>`, and a literal `{{.Version}}`.

## Brand

`internal/brand/logo.svg` is the single source of truth for the mark — a hexagonal frame around a lock with a keyhole. The same geometry appears on the Android launcher icon, in the app and web headers, on the browser tab, and as the icon of the Windows binary in Explorer.

| File | Role |
| --- | --- |
| `internal/brand/logo.svg` | Canonical, themed through CSS custom properties |
| `internal/brand/logo-solid.svg` | Same geometry, literal colours, for a favicon or a README |
| `internal/brand/icon.ico` | Eight frames, 16 → 256, embedded in the Windows binary |
| `cmd/pantegnos/rsrc_windows_*.syso` | COFF resources that give the `.exe` its icon |
| `web/logo.svg`, `web/favicon-*.png` | Copies for the browser, checked against the canonical file |

Regenerate the raster forms after editing the SVG, then rebuild the resources:

```bash
go run ./tools/icon
go run github.com/akavel/rsrc@latest -arch amd64 -ico internal/brand/icon.ico -o cmd/pantegnos/rsrc_windows_amd64.syso
```

Repeat the `rsrc` call for `386` and `arm64` when you change the icon.

`go test ./internal/brand` fails if the Android vector drawable, the web copy or the favicons drift away from the canonical file, so the surfaces cannot silently diverge.

## Usage

1. Place your encrypted config files in a `configs/` directory (or use `-input` to specify one).

2. Run the tool:

```bash
chmod +x Pantegnos
./Pantegnos -input configs -output output
```

3. Decrypted files appear in the output directory as `.txt` files.

### CLI Flags

| Flag      | Default   | Description                                 |
|-----------|-----------|--------------------------------------------|
| `-input`  | `configs` | Directory containing encrypted config files |
| `-output` | `output`  | Directory where decrypted files are saved   |

## Building

Requires **Go 1.26.3** or later.

```bash
go build -o pantegnos ./cmd/pantegnos
```

For cross-compilation:

```bash
# Linux
GOOS=linux GOARCH=amd64 go build -o pantegnos-linux ./cmd/pantegnos

# Windows
GOOS=windows GOARCH=amd64 go build -o pantegnos-win.exe ./cmd/pantegnos
```

Pre-built binaries are available in the [Releases](https://github.com/KernelDotDLL/Pantegnos/releases) section.

## Dependencies

- [colorgrad](https://github.com/mazznoer/colorgrad) — Terminal gradient text
- [termenv](https://github.com/muesli/termenv) — Terminal capabilities
- [golang.org/x/crypto](https://pkg.go.dev/golang.org/x/crypto) — ChaCha20-Poly1305 AEAD
- [golang.org/x/term](https://pkg.go.dev/golang.org/x/term) — Secure password input (bundle mode)

## License

Copyright (c) 2026 FrontierTM. Licensed under the MIT License. See [LICENSE](LICENSE) for details.

---

*This tool is provided as-is for security research purposes. Users are responsible for ensuring their use complies with applicable laws.*
