//go:build js && wasm

// This file is the only part of the application that knows it is running in a
// browser. It drives the DOM through syscall/js and forwards the handful of
// platform operations to the Android shell over window.PantegnosHost.

package mobile

import (
	"encoding/json"
	"syscall/js"
)

// Boot wires the application to the page and to the Android shell. It is called
// once, from the shell page, and never returns.
func Boot(version string) {
	host := &jsHost{}
	app := New(host)
	ui := &ui{app: app, version: version}

	// The locale is a device property, so it wins over the stored preference the
	// first time the app runs and the user can override it later.
	app.mu.Lock()
	stored := app.settings.Locale
	app.mu.Unlock()
	if stored == "" {
		SetLocale(host.Locale())
		app.mu.Lock()
		app.settings.Locale = Locale()
		app.mu.Unlock()
		app.syncSystemBars()
	}

	ui.install()
	ui.paint()
}

// ui owns the page: it installs listeners once, then repaints from Go whenever
// the application state changes.
type ui struct {
	app     *App
	version string
	// paintQueued collapses a burst of state changes into a single repaint.
	paintQueued bool
	// caret remembers the password field caret across a repaint.
	caret int
	// pwError is set when the last attempt was rejected, so the freshly rendered
	// field can pick up the error styling without waiting for a keystroke.
	pwError bool
}

// install registers the delegated listeners. Everything is delegated from the
// document so repainting the tree never loses a handler.
func (u *ui) install() {
	document := js.Global().Get("document")

	document.Call("addEventListener", "click", js.FuncOf(u.onClick))
	document.Call("addEventListener", "keydown", js.FuncOf(u.onKey))
	document.Call("addEventListener", "input", js.FuncOf(u.onInput))
	document.Call("addEventListener", "change", js.FuncOf(u.onInput))

	// The style element is filled once; the markup is replaced on every paint.
	style := document.Call("createElement", "style")
	style.Set("textContent", appCSS)
	document.Get("head").Call("appendChild", style)
}

func (u *ui) onClick(_ js.Value, args []js.Value) any {
	if len(args) == 0 {
		return nil
	}
	event := args[0]
	target := event.Get("target")
	for target.Get("tagName").String() != "" {
		// Walking up from the target means the innermost action wins, so a
		// button inside the drop zone does not also trigger the drop zone.
		action := dataset(target, "act")
		if action != "" {
			event.Call("preventDefault")
			u.dispatch(action, dataset(target, "id"), dataset(target, "value"))
			return nil
		}
		target = target.Get("parentElement")
	}
	return nil
}

func dataset(node js.Value, key string) string {
	if node.IsNull() || node.IsUndefined() {
		return ""
	}
	ds := node.Get("dataset")
	if ds.IsUndefined() || ds.IsNull() {
		return ""
	}
	return ds.Get(key).String()
}

func (u *ui) onKey(_ js.Value, args []js.Value) any {
	if len(args) == 0 {
		return nil
	}
	event := args[0]
	if event.Get("key").String() != "Enter" {
		return nil
	}
	switch event.Get("target").Get("id").String() {
	case "pw":
		u.submitPassword()
	case "pw-submit":
		u.submitPassword()
	}
	return nil
}

func (u *ui) onInput(_ js.Value, args []js.Value) any {
	// The submit button is enabled by the presence of a password, and the field
	// border shows the error state. Neither needs a full repaint.
	document := js.Global().Get("document")
	field := document.Call("getElementById", "pw")
	if field.IsNull() {
		return nil
	}
	submit := document.Call("getElementById", "pw-submit")
	if !submit.IsNull() {
		submit.Set("disabled", field.Get("value").String() == "")
	}
	if field.Get("dataset").Get("error").String() == "true" {
		field.Get("parentElement").Get("classList").Call("remove", "has-error")
		field.Get("dataset").Set("error", "false")
	}
	u.pwError = false
	u.rememberCaret()
	return nil
}

// rememberCaret snapshots the password field value and caret so a repaint can
// restore them.
func (u *ui) rememberCaret() {
	document := js.Global().Get("document")
	field := document.Call("getElementById", "pw")
	if field.IsNull() {
		u.caret = -1
		return
	}
	u.caret = int(field.Get("selectionStart").Float())
}

func (u *ui) dispatch(action, id, value string) {
	app := u.app
	switch action {
	case "pick":
		app.Pick()
	case "paste":
		app.Paste()
	case "clear":
		app.Clear()
	case "retry":
		app.Retry(parseID(id))
	case "remove":
		app.Remove(parseID(id))
	case "expand":
		app.ToggleExpand(parseID(id))
	case "wrap":
		app.ToggleWrap(parseID(id))
	case "copy":
		app.Copy(parseID(id))
	case "share":
		app.Share(parseID(id))
	case "save":
		app.Save(parseID(id))
	case "author":
		app.OpenAuthor(parseID(id))
	case "theme":
		app.ToggleTheme()
	case "settings":
		app.OpenSettings()
	case "close-settings":
		app.CloseSettings()
	case "theme-set":
		app.SetTheme(value)
	case "cancel-password":
		app.CancelPassword()
	case "submit-password":
		u.submitPassword()
	case "toggle-pw":
		u.togglePasswordVisibility()
	case "github":
		app.OpenGithub()
	case "website":
		app.OpenWebsite()
	default:
		return
	}
	u.paint()
}

