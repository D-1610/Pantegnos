package mobile

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/pbkdf2"
)

// fakeHost records what the application asked the platform to do.
type fakeHost struct {
	clipboard  string
	prefs      string
	clipCopied string
	shared     []string
	saved      []string
	external   []string
	haptics    int
	dark       bool
}

func (h *fakeHost) PickFiles() []PickedFile { return nil }
func (h *fakeHost) ReadClipboard() string   { return h.clipboard }
func (h *fakeHost) Copy(text string) bool   { h.clipCopied = text; return true }
func (h *fakeHost) Share(name, text string) {
	h.shared = append(h.shared, name)
}
func (h *fakeHost) Save(name, text string)     { h.saved = append(h.saved, name) }
func (h *fakeHost) OpenExternal(target string) { h.external = append(h.external, target) }
func (h *fakeHost) LoadPrefs() string          { return h.prefs }
func (h *fakeHost) SavePrefs(raw string)       { h.prefs = raw }
func (h *fakeHost) SetDark(dark bool)          { h.dark = dark }
func (h *fakeHost) Haptic()                    { h.haptics++ }

// slipProfile is a realistic plaintext SlipNet v20 payload.
const slipProfile = "20|ssh|Tokyo Edge|vpn.example.com|1.1.1.1,8.8.8.8|password|30|JP|443|" +
	"edge.example.com|1||socksuser|s3cr3t-socks-pw|1|root|sup3rs3cret|22|0|10.0.0.5|1|" +
	"https://dns.example.com/dns-query|tls|password|||||0||||0||||0|||1232"

func slipFile(t *testing.T, name string) []byte {
	t.Helper()
	return []byte("slipnet://" + base64.StdEncoding.EncodeToString([]byte(slipProfile)))
}

// waitFor polls until cond holds or the test times out. The worker is a real
// goroutine, so the assertions have to give it a moment.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func (a *App) jobByName(name string) *Job {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, j := range a.jobs {
		if j.Name == name {
			return j
		}
	}
	return nil
}

func TestAppDecryptsPlaintextProfile(t *testing.T) {
	host := &fakeHost{}
	app := New(host)
	app.Add("my-vpn.slip", slipFile(t, "my-vpn.slip"))

	waitFor(t, "the job to finish", func() bool {
		job := app.jobByName("my-vpn.slip")
		return job != nil && job.done()
	})

	job := app.jobByName("my-vpn.slip")
	if job.Phase != PhaseDone {
		t.Fatalf("phase = %v, want done (err %q)", job.Phase, job.Err)
	}
	if !strings.Contains(job.Module, "SlipNet") {
		t.Errorf("module = %q, want the SlipNet module", job.Module)
	}
	if !strings.Contains(job.Text, "Detected Profile Version: 20") {
		t.Errorf("text was not parsed:\n%s", job.Text)
	}
	if !job.Expanded {
		t.Error("a freshly decrypted result should be expanded")
	}
}

func TestAppRejectsGarbage(t *testing.T) {
	host := &fakeHost{}
	app := New(host)
	app.Add("notes.txt", []byte("this is not a config at all"))

	waitFor(t, "the job to fail", func() bool {
		job := app.jobByName("notes.txt")
		return job != nil && job.done()
	})

	job := app.jobByName("notes.txt")
	if job.Phase != PhaseFailed {
		t.Fatalf("phase = %v, want failed", job.Phase)
	}
	if job.Err == "" {
		t.Error("a failure must carry a reason the user can act on")
	}
}

func TestAppRejectsOversizedInput(t *testing.T) {
	host := &fakeHost{}
	app := New(host)
	app.Add("huge.slip", make([]byte, maxInputBytes+1))

	waitFor(t, "the job to fail", func() bool {
		job := app.jobByName("huge.slip")
		return job != nil && job.done()
	})
	if job := app.jobByName("huge.slip"); job.Phase != PhaseFailed {
		t.Fatalf("phase = %v, want failed", job.Phase)
	}
}

