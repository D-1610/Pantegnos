package mobile

import (
	"Pantegnos/internal/modules"

	// Registers every decryption module with the shared registry.
	_ "Pantegnos/internal/modules/impl"
)

// Engine adapts the shared decryption core to the UI: it answers "which module
// claims this file?" before any crypto runs, and performs the decryption.
type Engine struct{}

// Inspection is what the core knows about a file before decrypting it.
type Inspection struct {
	Supported     bool
	Module        string
	ApkAuthor     string
	Proto         string
	NeedsPassword bool
}

// DecryptOutcome is the result of a single decryption attempt.
type DecryptOutcome struct {
	OK        bool
	Text      string
	FileName  string
	Module    string
	ApkAuthor string
	Err       string
}

// Inspect identifies the module for a file without doing any work on it.
func (Engine) Inspect(name string, data []byte) Inspection {
	mod, proto, payload := modules.Lookup(name, data)
	if mod == nil {
		return Inspection{}
	}
	needs := false
	if mod.NeedsPassword != nil {
		needs = mod.NeedsPassword(proto, payload)
	}
	return Inspection{
		Supported:     true,
		Module:        mod.Name,
		ApkAuthor:     mod.ApkAuthor,
		Proto:         proto,
		NeedsPassword: needs,
	}
}

// Decrypt runs the module that claims this file.
func (Engine) Decrypt(name string, data []byte, password string) DecryptOutcome {
	mod, proto, payload := modules.Lookup(name, data)
	if mod == nil {
		return DecryptOutcome{Err: "no module supports this file type"}
	}
	result, err := mod.Decrypt(modules.Request{
		FileName: name,
		Data:     data,
		Proto:    proto,
		Payload:  payload,
		Password: password,
	})
	if err != nil {
		return DecryptOutcome{Err: err.Error(), Module: mod.Name, ApkAuthor: mod.ApkAuthor}
	}
	return DecryptOutcome{
		OK:        true,
		Text:      result.Text,
		FileName:  result.FileName,
		Module:    mod.Name,
		ApkAuthor: mod.ApkAuthor,
	}
}

// maxInputBytes guards the in-memory bridge. Configs are kilobytes; anything
// this large is a mistake or an attack.
const maxInputBytes = 4 << 20

// maxPasswordRetries is how many extra attempts a rejected passphrase gets
// before the job is marked failed.
const maxPasswordRetries = 2
