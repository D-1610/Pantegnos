//go:build js && wasm

// Command mobile is the Android application: the same Go decryption core the
// CLI and the web decryptor use, with the entire interface written in Go and
// rendered into the system WebView by internal/mobile.
//
// The shell page (internal/mobile/index.html) only loads this module; every
// pixel the user sees is produced by internal/mobile.
package main

import (
	"syscall/js"

	"Pantegnos/internal/mobile"
)

// version is baked in by the Gradle build.
var version = "dev"

func main() {
	// The shell page calls this once the Go runtime is up. It blocks for the
	// lifetime of the process, exactly like the web build does.
	js.Global().Set("pantegnosBoot", js.FuncOf(func(js.Value, []js.Value) any {
		mobile.Boot(version)
		return nil
	}))
	select {}
}
