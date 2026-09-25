package impl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const npvOpenSample = "NPVO1\n" + `{"configs":[{"name":"Demo","address":"demo.example:443","v2rayProfile":{` +
	`"configType":5,"remarks":"npvs1:RGVtbyIs","server":"npvs1:ZGVtby5leGFtcGxl","serverPort":"npvs1:NDQz",` +
	`"password":"npvs1:MDAwMDAwMDAtMDAwMC0wMDAwLTAwMDAtMDAwMDAwMDAwMDAw","method":"npvs1:bm9uZQ==",` +
	`"network":"npvs1:d3M=","host":"npvs1:ZGVtby5leGFtcGxl","path":"npvs1:L3dz","security":"npvs1:dGxz",` +
	`"sni":"npvs1:ZGVtby5leGFtcGxl","alpn":"npvs1:aHR0cC8xLjE=","fingerPrint":"npvs1:Y2hyb21l",` +
	`"insecure":false,"tlsAllowInsecure":false}}]}`

func TestNpvOpenEnvelopeRendersShareURI(t *testing.T) {
	if !isNpvOpenEnvelope([]byte(npvOpenSample)) {
		t.Fatal("open envelope not detected")
	}

	env, err := parseNpvOpenEnvelope([]byte(npvOpenSample))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(env.Configs) != 1 {
		t.Fatalf("configs = %d, want 1", len(env.Configs))
	}

	links := npvOpenLinks(env)
	if len(links) != 1 {
		t.Fatalf("links = %d, want 1", len(links))
	}
	if !strings.HasPrefix(links[0], "vless://") {
		t.Fatalf("link = %q, want a vless share uri", links[0])
	}
	if err := npvVerifyShareURIs(links); err != nil {
		t.Fatalf("round trip: %v", err)
	}

	parsed, err := npvParseShareURI(links[0])
	if err != nil {
		t.Fatalf("parse share uri: %v", err)
	}
	if parsed.Secret != "00000000-0000-0000-0000-000000000000" {
		t.Errorf("secret = %q", parsed.Secret)
	}
	if parsed.Host != "demo.example" || parsed.Port != 443 {
		t.Errorf("endpoint = %s:%d", parsed.Host, parsed.Port)
	}
	if parsed.Params["type"] != "ws" || parsed.Params["security"] != "tls" {
		t.Errorf("params = %v", parsed.Params)
	}
	if parsed.Remark != "Demo" {
		t.Errorf("remark = %q", parsed.Remark)
	}
}

func TestNpvLegacyMarkerDetected(t *testing.T) {
	if got := npvLegacyMarkerOf([]byte("NPVTSUB1\n{\"configs\":[]}")); got != npvSubMarker {
		t.Fatalf("marker = %q, want %q", got, npvSubMarker)
	}
	if got := npvLegacyMarkerOf([]byte("NPVT1\nabc")); got != npvLegacyMarker {
		t.Fatalf("marker = %q, want %q", got, npvLegacyMarker)
	}
	if got := npvLegacyMarkerOf([]byte("NPVS\x05")); got != "" {
		t.Fatalf("marker = %q, want empty", got)
	}
}

func TestNpvOpenFixture(t *testing.T) {
	dir := os.Getenv("PAN_NPVS_FIXTURES")
	if dir == "" {
		t.Skip("PAN_NPVS_FIXTURES not set")
	}
	data, err := os.ReadFile(filepath.Join(dir, "@FreeNetX_ir \U0001f1f3\U0001f1f1.npvs"))
	if err != nil {
		t.Skip(err)
	}
	env, err := parseNpvOpenEnvelope(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	links := npvOpenLinks(env)
	if err := npvVerifyShareURIs(links); err != nil {
		t.Fatalf("round trip: %v", err)
	}
	t.Logf("links: %v", links)
}
