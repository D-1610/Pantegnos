package impl

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"Pantegnos/internal/modules"
)

// bpfBuild assembles a container the way the sing-box clients do, so the
// parser is tested against the real wire layout rather than a mock of it.
func bpfBuild(t *testing.T, version int, p bpfProfile) []byte {
	t.Helper()

	var body bytes.Buffer
	writeUvarint := func(v uint64) {
		for v >= 0x80 {
			body.WriteByte(byte(v) | 0x80)
			v >>= 7
		}
		body.WriteByte(byte(v))
	}
	writeString := func(s string) {
		writeUvarint(uint64(len(s)))
		body.WriteString(s)
	}
	writeInt32 := func(v int32) {
		var b [4]byte
		binary.BigEndian.PutUint32(b[:], uint32(v))
		body.Write(b[:])
	}
	writeInt64 := func(v int64) {
		var b [8]byte
		binary.BigEndian.PutUint64(b[:], uint64(v))
		body.Write(b[:])
	}

	writeString(p.Name)
	writeInt32(p.Type)
	writeString(p.Config)
	if p.Type == bpfProfileRemote {
		writeString(p.RemotePath)
		if p.AutoUpdate {
			body.WriteByte(1)
		} else {
			body.WriteByte(0)
		}
		if version >= 1 {
			writeInt32(p.AutoUpdateMinutes)
		}
		writeInt64(p.LastUpdated)
	}

	var zipped bytes.Buffer
	zw := gzip.NewWriter(&zipped)
	if _, err := zw.Write(body.Bytes()); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}

	out := append([]byte{bpfMsgProfileContent, byte(version)}, zipped.Bytes()...)
	return out
}

const bpfSampleConfig = `{"outbounds":[{"type":"vless","tag":"de","server":"198.51.100.7","server_port":443,` +
	`"uuid":"00000000-0000-4000-8000-000000000000","tls":{"enabled":true,"server_name":"cdn.example.com",` +
	`"reality":{"enabled":true,"public_key":"X_demo","short_id":"0123abcd"}}}]}`