func TestAppPasteRejectsForeignClipboard(t *testing.T) {
	host := &fakeHost{clipboard: "https://example.com"}
	app := New(host)
	app.Paste()

	if len(app.Snapshot().Jobs) != 0 {
		t.Error("a foreign clipboard must not enqueue anything")
	}
	if got := app.drainNotices(); len(got) != 1 || got[0] != "paste_unsupported" {
		t.Errorf("notices = %v, want a single paste_unsupported", got)
	}
}

func TestAppPasteAcceptsConfigURI(t *testing.T) {
	host := &fakeHost{clipboard: "slipnet://" + base64.StdEncoding.EncodeToString([]byte(slipProfile))}
	app := New(host)
	app.Paste()

	waitFor(t, "the pasted job to finish", func() bool {
		job := app.jobByName("clipboard.slip")
		return job != nil && job.done()
	})
	job := app.jobByName("clipboard.slip")
	if job.Phase != PhaseDone {
		t.Fatalf("phase = %v (%s)", job.Phase, job.Err)
	}
	if !job.FromClip {
		t.Error("a pasted job should be flagged as such")
	}
}

// TestAppPasswordFlow is the whole passphrase journey: prompt, reject, accept.
func TestAppPasswordFlow(t *testing.T) {
	const password = "hunter2"
	payload := buildBundle(t, password, slipProfile)

	host := &fakeHost{}
	app := New(host)
	app.Add("locked.slip", payload)

	waitFor(t, "the password prompt", func() bool {
		snap := app.Snapshot()
		return snap.PasswordJob != nil
	})

	if snap := app.Snapshot(); snap.PasswordJob.Name != "locked.slip" {
		t.Fatalf("prompt is for %q", snap.PasswordJob.Name)
	}
	if err := app.Snapshot().PasswordErr; err != "" {
		t.Errorf("first prompt should not show an error, got %q", err)
	}

	// A wrong password re-prompts with the reason attached.
	app.SubmitPassword("wrong-password")
	waitFor(t, "the retry prompt", func() bool {
		snap := app.Snapshot()
		return snap.PasswordJob != nil && snap.PasswordErr != ""
	})
	if got := app.Snapshot().PasswordErr; !strings.Contains(got, "password") {
		t.Errorf("error = %q, want it to mention the password", got)
	}

	// The right password decrypts.
	app.SubmitPassword(password)
	waitFor(t, "the job to finish", func() bool {
		job := app.jobByName("locked.slip")
		return job != nil && job.done()
	})
	job := app.jobByName("locked.slip")
	if job.Phase != PhaseDone {
		t.Fatalf("phase = %v (%s)", job.Phase, job.Err)
	}
	if !strings.Contains(job.Text, "20|ssh|Tokyo Edge") {
		t.Errorf("payload = %q", job.Text)
	}
}

func TestAppCancelPasswordParksTheJob(t *testing.T) {
	host := &fakeHost{}
	app := New(host)
	app.Add("locked.slip", buildBundle(t, "hunter2", slipProfile))

	waitFor(t, "the password prompt", func() bool { return app.Snapshot().PasswordJob != nil })
	app.CancelPassword()

	waitFor(t, "the job to park", func() bool {
		job := app.jobByName("locked.slip")
		return job != nil && job.Phase == PhaseWaitPassword
	})
	if snap := app.Snapshot(); snap.PasswordJob != nil {
		t.Error("cancelling must close the prompt")
	}
}

