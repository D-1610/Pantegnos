package impl

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const fixturePayload = "kZAuj0qpR40HB9rqYikTNboOboLX2f7O7blLu+1XYhPoBLOV1iQdNdmarvFUH2f7gv9W3CCiF8Uo2lcv3kUIvtt3tusLMKsvgztn33wHgbSVHQCfHE3/3dr6EcpW572/ymcTh4QPV8OYKmC8MnFIVGmosyvrH9eiNOEZVdWY5cOeg3Wbcwumm5H8UT7cjF/FYKpMdOw9RPCW/H1uhDybINIMWqY3cAqseUgR2RZ8JuL08DHUFqIqVfJKq0gfrUii/Soica53N4w9XfRBlRlX4FfhUBAg1d44a3R2FBH7y1mKsRk5KQENja1srwYhctDRlmEEnBXhwrOGN4o8d0UeWmRdTq5XJUJOiYznTxeJ4XMtewVdldkR3Aoo7ZssVvqNfuaPkyO/yTYxpj3vpQgB4JVuKKEP/+5FS9fH9Ooqc222hykeHPE5uEd5wKf5pyckCmw2A5OxAZuLv8fgSQT70dZMhr4WwypJ9LKLLWHLsoK15x/ZrcGNNn9U3rEkbHq5wJ0WRs3zhfO4mFVR7ABnbERr6FOdqRdl0p+:::::"

const fixturePlaintext = "506:3:1:true:ZGVtbw:ZGVtby1wYXNz:c3NoLmV4YW1wbGUudGVzdA:VCAyMg::MjI::true:R0VUIC8gSFRUUC8xLjFbY3JsZl1Ib3N0OiBbaG9zdF0:false::false::true:cHJveHkuZXhhbXBsZS50ZXN0:ODA4MA:false:true:true:true::false:false:cm91bmQgdHJpcCBzYW1wbGU:false::::::::::::::::::::::::::::::"

func tlsHasCommentLine(text string) bool {
	for line := range strings.Lines(text) {
		if strings.HasPrefix(strings.TrimSpace(line), "//") {
			return true
		}
	}
	return false
}

func TestTLSConfigKeyLiterals(t *testing.T) {
	keys := tlsConfigKeys()
	if len(keys) != len(tlsConfigKeyLiterals) {
		t.Fatalf("keys = %d, want %d", len(keys), len(tlsConfigKeyLiterals))
	}

	seen := make(map[string]struct{}, len(keys))
	for i, key := range keys {
		if len(key) != 32 {
			t.Fatalf("key %d is %d bytes, want 32", i, len(key))
		}
		if _, dup := seen[string(key)]; dup {
			t.Fatalf("key %d duplicates an earlier key", i)
		}
		seen[string(key)] = struct{}{}
	}

	for i, literal := range tlsConfigKeyLiterals {
		for _, r := range literal {
			if r > 0xFFFF {
				t.Fatalf("key %d literal holds a non-BMP rune", i)
			}
		}
	}
}