func TestParseBPFLocal(t *testing.T) {
	raw := bpfBuild(t, bpfVersionCurrent, bpfProfile{
		Name:   "\U0001f3a8@oneclickvpnkeys",
		Type:   bpfProfileLocal,
		Config: bpfSampleConfig,
	})

	got, err := parseBPF(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.Name != "\U0001f3a8@oneclickvpnkeys" {
		t.Errorf("name = %q", got.Name)
	}
	if got.Type != bpfProfileLocal {
		t.Errorf("type = %d, want %d", got.Type, bpfProfileLocal)
	}
	if got.Config != bpfSampleConfig {
		t.Errorf("config = %q", got.Config)
	}
}

func TestParseBPFRemoteV1(t *testing.T) {
	raw := bpfBuild(t, bpfVersionCurrent, bpfProfile{
		Name:              "Subscription",
		Type:              bpfProfileRemote,
		Config:            bpfSampleConfig,
		RemotePath:        "https://example.com/sub.json",
		AutoUpdate:        true,
		AutoUpdateMinutes: 60,
		LastUpdated:       1767225600000,
	})

	got, err := parseBPF(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.RemotePath != "https://example.com/sub.json" {
		t.Errorf("remotePath = %q", got.RemotePath)
	}
	if !got.AutoUpdate || got.AutoUpdateMinutes != 60 {
		t.Errorf("autoUpdate = %v every %d min", got.AutoUpdate, got.AutoUpdateMinutes)
	}
	if got.LastUpdated != 1767225600000 {
		t.Errorf("lastUpdated = %d", got.LastUpdated)
	}
}

// Version 0 remote profiles carry the flag and the timestamp but no interval,
// so the reader has to key the interval off the version.
func TestParseBPFRemoteV0SkipsInterval(t *testing.T) {
	raw := bpfBuild(t, 0, bpfProfile{
		Name:       "Old",
		Type:       bpfProfileRemote,
		Config:     bpfSampleConfig,
		RemotePath: "https://example.com/old.json",
		AutoUpdate: true,
	})

	got, err := parseBPF(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !got.AutoUpdate {
		t.Error("autoUpdate = false, want true")
	}
	if got.AutoUpdateMinutes != 0 {
		t.Errorf("autoUpdateMinutes = %d, want 0", got.AutoUpdateMinutes)
	}
	if got.Version != 0 {
		t.Errorf("version = %d, want 0", got.Version)
	}
}

// An empty name and an empty config are legal, and the renderer has to fall
// back to the file stem rather than print a blank.
func TestDecryptBPFFallsBackToFileStem(t *testing.T) {
	raw := bpfBuild(t, bpfVersionCurrent, bpfProfile{Type: bpfProfileLocal})

	res, err := decryptBPF(modules.Request{FileName: "dir/profile.bpf", Data: raw})
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if res.FileName != "profile.txt" {
		t.Errorf("fileName = %q, want profile.txt", res.FileName)
	}
	if !strings.Contains(res.Text, "name: profile") {
		t.Errorf("text does not fall back to the stem:\n%s", res.Text)
	}
	if !strings.Contains(res.Text, "carries no config") {
		t.Errorf("empty config not reported:\n%s", res.Text)
	}
}

func TestDecryptBPFRendersConfigAndMetadata(t *testing.T) {
	raw := bpfBuild(t, bpfVersionCurrent, bpfProfile{
		Name:              "Sub",
		Type:              bpfProfileRemote,
		Config:            bpfSampleConfig,
		RemotePath:        "https://example.com/sub.json",
		AutoUpdate:        true,
		AutoUpdateMinutes: 30,
		LastUpdated:       1767225600000,
	})

	res, err := decryptBPF(modules.Request{FileName: "sub.bpf", Data: raw})
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	for _, want := range []string{
		"// type: remote (container version 1)",
		"// source: https://example.com/sub.json",
		"// auto-update: every 30 min",
		`"server": "198.51.100.7"`,
	} {
		if !strings.Contains(res.Text, want) {
			t.Errorf("output is missing %q:\n%s", want, res.Text)
		}
	}
	// The header has to stay out of the JSON body so the file stays importable.
	header, body, found := strings.Cut(res.Text, "\n\n")
	if !found || !strings.HasPrefix(header, "// ---- sing-box profile ----") {
		t.Fatalf("unexpected header layout:\n%s", res.Text)
	}
	if !strings.HasPrefix(strings.TrimSpace(body), "{") || !strings.HasSuffix(strings.TrimSpace(body), "}") {
		t.Errorf("config body is not a bare JSON object:\n%s", body)
	}
}

func TestParseBPFRejects(t *testing.T) {
	valid := bpfBuild(t, bpfVersionCurrent, bpfProfile{
		Name:   "ok",
		Type:   bpfProfileLocal,
		Config: bpfSampleConfig,
	})

	// Every case must be a clean error, never a panic or a silent success.
	cases := []struct {
		name string
		raw  []byte
		want string
	}{
		{"empty", nil, "too small"},
		{"one byte", valid[:1], "too small"},
		{"wrong message type", append([]byte{0x04, 0x01}, valid[2:]...), "message type 0x04"},
		{"future version", append([]byte{bpfMsgProfileContent, 0x09}, valid[2:]...), "newer than the supported version"},
		{"not gzip", []byte{bpfMsgProfileContent, 0x01, 0x00, 0x01, 0x02}, "not a gzip stream"},
		{"truncated gzip", valid[:len(valid)/2], "inflate failed"},
		{"gzip bomb", bpfBomb(t), "exceeds the"},
		{"truncated body", bpfBodyWithout(t, valid, 4), "truncated"},
		{"varint too long", bpfUvarintOverflow(t), "overflows 64 bits"},
		{"absurd length", bpfAbsurdLength(t), "exceeds the"},
		{"unknown type", bpfWithType(t, 7), "unknown profile type 7"},
		{"icloud", bpfWithType(t, bpfProfileICloud), "iCloud profile"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseBPF(tt.raw)
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want it to mention %q", err, tt.want)
			}
		})
	}
}

func bpfGzip(t *testing.T, payload []byte) []byte {
	t.Helper()
	var out bytes.Buffer
	zw := gzip.NewWriter(&out)
	if _, err := zw.Write(payload); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return out.Bytes()
}

// bpfBomb deflates a payload larger than the cap - all zeroes, so the container
// is tiny and the reader has to refuse it on the way out rather than after
// buffering it.
func bpfBomb(t *testing.T) []byte {
	t.Helper()
	huge := make([]byte, bpfMaxInflated+1024)
	return append([]byte{bpfMsgProfileContent, bpfVersionCurrent}, bpfGzip(t, huge)...)
}

// bpfBodyWithout chops n bytes off the inflated container.
func bpfBodyWithout(t *testing.T, full []byte, n int) []byte {
	t.Helper()
	zr, err := gzip.NewReader(bytes.NewReader(full[2:]))
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	var body bytes.Buffer
	if _, err := body.ReadFrom(zr); err != nil {
		t.Fatalf("inflate: %v", err)
	}
	return append([]byte{bpfMsgProfileContent, bpfVersionCurrent}, bpfGzip(t, body.Bytes()[:body.Len()-n])...)
}

func bpfUvarintOverflow(t *testing.T) []byte {
	t.Helper()
	// Ten continuation bytes carry more than 64 bits.
	body := bytes.Repeat([]byte{0x80}, 10)
	return append([]byte{bpfMsgProfileContent, bpfVersionCurrent}, bpfGzip(t, body)...)
}

func bpfAbsurdLength(t *testing.T) []byte {
	t.Helper()
	body := []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x7f}
	return append([]byte{bpfMsgProfileContent, bpfVersionCurrent}, bpfGzip(t, body)...)
}