func TestAppActionsDriveTheHost(t *testing.T) {
	host := &fakeHost{}
	app := New(host)
	app.Add("my-vpn.slip", slipFile(t, "my-vpn.slip"))
	waitFor(t, "the job to finish", func() bool {
		job := app.jobByName("my-vpn.slip")
		return job != nil && job.done()
	})
	id := app.jobByName("my-vpn.slip").ID

	app.Copy(id)
	if host.clipCopied == "" {
		t.Error("copy did not reach the host")
	}
	if got := app.drainNotices(); len(got) == 0 || got[0] != "msg_copied" {
		t.Errorf("notices = %v", got)
	}

	app.Share(id)
	app.Save(id)
	if len(host.shared) != 1 || host.shared[0] != "my-vpn.txt" {
		t.Errorf("shared = %v, want my-vpn.txt", host.shared)
	}
	if len(host.saved) != 1 || host.saved[0] != "my-vpn.txt" {
		t.Errorf("saved = %v, want my-vpn.txt", host.saved)
	}
}

func TestAppRemoveAndClear(t *testing.T) {
	host := &fakeHost{}
	app := New(host)
	app.Add("a.slip", slipFile(t, "a.slip"))
	app.Add("b.slip", slipFile(t, "b.slip"))
	waitFor(t, "both jobs", func() bool { return len(app.Snapshot().Jobs) == 2 })

	app.Remove(app.jobByName("a.slip").ID)
	if jobs := app.Snapshot().Jobs; len(jobs) != 1 || jobs[0].Name != "b.slip" {
		t.Errorf("after remove: %v", names(jobs))
	}

	app.Clear()
	if jobs := app.Snapshot().Jobs; len(jobs) != 0 {
		t.Errorf("after clear: %v", names(jobs))
	}
}

func TestAppPersistsSettings(t *testing.T) {
	host := &fakeHost{prefs: `{"theme":"dark","locale":"fa"}`}
	app := New(host)
	if app.Snapshot().Settings.Theme != "dark" {
		t.Errorf("theme = %q, want dark", app.Snapshot().Settings.Theme)
	}

	app.SetTheme("light")
	if !strings.Contains(host.prefs, `"theme":"light"`) {
		t.Errorf("prefs = %q", host.prefs)
	}

	// A second instance sees the change, which is what the host relies on.
	next := New(host)
	if next.Snapshot().Settings.Theme != "light" {
		t.Errorf("restored theme = %q", next.Snapshot().Settings.Theme)
	}
}

func TestAppRejectsUnknownSettings(t *testing.T) {
	host := &fakeHost{}
	app := New(host)
	app.SetTheme("neon")
	app.SetLocale("de")
	if s := app.Snapshot().Settings; s.Theme != "system" || s.Locale != "en" {
		t.Errorf("settings = %+v, want the defaults", s)
	}
}

func TestToggleThemeFlipsResolvedAppearance(t *testing.T) {
	host := &fakeHost{}
	systemPrefersDark = func() bool { return true }
	defer func() { systemPrefersDark = func() bool { return true } }()

	app := New(host)
	app.ToggleTheme()
	if got := app.Snapshot().Settings.Theme; got != "light" {
		t.Errorf("from system-dark, theme = %q, want light", got)
	}
	app.ToggleTheme()
	if got := app.Snapshot().Settings.Theme; got != "dark" {
		t.Errorf("theme = %q, want dark", got)
	}
}

func TestRenderProducesMarkup(t *testing.T) {
	host := &fakeHost{}
	app := New(host)
	app.Add("my-vpn.slip", slipFile(t, "my-vpn.slip"))
	waitFor(t, "the job to finish", func() bool {
		job := app.jobByName("my-vpn.slip")
		return job != nil && job.done()
	})

	markup := render(app.Snapshot(), "1.2.3")
	for _, want := range []string{
		"PANTEGNOS",
		`data-theme="system"`,
		"my-vpn.slip",
		"SLIP",
		"Decrypted", // the translated status, not the key
		"Copy",
		"Wrap",
		".npvs",
	} {
		if !strings.Contains(markup, want) {
			t.Errorf("markup is missing %q", want)
		}
	}
	// The viewer shows the payload the core produced, untouched.
	if !strings.Contains(markup, "sup3rs3cret") {
		t.Error("the decrypted payload should be shown verbatim")
	}
	// Masking is gone entirely, so no trace of it should remain in the markup.
	for _, gone := range []string{"mask", "Mask"} {
		if strings.Contains(markup, gone) {
			t.Errorf("markup still mentions %q", gone)
		}
	}
}