func (u *ui) submitPassword() {
	document := js.Global().Get("document")
	field := document.Call("getElementById", "pw")
	if field.IsNull() {
		return
	}
	u.rememberCaret()
	u.app.SubmitPassword(field.Get("value").String())
	u.paint()
}

func (u *ui) togglePasswordVisibility() {
	document := js.Global().Get("document")
	field := document.Call("getElementById", "pw")
	if field.IsNull() {
		return
	}
	hidden := field.Get("type").String() == "password"
	field.Set("type", "password")
	if hidden {
		field.Set("type", "text")
	}
}

// paint replaces the application markup and shows any pending message.
func (u *ui) paint() {
	if u.paintQueued {
		return
	}
	u.paintQueued = true
	js.Global().Call("setTimeout", js.FuncOf(func(js.Value, []js.Value) any {
		u.paintQueued = false
		u.repaint()
		return nil
	}), 0)
}

func (u *ui) repaint() {
	document := js.Global().Get("document")
	body := document.Get("body")
	if body.IsNull() {
		return
	}

	// Preserve the scroll offset and whatever the user had focused.
	window := js.Global()
	scrollX := window.Get("scrollX").Float()
	scrollY := window.Get("scrollY").Float()

	markup := render(u.app.Snapshot(), u.version)
	body.Set("innerHTML", markup)

	window.Call("scrollTo", scrollX, scrollY)

	// A password prompt survives a repaint with its value and caret intact, so a
	// rejected attempt does not wipe what the user typed.
	if field := document.Call("getElementById", "pw"); !field.IsNull() {
		if caret := u.caret; caret >= 0 {
			field.Call("focus")
			field.Set("selectionStart", caret)
			field.Set("selectionEnd", caret)
		}
		if u.pwError {
			field.Get("parentElement").Get("classList").Call("add", "has-error")
		}
		if submit := document.Call("getElementById", "pw-submit"); !submit.IsNull() {
			submit.Set("disabled", field.Get("value").String() == "")
		}
	}
	u.showNotices()
}

func (u *ui) showNotices() {
	// Notices are drained by Snapshot, so read them straight from the app.
	document := js.Global().Get("document")
	toast := document.Call("getElementById", "toast")
	if toast.IsNull() {
		return
	}
	messages := u.app.drainNotices()
	if len(messages) == 0 {
		toast.Set("hidden", true)
		return
	}
	toast.Set("hidden", false)
	toast.Set("textContent", t(messages[0]))
	js.Global().Call("setTimeout", js.FuncOf(func(js.Value, []js.Value) any {
		if node := js.Global().Get("document").Call("getElementById", "toast"); !node.IsNull() {
			node.Set("hidden", true)
		}
		return nil
	}), 2200)
}

// jsHost forwards platform operations to the Android shell.
type jsHost struct{}

func hostJS() js.Value { return js.Global().Get("PantegnosHost") }

func (h *jsHost) PickFiles() []PickedFile {
	host := hostJS()
	if host.IsUndefined() || host.IsNull() {
		return nil
	}
	raw := host.Call("pickFiles").String()
	if raw == "" {
		return nil
	}
	var files []PickedFile
	if err := json.Unmarshal([]byte(raw), &files); err != nil {
		return nil
	}
	return files
}

func (h *jsHost) ReadClipboard() string {
	host := hostJS()
	if host.IsUndefined() || host.IsNull() {
		return ""
	}
	return host.Call("readClipboard").String()
}

func (h *jsHost) Copy(text string) bool {
	host := hostJS()
	if host.IsUndefined() || host.IsNull() {
		return false
	}
	return host.Call("copyToClipboard", text).Bool()
}

func (h *jsHost) Share(name, text string) {
	if host := hostJS(); !host.IsUndefined() && !host.IsNull() {
		host.Call("share", name, text)
	}
}

func (h *jsHost) Save(name, text string) {
	if host := hostJS(); !host.IsUndefined() && !host.IsNull() {
		host.Call("save", name, text)
	}
}

func (h *jsHost) OpenExternal(target string) {
	if host := hostJS(); !host.IsUndefined() && !host.IsNull() {
		host.Call("openExternal", target)
	}
}

func (h *jsHost) LoadPrefs() string {
	host := hostJS()
	if host.IsUndefined() || host.IsNull() {
		return ""
	}
	return host.Call("loadPrefs").String()
}

func (h *jsHost) SavePrefs(raw string) {
	if host := hostJS(); !host.IsUndefined() && !host.IsNull() {
		host.Call("savePrefs", raw)
	}
}

func (h *jsHost) SetDark(dark bool) {
	if host := hostJS(); !host.IsUndefined() && !host.IsNull() {
		host.Call("setDark", dark)
	}
}

func (h *jsHost) Haptic() {
	if host := hostJS(); !host.IsUndefined() && !host.IsNull() {
		host.Call("haptic")
	}
}

// Locale asks the shell for the device language.
func (h *jsHost) Locale() string {
	host := hostJS()
	if host.IsUndefined() || host.IsNull() {
		return "en"
	}
	return host.Call("locale").String()
}

// systemPrefersDark follows the OS appearance preference.
func init() {
	systemPrefersDark = func() bool {
		query := js.Global().Get("matchMedia")
		if query.Type() != js.TypeFunction {
			return true
		}
		// MediaQueryList.matches is a property, not a method.
		return query.Invoke("(prefers-color-scheme: dark)").Get("matches").Bool()
	}
}
