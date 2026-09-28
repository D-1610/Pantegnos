package impl

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"Pantegnos/internal/modules"
)

const (
	npvGen2FixtureB64 = "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"
	npvGen2FixtureSalt  = "b3e16e38809576cbf6aaafd638b6b070"
	npvGen2FixtureCfgID = "96b7d923079c7bc36d1310bd4d111a79"
	npvGen2FixtureA16   = "ffd6b86349231c14b37f6c929e24cd42"
	npvGen2FixtureKDK   = "52043647a29853e765f8efe370fac47155616bfaf653d61b0075a2f183fb6b6b"
	npvGen2FixtureName  = "Internet Server VPN / WhatsApp"
)

const npvGen2FixturePayloadPrefix = "GET /-Internet-Server-VPN-WhatsApp-"

func npvGen2Fixture(t *testing.T) []byte {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString(npvGen2FixtureB64)
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	return data
}

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("bad hex %q: %v", s, err)
	}
	return b
}

func TestNpvGen2A16Vectors(t *testing.T) {
	for _, tc := range []struct{ salt, want string }{
		{npvGen2FixtureSalt, npvGen2FixtureA16},
		{"000102030405060708090a0b0c0d0e0f", "f659c73d8c0fa5150e3254dfe0a1106d"},
		{"f7c7701d0117e9f1f0ce9508abb11519", "9821914745243955b1bdde3b7952303e"},
		{"c45a3c0407fcc90458a59f7b9d762617", "e34dd0a6557a2a2dd41836e3d83740bc"},
		{"2eb61dd803807ce1f73f10aa8ae97711", "8eb1c4b1fa21daa7e4de7b8e4864f79a"},
		{"00000000000000000000000000000000", "4878126b14231f6f522f310686001524"},
		{"50d72fa987791d011e5d83e3e36ea971", "036ab758c7e097dc77b24b995908628a"},
	} {
		got, err := npvGen2A16(mustHex(t, tc.salt))
		if err != nil {
			t.Fatalf("salt %s: %v", tc.salt, err)
		}
		if hex.EncodeToString(got[:]) != tc.want {
			t.Errorf("salt %s: A16 = %x, want %s", tc.salt, got, tc.want)
		}
	}

	if _, err := npvGen2A16([]byte("short")); err == nil {
		t.Error("a non-16-byte salt must be rejected")
	}
}

func TestNpvGen2KDKVector(t *testing.T) {
	a16, err := npvGen2A16(mustHex(t, npvGen2FixtureSalt))
	if err != nil {
		t.Fatal(err)
	}
	kdk := npvGen2KDK(a16, mustHex(t, npvGen2FixtureCfgID))
	if hex.EncodeToString(kdk[:]) != npvGen2FixtureKDK {
		t.Fatalf("KDK = %x, want %s", kdk, npvGen2FixtureKDK)
	}
}

func TestNpvGen2WhiteboxVectors(t *testing.T) {
	tables, err := loadNpvGen2Tables()
	if err != nil {
		t.Fatal(err)
	}
	block := tables[:npvGen2BlockSize]

	for _, tc := range []struct {
		state string
		i     int
		args  [4]string
		want  string
	}{
		{"b395af7080aab038f6b66ecb38e176d6", 0, [4]string{"b51b86d6", "e47f474c", "e71972b9", "304961f0"}, "8634d2d3"},
		{"8634d2d380aab038f6b66ecb38e176d6", 0, [4]string{"b79e3599", "9deff726", "9f2e1d6d", "452bee07"}, "f07431d5"},
		{"f07431d580aab038f6b66ecb38e176d6", 1, [4]string{"a17fadc8", "dc53fd9a", "d062b207", "a489dfa8"}, "09c73dfd"},
		{"f07431d509c73dfdf6b66ecb38e176d6", 1, [4]string{"84c2785b", "f72828d1", "501b597f", "57c58d75"}, "74348480"},
		{"f07431d574348480f6b66ecb38e176d6", 2, [4]string{"a6cddf17", "ba0bd655", "d24d1207", "5e980c6e"}, "9013172b"},
		{"f07431d5743484809013172b38e176d6", 2, [4]string{"846ea27d", "ee277f7b", "d9a9df4b", "ef410f79"}, "5ca10d34"},
		{"f07431d5743484805ca10d3438e176d6", 3, [4]string{"e7099930", "ee0a772e", "88de5cf1", "4b20a923"}, "cafd1bcc"},
	} {
		state := mustHex(t, tc.state)
		args := [4]uint64{}
		for k, a := range tc.args {
			v, err := strconv.ParseUint(a, 16, 32)
			if err != nil {
				t.Fatalf("bad argument %q: %v", a, err)
			}
			args[k] = v
		}

		if tc.state == "b395af7080aab038f6b66ecb38e176d6" {
			first := npvGen2GroupArgs(block, 0x6000, tc.i, state[0:4])
			for k := range 4 {
				if uint64(first[k]) != args[k] {
					t.Errorf("group args = %08x, want %v", first, tc.args)
				}
			}
		}
		if tc.i == 0 && tc.state == "8634d2d380aab038f6b66ecb38e176d6" {
			second := npvGen2GroupArgs(block, 0xA000, tc.i, state[0:4])
			for k := range 4 {
				if uint64(second[k]) != args[k] {
					t.Errorf("second group args = %08x, want %v", second, tc.args)
				}
			}
		}

		written := npvGen2Sub7DC0(block, tc.i, args[0], args[1], args[2], args[3])
		if hex.EncodeToString(written[:]) != tc.want {
			t.Errorf("call for state %s: wrote %s, want %s", tc.state, hex.EncodeToString(written[:]), tc.want)
		}
	}
}