func TestVersionReachesTheMarkup(t *testing.T) {
	defer SetLocale("en")
	app := New(&fakeHost{})

	// A build with no version must never render an empty badge.
	for _, raw := range []string{"", "   ", "null", "{{.Version}}"} {
		markup := render(app.Snapshot(), raw)
		if !strings.Contains(markup, "dev") {
			t.Errorf("version %q did not fall back to dev", raw)
		}
		if strings.Contains(markup, ">v<") {
			t.Errorf("version %q rendered an empty badge", raw)
		}
	}

	// A real version is prefixed once, not twice.
	markup := render(app.Snapshot(), "1.4.2")
	if !strings.Contains(markup, "v1.4.2") {
		t.Error("a real version did not reach the markup")
	}
	if strings.Contains(markup, "vv1.4.2") {
		t.Error("the version was prefixed twice")
	}
}

func TestRenderEscapesHostileText(t *testing.T) {
	hostile := `<img src=x onerror=alert(1)>.slip`
	host := &fakeHost{}
	app := New(host)
	app.Add(hostile, slipFile(t, "x"))
	waitFor(t, "the job to finish", func() bool {
		job := app.jobByName(hostile)
		return job != nil && job.done()
	})
	markup := render(app.Snapshot(), "dev")
	if strings.Contains(markup, "<img src=x") {
		t.Error("markup did not escape a hostile file name")
	}
	if !strings.Contains(markup, "&lt;img") {
		t.Error("expected the escaped form to be present")
	}
}

func TestLocalesDefineTheSameKeys(t *testing.T) {
	keys := localeKeys()
	en, okEn := keys["en"]
	fa, okFa := keys["fa"]
	if !okEn || !okFa {
		t.Fatalf("locales loaded: %v", keys)
	}
	if len(en) != len(fa) {
		t.Fatalf("en has %d keys, fa has %d", len(en), len(fa))
	}
	for i := range en {
		if en[i] != fa[i] {
			t.Errorf("key %d differs: en=%q fa=%q", i, en[i], fa[i])
		}
	}
}

func TestLocaleSwitchesText(t *testing.T) {
	defer SetLocale("en")
	SetLocale("en")
	if IsRTL() {
		t.Error("english must not be RTL")
	}
	SetLocale("fa")
	if !IsRTL() {
		t.Error("persian must be RTL")
	}
	if got := t2("drop_title"); got == "Drop a config file here" {
		t.Error("persian catalogue was not used")
	}
}

func t2(key string) string { return t(key) }

func names(jobs []*Job) []string {
	out := make([]string, len(jobs))
	for i, j := range jobs {
		out[i] = j.Name
	}
	return out
}

// buildBundle produces a real `slipnet-bundle-enc://` payload, mirroring the
// reader in internal/modules/impl/slipnet.go.
func buildBundle(t *testing.T, password, plaintext string) []byte {
	t.Helper()
	const (
		version    = 0x01
		saltLength = 16
		ivLength   = 12
		iterations = 600000
		keySize    = 32
	)
	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		t.Fatalf("salt: %v", err)
	}
	iv := make([]byte, ivLength)
	if _, err := rand.Read(iv); err != nil {
		t.Fatalf("iv: %v", err)
	}
	key := pbkdf2.Key([]byte(password), salt, iterations, keySize, sha256.New)
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("cipher: %v", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("gcm: %v", err)
	}
	blob := []byte{version}
	blob = append(blob, salt...)
	blob = append(blob, iv...)
	blob = gcm.Seal(blob, iv, []byte(plaintext), nil)
	return []byte("slipnet-bundle-enc://" + base64.StdEncoding.EncodeToString(blob))
}