func bpfWithType(t *testing.T, kind int32) []byte {
	t.Helper()

	var body bytes.Buffer
	// name: uvarint 2, "ok"
	body.WriteString("\x02ok")

	var b [4]byte
	binary.BigEndian.PutUint32(b[:], uint32(kind))
	body.Write(b[:])

	// config: uvarint 2, "{}"
	body.WriteString("\x02{}")

	return append([]byte{bpfMsgProfileContent, bpfVersionCurrent}, bpfGzip(t, body.Bytes())...)
}

// The module has to win on the extension, and the whole pipeline has to reach
// it through the shared registry rather than only through a direct call.
func TestBPFModuleIsRegistered(t *testing.T) {
	raw := bpfBuild(t, bpfVersionCurrent, bpfProfile{
		Name:   "\U0001f3a8@oneclickvpnkeys",
		Type:   bpfProfileLocal,
		Config: bpfSampleConfig,
	})

	mod, proto, payload := modules.Lookup("\U0001f3a8@oneclickvpnkeys.bpf", raw)
	if mod == nil || mod.Extension != ".bpf" {
		t.Fatalf("Lookup module = %v, want the .bpf module", mod)
	}
	if proto != "" {
		t.Errorf("proto = %q, want empty for a binary container", proto)
	}
	if payload != string(raw) {
		t.Error("Lookup must hand the raw bytes through untouched")
	}

	res, err := mod.Decrypt(modules.Request{
		FileName: "\U0001f3a8@oneclickvpnkeys.bpf",
		Data:     raw,
		Payload:  payload,
	})
	if err != nil {
		t.Fatalf("decrypt through the registry: %v", err)
	}
	if res.FileName != "\U0001f3a8@oneclickvpnkeys.txt" {
		t.Errorf("fileName = %q", res.FileName)
	}
	if !strings.Contains(res.Text, `"type": "vless"`) {
		t.Errorf("outbound not recovered:\n%s", res.Text)
	}
}

// A real export from the sample directory, when one is present locally. The
// directory is gitignored, so the test is skipped rather than failed in CI.
func TestBPFFixture(t *testing.T) {
	dir := os.Getenv("PAN_BPF_FIXTURES")
	if dir == "" {
		t.Skip("PAN_BPF_FIXTURES not set")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skip(err)
	}

	var checked int
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".bpf" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", entry.Name(), err)
		}

		mod, _, payload := modules.Lookup(entry.Name(), data)
		if mod == nil || mod.Extension != ".bpf" {
			t.Fatalf("%s was not claimed by the .bpf module", entry.Name())
		}
		res, err := mod.Decrypt(modules.Request{FileName: entry.Name(), Data: data, Payload: payload})
		if err != nil {
			t.Fatalf("decrypt %s: %v", entry.Name(), err)
		}
		if !strings.Contains(res.Text, `"outbounds"`) {
			t.Errorf("%s produced no sing-box config:\n%s", entry.Name(), res.Text)
		}
		t.Logf("%s -> %s", entry.Name(), res.FileName)
		checked++
	}

	if checked == 0 {
		t.Skipf("no .bpf sample in %s", dir)
	}
}
