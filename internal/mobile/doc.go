// Package mobile contains the Android application: every piece of behaviour and
// every byte of markup the user sees is produced here, compiled to js/wasm and
// rendered by the system WebView. The Android host contributes only the few
// things the platform reserves for itself - the file picker, the share sheet,
// the clipboard, the system bars and persisted preferences.
//
// Nothing in this package may import anything platform specific: it has to build
// and test on a plain `go test` run.
package mobile