func TestTLSConfigKnownAnswer(t *testing.T) {
	plaintext, keyIndex, err := DecryptTLSConfig([]byte(fixturePayload))
	if err != nil {
		t.Fatalf("DecryptTLSConfig: %v", err)
	}
	if keyIndex != 0 {
		t.Errorf("key index = %d, want 0", keyIndex)
	}
	if string(plaintext) != fixturePlaintext {
		t.Fatalf("plaintext mismatch:\n got %q\nwant %q", plaintext, fixturePlaintext)
	}

	cfg, err := ParseTLSConfig(plaintext)
	if err != nil {
		t.Fatalf("ParseTLSConfig: %v", err)
	}
	if cfg.Tag != tlsTagV4 {
		t.Errorf("tag = %d, want %d", cfg.Tag, tlsTagV4)
	}
	if !cfg.Bool("useProxy") {
		t.Error("useProxy = false, want true")
	}
	for name, want := range map[string]string{
		"sshUser":   "demo",
		"sshHost":   "ssh.example.test",
		"proxyHost": "proxy.example.test",
		"proxyPort": "8080",
		"payload":   "GET / HTTP/1.1[crlf]Host: [host]",
		"message":   "round trip sample",
	} {
		if got := cfg.Value(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}

	rendered := cfg.Format(keyIndex)
	if tlsHasCommentLine(rendered) {
		t.Errorf("rendered profile carries comment syntax:\n%s", rendered)
	}
	for _, unwanted := range []string{"(empty)", "❌ FALSE"} {
		if strings.Contains(rendered, unwanted) {
			t.Errorf("rendered profile keeps %q:\n%s", unwanted, rendered)
		}
	}
	if got := strings.Count(rendered, "Private Server"); got != 1 {
		t.Errorf("Private Server rendered %d times, want only in the Connection row:\n%s", got, rendered)
	}
	for _, want := range []string{
		"Connection                     | Private Server (SSH)",
		"Port                           | T 80 (1)",
	} {
		if !strings.Contains(rendered, want) {
			t.Errorf("rendered profile misses %q:\n%s", want, rendered)
		}
	}
	for _, want := range []string{
		"proxy.example.test",
		"8080",
		"demo",
		"round trip sample",
		"✅ TRUE",
		"GET / HTTP/1.1[crlf]Host: [host]",
	} {
		if !strings.Contains(rendered, want) {
			t.Errorf("rendered profile misses %q:\n%s", want, rendered)
		}
	}
	for _, encoded := range []string{"ZGVtbw", "cHJveHkuZXhhbXBsZS50ZXN0", "ODA4MA", fixturePlaintext} {
		if strings.Contains(rendered, encoded) {
			t.Errorf("rendered profile leaks base64 %q:\n%s", encoded, rendered)
		}
	}
}

func TestTLSConfigFormatFieldsForUnknownTag(t *testing.T) {
	fields := strings.Split(fixturePlaintext, ":")
	fields[0] = "371"

	encrypted, err := EncryptTLSConfig([]byte(strings.Join(fields, ":")), 4)
	if err != nil {
		t.Fatalf("EncryptTLSConfig: %v", err)
	}
	plaintext, keyIndex, err := DecryptTLSConfig(encrypted)
	if err != nil {
		t.Fatalf("DecryptTLSConfig: %v", err)
	}
	cfg, err := ParseTLSConfig(plaintext)
	if err != nil {
		t.Fatalf("ParseTLSConfig: %v", err)
	}

	rendered := cfg.Format(keyIndex)
	if !strings.Contains(rendered, "Field 18") {
		t.Errorf("undocumented tag should fall back to positional labels:\n%s", rendered)
	}
	if strings.Contains(rendered, "Proxy host") {
		t.Errorf("undocumented tag should not borrow documented labels:\n%s", rendered)
	}
}

func TestTLSConfigPortSelection(t *testing.T) {
	for index, want := range tlsPortSelections {
		got := tlsPortSelection(strconv.Itoa(index))
		if !strings.HasPrefix(got, want) {
			t.Errorf("port %d = %q, want prefix %q", index, got, want)
		}
	}
	for _, raw := range []string{"", "9", "-1", "AUTO", "T 80"} {
		if got := tlsPortSelection(raw); got != raw {
			t.Errorf("port %q = %q, want it unchanged", raw, got)
		}
	}
}

func TestTLSConfigRoundTripEveryKey(t *testing.T) {
	plaintext := []byte(fixturePlaintext)

	for index := range tlsConfigKeys() {
		encrypted, err := EncryptTLSConfig(plaintext, index)
		if err != nil {
			t.Fatalf("key %d: EncryptTLSConfig: %v", index, err)
		}
		if !bytes.HasSuffix(encrypted, []byte(TLSConfigTrailer)) {
			t.Fatalf("key %d: payload does not end with %q", index, TLSConfigTrailer)
		}

		got, keyIndex, err := DecryptTLSConfig(encrypted)
		if err != nil {
			t.Fatalf("key %d: DecryptTLSConfig: %v", index, err)
		}
		if keyIndex != index {
			t.Errorf("key %d: reported index %d", index, keyIndex)
		}
		if !bytes.Equal(got, plaintext) {
			t.Fatalf("key %d: round trip mismatch", index)
		}
	}
}

func TestTLSConfigAcceptsChatNoise(t *testing.T) {
	encrypted, err := EncryptTLSConfig([]byte(fixturePlaintext), 0)
	if err != nil {
		t.Fatalf("EncryptTLSConfig: %v", err)
	}

	variants := map[string][]byte{
		"trailer":            encrypted,
		"no trailer":         bytes.TrimSuffix(encrypted, []byte(TLSConfigTrailer)),
		"wrapped in spaces":  []byte("  " + string(encrypted) + "\n"),
		"windows line break": []byte(string(encrypted) + "\r\n"),
	}
	for name, payload := range variants {
		got, _, err := DecryptTLSConfig(payload)
		if err != nil {
			t.Errorf("%s: DecryptTLSConfig: %v", name, err)
			continue
		}
		if string(got) != fixturePlaintext {
			t.Errorf("%s: plaintext mismatch", name)
		}
	}
}

func TestTLSConfigRejectsWrongPayload(t *testing.T) {
	encrypted, err := EncryptTLSConfig([]byte(fixturePlaintext), 0)
	if err != nil {
		t.Fatalf("EncryptTLSConfig: %v", err)
	}

	tampered := bytes.Clone(encrypted)
	mid := len(tampered) / 2
	if tampered[mid] == 'A' {
		tampered[mid] = 'B'
	} else {
		tampered[mid] = 'A'
	}
	if _, _, err := DecryptTLSConfig(tampered); err == nil {
		t.Error("tampered ciphertext decrypted without error")
	}
	if _, _, err := DecryptTLSConfig([]byte("not a config at all")); err == nil {
		t.Error("garbage decrypted without error")
	}
	if _, _, err := DecryptTLSConfig(nil); err == nil {
		t.Error("empty payload decrypted without error")
	}
}

func TestTLSConfigIgnoresUnauthenticatedFiller(t *testing.T) {
	encrypted, err := EncryptTLSConfig([]byte(fixturePlaintext), 0)
	if err != nil {
		t.Fatalf("EncryptTLSConfig: %v", err)
	}

	tampered := bytes.Clone(encrypted)
	if tampered[20] == 'A' {
		tampered[20] = 'B'
	} else {
		tampered[20] = 'A'
	}

	got, _, err := DecryptTLSConfig(tampered)
	if err != nil {
		t.Fatalf("filler byte changed decryption: %v", err)
	}
	if string(got) != fixturePlaintext {
		t.Fatal("plaintext mismatch after changing a filler byte")
	}
}

func TestParseTLSConfigNeedsFullLayout(t *testing.T) {
	if _, err := ParseTLSConfig([]byte("506:0:0:true")); err == nil {
		t.Error("short field list parsed without error")
	}
}

func TestTLSConfigRealSamples(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "configs")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("no sample directory: %v", err)
	}

	found := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".tls") {
			continue
		}
		found++

		raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatalf("%s: read: %v", entry.Name(), err)
		}
		plaintext, keyIndex, err := DecryptTLSConfig(raw)
		if err != nil {
			t.Errorf("%s: DecryptTLSConfig: %v", entry.Name(), err)
			continue
		}
		cfg, err := ParseTLSConfig(plaintext)
		if err != nil {
			t.Errorf("%s: ParseTLSConfig: %v", entry.Name(), err)
			continue
		}
		if cfg.Tag <= 0 {
			t.Errorf("%s: tag = %d, want a positive config tag", entry.Name(), cfg.Tag)
		}
		if keyIndex < 0 || keyIndex >= len(tlsConfigKeyLiterals) {
			t.Errorf("%s: key index %d out of range", entry.Name(), keyIndex)
		}

		if cfg.Tag != tlsTagV4 && cfg.Tag != tlsTagV5 {
			t.Logf("%s: tag %d has no documented field layout, only decoding is checked", entry.Name(), cfg.Tag)
		}

		rendered := cfg.Format(keyIndex)
		if tlsHasCommentLine(rendered) {
			t.Errorf("%s: rendered profile carries comment syntax", entry.Name())
		}
		for i, spec := range tlsConfigFields {
			if spec.kind != tlsFieldText {
				continue
			}
			encoded := strings.TrimSpace(cfg.Fields[i])
			decoded := cfg.Value(spec.name)
			if encoded == "" || decoded == encoded {
				continue
			}
			if strings.Contains(rendered, encoded) {
				t.Errorf("%s: %s still rendered as base64", entry.Name(), spec.label)
			}
			first, _, _ := strings.Cut(decoded, "\n")
			if first = strings.TrimSpace(first); first != "" && !strings.Contains(rendered, first) {
				t.Errorf("%s: %s missing its decoded value", entry.Name(), spec.label)
			}
		}
		t.Logf("%s: tag=%d key=#%d fields=%d ssh=%s proxy=%s sni=%s",
			entry.Name(), cfg.Tag, keyIndex, len(cfg.Fields),
			cfg.Value("sshHost"), cfg.Value("proxyHost"), cfg.Value("sniHost"))
	}
	if found == 0 {
		t.Skip("no .tls samples in configs/")
	}
}
