package impl

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

func TestDecodeOne(t *testing.T) {
	b64 := base64.StdEncoding.EncodeToString([]byte("hello-wasm"))
	b64NoPad := strings.TrimRight(b64, "=")
	b64URL := base64.URLEncoding.EncodeToString([]byte("hello-wasm"))

	tests := []struct {
		name    string
		input   string
		want    []byte
		wantErr bool
	}{
		{"hex", "68656c6c6f", []byte("hello"), false},
		{"base64 std padded", b64, []byte("hello-wasm"), false},
		{"base64 std unpadded", b64NoPad, []byte("hello-wasm"), false},
		{"base64 url-safe", b64URL, []byte("hello-wasm"), false},
		{"empty", "", nil, true},
		{"junk", "not valid !!!", nil, true},
	}

	for _, tt := range tests {
		got, err := decodeOne(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("%s: decodeOne(%q) = %v, want error", tt.name, tt.input, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: decodeOne(%q) error: %v", tt.name, tt.input, err)
			continue
		}
		if !bytes.Equal(got, tt.want) {
			t.Errorf("%s: decodeOne(%q) = %v, want %v", tt.name, tt.input, got, tt.want)
		}
	}
}

func TestWsHost(t *testing.T) {
	tests := []struct {
		name string
		ws   *wsSettingsT
		want string
	}{
		{"top-level host", &wsSettingsT{Host: "front.example"}, "front.example"},
		{"headers Host", &wsSettingsT{Headers: map[string]string{"Host": "hdr.example"}}, "hdr.example"},
		{"headers host", &wsSettingsT{Headers: map[string]string{"host": "hdr.example"}}, "hdr.example"},
		{
			"top-level host wins over headers",
			&wsSettingsT{Host: "front.example", Headers: map[string]string{"Host": "hdr.example"}},
			"front.example",
		},
		{"empty", &wsSettingsT{}, ""},
		{"unrelated header", &wsSettingsT{Headers: map[string]string{"X-Other": "v"}}, ""},
	}

	for _, tt := range tests {
		if got := wsHost(tt.ws); got != tt.want {
			t.Errorf("%s: wsHost = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestHostPort(t *testing.T) {
	tests := []struct {
		name string
		addr string
		port int
		want string
	}{
		{"hostname", "example.com", 443, "example.com:443"},
		{"ipv4", "203.0.113.7", 443, "203.0.113.7:443"},
		{"ipv6 bare", "2001:db8::1", 443, "[2001:db8::1]:443"},
		{"ipv6 loopback", "::1", 8443, "[::1]:8443"},
		{"ipv6 already bracketed", "[2001:db8::1]", 443, "[2001:db8::1]:443"},
		{"ipv6 zone", "fe80::1%eth0", 443, "[fe80::1%eth0]:443"},
		{"ipv4 mapped", "::ffff:203.0.113.7", 443, "[::ffff:203.0.113.7]:443"},
	}

	for _, tt := range tests {
		if got := hostPort(tt.addr, tt.port); got != tt.want {
			t.Errorf("%s: hostPort(%q, %d) = %q, want %q", tt.name, tt.addr, tt.port, got, tt.want)
		}
	}
}

// Domain-fronted profile: the Host header and the TLS SNI name differ, so the
// host= parameter has to survive independently of sni.
func TestVlessURIDomainFronting(t *testing.T) {
	ss := &streamSettingsT{
		Network:  "ws",
		Security: "tls",
		TLSSettings: &tlsSettingsT{
			ServerName: "cdn.example",
		},
		WSSettings: &wsSettingsT{
			Path: "/ws",
			Host: "front.example",
		},
	}
	vs := vnextSettingsT{Vnext: []vnextEntryT{{
		Address: "198.51.100.9",
		Port:    443,
		Users:   []vnextUserT{{Id: "uuid-1", Encryption: "none"}},
	}}}

	uri, err := vlessURI(vs, ss, "fronted")
	if err != nil {
		t.Fatalf("vlessURI: %v", err)
	}

	q := queryOf(t, uri)
	if got := q.Get("host"); got != "front.example" {
		t.Errorf("host = %q, want %q", got, "front.example")
	}
	if got := q.Get("sni"); got != "cdn.example" {
		t.Errorf("sni = %q, want %q", got, "cdn.example")
	}
	if got := q.Get("path"); got != "/ws" {
		t.Errorf("path = %q, want %q", got, "/ws")
	}
}

func TestVlessURIIPv6(t *testing.T) {
	ss := &streamSettingsT{Network: "ws", WSSettings: &wsSettingsT{Path: "/ws"}}
	vs := vnextSettingsT{Vnext: []vnextEntryT{{
		Address: "2001:db8::1",
		Port:    443,
		Users:   []vnextUserT{{Id: "uuid-1", Encryption: "none"}},
	}}}

	uri, err := vlessURI(vs, ss, "v6")
	if err != nil {
		t.Fatalf("vlessURI: %v", err)
	}
	if !strings.Contains(uri, "@[2001:db8::1]:443?") {
		t.Errorf("vless URI is not bracketed for IPv6: %s", uri)
	}
}

func TestTrojanAndShadowsocksIPv6(t *testing.T) {
	ts := serversSettingsT{Servers: []serverEntryT{{
		Address:  "2001:db8::1",
		Port:     8443,
		Password: "pw",
		Method:   "aes-256-gcm",
	}}}

	trojan, err := trojanURI(ts, &streamSettingsT{Network: "ws", WSSettings: &wsSettingsT{Path: "/ws"}}, "t6")
	if err != nil {
		t.Fatalf("trojanURI: %v", err)
	}
	if !strings.Contains(trojan, "@[2001:db8::1]:8443?") {
		t.Errorf("trojan URI is not bracketed for IPv6: %s", trojan)
	}

	ss, err := shadowsocksURI(ts, "s6")
	if err != nil {
		t.Fatalf("shadowsocksURI: %v", err)
	}
	if !strings.HasSuffix(ss, "@[2001:db8::1]:8443#s6") {
		t.Errorf("shadowsocks URI is not bracketed for IPv6: %s", ss)
	}
}

func TestVmessHostFromTopLevelWsHost(t *testing.T) {
	ss := &streamSettingsT{
		Network:    "ws",
		WSSettings: &wsSettingsT{Path: "/ws", Host: "front.example"},
	}
	vs := vnextSettingsT{Vnext: []vnextEntryT{{
		Address: "2001:db8::1",
		Port:    443,
		Users:   []vnextUserT{{Id: "uuid-1"}},
	}}}

	uri, err := vmessURI(vs, ss, "vm6")
	if err != nil {
		t.Fatalf("vmessURI: %v", err)
	}

	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(uri, "vmess://"))
	if err != nil {
		t.Fatalf("vmess payload is not base64: %v", err)
	}
	var obj map[string]string
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("vmess payload is not JSON: %v", err)
	}
	if got := obj["host"]; got != "front.example" {
		t.Errorf("host = %q, want %q", got, "front.example")
	}
	// vmess carries the address in a JSON field, so it must stay unbracketed.
	if got := obj["add"]; got != "2001:db8::1" {
		t.Errorf("add = %q, want %q", got, "2001:db8::1")
	}
}

func queryOf(t *testing.T, uri string) url.Values {
	t.Helper()
	i := strings.Index(uri, "?")
	if i < 0 {
		t.Fatalf("URI has no query: %s", uri)
	}
	frag := strings.Index(uri[i:], "#")
	if frag < 0 {
		frag = len(uri[i:])
	}
	q, err := url.ParseQuery(uri[i+1 : i+frag])
	if err != nil {
		t.Fatalf("URI query is unparseable: %v", err)
	}
	return q
}