func TestNpvGen2DecryptsFixtureLinks(t *testing.T) {
	env, err := parseNpvGen2Envelope(npvGen2Fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	metadata, fields, keys, err := env.open("")
	if err != nil {
		t.Fatal(err)
	}

	var meta npvGen2Metadata
	if err := json.Unmarshal(metadata, &meta); err != nil {
		t.Fatalf("metadata is not JSON: %v", err)
	}
	if meta.IssuedAt == "" || meta.Policy.ConfigVersion == 0 {
		t.Errorf("metadata looks empty: %+v", meta)
	}

	links, err := npvGen2Links(fields)
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 {
		t.Fatalf("fixture carries %d configs, want 1", len(links))
	}
	link := links[0]

	for _, want := range []string{
		"ssh://ssh:xxxx@help.snapchat.com:80?",
		"sshConfigType=SSH-Payload",
		"remarks=Internet+Server+VPN+%2F+WhatsApp",
		"#" + npvGen2FixturePayloadPrefix,
		"Host: x.m-90.xyz",
		"[crlf]",
	} {
		if !strings.Contains(link, want) {
			t.Errorf("link does not contain %q: %s", want, link)
		}
	}

	parsed, err := url.Parse(link)
	if err != nil {
		t.Fatalf("link does not parse: %v", err)
	}
	if parsed.Scheme != "ssh" || parsed.Host != "help.snapchat.com:80" || parsed.User.Username() != "ssh" {
		t.Errorf("link target = %s %s %s", parsed.Scheme, parsed.Host, parsed.User.Username())
	}
	if pass, _ := parsed.User.Password(); pass != "xxxx" {
		t.Errorf("link password = %q", pass)
	}
	if parsed.Query().Get("sshConfigType") != "SSH-Payload" || parsed.Query().Get("remarks") != npvGen2FixtureName {
		t.Errorf("link query = %v", parsed.Query())
	}
	if !strings.HasPrefix(parsed.Fragment, npvGen2FixturePayloadPrefix) {
		t.Errorf("link payload = %.60q", parsed.Fragment)
	}

	byName := make(map[string]string, len(keys))
	for _, kv := range keys {
		byName[kv[0]] = kv[1]
	}
	if byName["appKey A16 (gen-2 whitebox)"] != npvGen2FixtureA16 || byName["KDK"] != npvGen2FixtureKDK {
		t.Errorf("key material = %v", byName)
	}
}

func TestNpvGen2ModuleDecryptsFixture(t *testing.T) {
	data := npvGen2Fixture(t)

	mod, proto, payload := modules.Lookup("npvs_v5_appkey.npvs", data)
	if mod == nil {
		t.Fatal("no module matched the v5 fixture")
	}
	if mod.NeedsPassword != nil && mod.NeedsPassword(proto, payload) {
		t.Error("a v5 app-key config needs no passphrase")
	}

	res, err := mod.Decrypt(modules.Request{
		FileName: "npvs_v5_appkey.npvs",
		Data:     data,
		Proto:    proto,
		Payload:  payload,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Text, "ssh://ssh:xxxx@help.snapchat.com:80?") {
		t.Errorf("output does not carry the ssh link: %s", res.Text)
	}
	if strings.Contains(res.Text, `"sshConfig"`) || strings.Contains(res.Text, `"metadata"`) {
		t.Error("output must not dump JSON blocks")
	}
	if !strings.Contains(res.Text, "// ---- recovered key material ----") {
		t.Error("output must end with the key material block")
	}
	if res.FileName != "npvs_v5_appkey.txt" {
		t.Errorf("output name = %q", res.FileName)
	}
}

func TestNpvGen2RejectsTamperedConfig(t *testing.T) {
	data := npvGen2Fixture(t)

	for _, tc := range []struct {
		name  string
		off   int
		wants string
	}{
		{"salt", 9 + 55, "app-key wrap"},
		{"wrap", 9 + 80, "app-key wrap"},
		{"sealed metadata", 9 + 140, "sealed metadata"},
	} {
		bad := bytes.Clone(data)
		bad[tc.off] ^= 0xFF

		env, err := parseNpvGen2Envelope(bad)
		if err != nil {
			t.Fatalf("%s: parse: %v", tc.name, err)
		}
		if _, _, _, err := env.open(""); err == nil || !strings.Contains(err.Error(), tc.wants) {
			t.Errorf("%s: error = %v, want a %q error", tc.name, err, tc.wants)
		}
	}

	if _, err := parseNpvGen2Envelope(data[:64]); err == nil {
		t.Error("a truncated envelope must be rejected")
	}
	if isNpvGen2Envelope(data[1:]) {
		t.Error("a shifted magic must not be recognised")
	}
}

func TestNpvGen2LinkShapes(t *testing.T) {
	for _, tc := range []struct {
		name  string
		field map[string]string
		want  string
	}{
		{
			name: "shadowsocks",
			field: map[string]string{
				"configType": "3",
				"server":     "51.38.71.51",
				"serverPort": "2083",
				"method":     "aes-256-gcm",
				"password":   "s3cr3t",
				"remarks":    "ss sample",
			},
			want: "ss://YWVzLTI1Ni1nY206czNjcjN0@51.38.71.51:2083#ss%20sample",
		},
		{
			name: "vless reality tcp",
			field: map[string]string{
				"configType":  "5",
				"server":      "212.46.33.5",
				"serverPort":  "443",
				"password":    "71091100-c6ed-41fa-84ac-acf7ff9eeb7f",
				"method":      "none",
				"network":     "tcp",
				"security":    "reality",
				"flow":        "xtls-rprx-vision",
				"sni":         "www.example.com",
				"fingerPrint": "chrome",
				"publicKey":   "z37XIezsPyfMgmdXyQ2QzPJHaaZ4S4yUL6oKs1oQh0c",
				"shortId":     "46d9c2b8a1f0e5d3",
				"remarks":     "vless reality",
			},
			want: "vless://71091100-c6ed-41fa-84ac-acf7ff9eeb7f@212.46.33.5:443?encryption=none&flow=xtls-rprx-vision&fp=chrome&pbk=z37XIezsPyfMgmdXyQ2QzPJHaaZ4S4yUL6oKs1oQh0c&security=reality&sid=46d9c2b8a1f0e5d3&sni=www.example.com&type=tcp#vless%20reality",
		},
		{
			name: "vless tcp http obfs",
			field: map[string]string{
				"configType": "5",
				"server":     "198.51.100.7",
				"serverPort": "80",
				"password":   "6f4a1f2c-1111-2222-3333-444455556666",
				"method":     "none",
				"network":    "tcp",
				"headerType": "http",
				"host":       "cdn.example.net",
				"path":       "/stream",
				"remarks":    "vless http obfs",
			},
			want: "vless://6f4a1f2c-1111-2222-3333-444455556666@198.51.100.7:80?encryption=none&headerType=http&host=cdn.example.net&path=%2Fstream&security=none&type=tcp#vless%20http%20obfs",
		},
		{
			name: "vless grpc authority",
			field: map[string]string{
				"configType":  "5",
				"server":      "89.35.14.30",
				"serverPort":  "443",
				"password":    "2f35965a-9a9b-45fd-ba32-987296dfb6be",
				"method":      "none",
				"network":     "grpc",
				"mode":        "gun",
				"serviceName": "hub.v1.RelayService",
				"authority":   "/t.me/V2rayBaaz",
				"security":    "reality",
				"sni":         "md1.univesalsrv.com",
				"fingerPrint": "firefox",
				"publicKey":   "xWejOQUn2koOUorUPSrHMLM-lyHlZ441KzxKc9bLmkY",
				"shortId":     "221ba6174224ca5b",
				"remarks":     "vless grpc",
			},
			want: "vless://2f35965a-9a9b-45fd-ba32-987296dfb6be@89.35.14.30:443?authority=%2Ft.me%2FV2rayBaaz&encryption=none&fp=firefox&mode=gun&pbk=xWejOQUn2koOUorUPSrHMLM-lyHlZ441KzxKc9bLmkY&security=reality&serviceName=hub.v1.RelayService&sid=221ba6174224ca5b&sni=md1.univesalsrv.com&type=grpc#vless%20grpc",
		},
		{
			name: "vless xhttp extra",
			field: map[string]string{
				"configType":       "5",
				"server":           "198.51.100.10",
				"serverPort":       "8443",
				"password":         "abcdefab-1111-2222-3333-444455556666",
				"method":           "mlkem768x25519plus.native.0rtt.secret",
				"network":          "xhttp",
				"path":             "/x",
				"xhttpMode":        "auto",
				"xhttpExtra":       "pad=1",
				"security":         "tls",
				"sni":              "x.example.com",
				"tlsAllowInsecure": "true",
				"remarks":          "vless xhttp",
			},
			want: "vless://abcdefab-1111-2222-3333-444455556666@198.51.100.10:8443?allowInsecure=1&encryption=mlkem768x25519plus.native.0rtt.secret&extra=pad%3D1&mode=auto&path=%2Fx&security=tls&sni=x.example.com&type=xhttp#vless%20xhttp",
		},
		{
			name: "trojan ws tls",
			field: map[string]string{
				"configType":  "6",
				"server":      "199.232.78.101",
				"serverPort":  "443",
				"password":    "MItIVPN",
				"network":     "ws",
				"host":        "mitivpn-11e.global.ssl.fastly.net",
				"path":        "/",
				"security":    "tls",
				"sni":         "ssl.fastly.com",
				"alpn":        "http/1.1",
				"fingerPrint": "chrome",
				"remarks":     "trojan sample",
			},
			want: "trojan://MItIVPN@199.232.78.101:443?alpn=http%2F1.1&fp=chrome&host=mitivpn-11e.global.ssl.fastly.net&path=%2F&security=tls&sni=ssl.fastly.com&type=ws#trojan%20sample",
		},
		{
			name: "vmess tcp",
			field: map[string]string{
				"configType": "1",
				"server":     "me6.movieseryalirani.ir",
				"serverPort": "57361",
				"password":   "ed815caa-bff1-4419-8c61-1d108eabaf1f",
				"method":     "auto",
				"network":    "tcp",
				"headerType": "none",
				"remarks":    "vmess sample",
			},
			want: "vmess://eyJhZGQiOiJtZTYubW92aWVzZXJ5YWxpcmFuaS5pciIsImFpZCI6IjAiLCJhbHBuIjoiIiwiZnAiOiIiLCJob3N0IjoiIiwiaWQiOiJlZDgxNWNhYS1iZmYxLTQ0MTktOGM2MS0xZDEwOGVhYmFmMWYiLCJpbnNlY3VyZSI6IiIsIm5ldCI6InRjcCIsInBhdGgiOiIiLCJwb3J0IjoiNTczNjEiLCJwcyI6InZtZXNzIHNhbXBsZSIsInNjeSI6ImF1dG8iLCJzbmkiOiIiLCJ0bHMiOiIiLCJ0eXBlIjoibm9uZSIsInYiOiIyIn0=",
		},
		{
			name: "vmess kcp tls",
			field: map[string]string{
				"configType":       "1",
				"server":           "198.51.100.9",
				"serverPort":       "8080",
				"password":         "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
				"method":           "auto",
				"network":          "kcp",
				"headerType":       "srtp",
				"seed":             "seedvalue",
				"security":         "tls",
				"sni":              "kcp.example.com",
				"tlsAllowInsecure": "true",
				"remarks":          "vmess kcp",
			},
			want: "vmess://eyJhZGQiOiIxOTguNTEuMTAwLjkiLCJhaWQiOiIwIiwiYWxwbiI6IiIsImZwIjoiIiwiaG9zdCI6IiIsImlkIjoiYWFhYWFhYWEtYmJiYi1jY2NjLWRkZGQtZWVlZWVlZWVlZWVlIiwiaW5zZWN1cmUiOiIxIiwibmV0Ijoia2NwIiwicGF0aCI6InNlZWR2YWx1ZSIsInBvcnQiOiI4MDgwIiwicHMiOiJ2bWVzcyBrY3AiLCJzY3kiOiJhdXRvIiwic25pIjoia2NwLmV4YW1wbGUuY29tIiwidGxzIjoidGxzIiwidHlwZSI6InNydHAiLCJ2IjoiMiJ9",
		},
	} {
		if got := npvGen2V2RayLink(tc.field["remarks"], "", tc.field); got != tc.want {
			t.Errorf("%s link = %q, want %q", tc.name, got, tc.want)
		}
	}
}
