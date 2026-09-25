package impl

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"Pantegnos/internal/modules"
)

const (
	npvOpenMarker   = "NPVO1"
	npvSubMarker    = "NPVTSUB1"
	npvLegacyMarker = "NPVT1"
	npvOpenMaxSize  = 8 << 20
)

type npvOpenEnvelope struct {
	Configs []any `json:"configs"`
}

func isNpvOpenEnvelope(b []byte) bool {
	head := strings.TrimLeft(string(b), " \t\r\n")
	return strings.HasPrefix(head, npvOpenMarker)
}

func npvLegacyMarkerOf(b []byte) string {
	head := strings.TrimLeft(string(b), " \t\r\n")
	switch {
	case strings.HasPrefix(head, npvSubMarker):
		return npvSubMarker
	case strings.HasPrefix(head, npvLegacyMarker):
		return npvLegacyMarker
	}
	return ""
}

func npvOpenStripMarker(s, marker string) string {
	s = strings.ReplaceAll(s, marker+"\n", "")
	return strings.ReplaceAll(s, marker, "")
}

func parseNpvOpenEnvelope(b []byte) (*npvOpenEnvelope, error) {
	if len(b) > npvOpenMaxSize {
		return nil, fmt.Errorf("npvs open: file too large: %d bytes", len(b))
	}

	body := npvOpenStripMarker(string(b), npvOpenMarker)
	if strings.TrimSpace(body) == "" {
		return nil, errors.New("npvs open: empty payload after marker")
	}

	var env npvOpenEnvelope
	if err := json.Unmarshal([]byte(body), &env); err != nil {
		return nil, fmt.Errorf("npvs open: payload is not a config list: %w", err)
	}
	if len(env.Configs) == 0 {
		return nil, errors.New("npvs open: payload has no configs")
	}
	return &env, nil
}

func npvOpenDecodeSentinels(v any) any {
	switch t := v.(type) {
	case string:
		return decodeNpvSentinels(t)
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, x := range t {
			out[k] = npvOpenDecodeSentinels(x)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, x := range t {
			out[i] = npvOpenDecodeSentinels(x)
		}
		return out
	default:
		return v
	}
}

func npvOpenLinks(env *npvOpenEnvelope) []string {
	links := make([]string, 0, len(env.Configs))
	for _, cfg := range env.Configs {
		links = append(links, npvGen2Link(npvOpenDecodeSentinels(cfg)))
	}
	return links
}

func decryptNPVSOpen(req modules.Request) (modules.Result, error) {
	env, err := parseNpvOpenEnvelope(req.Data)
	if err != nil {
		return modules.Result{}, err
	}

	links := npvOpenLinks(env)

	var sb strings.Builder
	sb.WriteString(strings.Join(links, "\n"))
	sb.WriteString("\n\n// ---- container ----\n")
	sb.WriteString("// NPVO1 (open export, plain JSON payload, no key material)\n")
	fmt.Fprintf(&sb, "// configs: %d\n", len(env.Configs))
	fmt.Fprintf(&sb, "// source: %s\n", filepath.Base(req.FileName))

	return modules.Result{
		Text:     sb.String(),
		FileName: modules.OutputName(req.FileName, ".npvs"),
		Echo:     true,
	}, nil
}

type npvShareURI struct {
	Protocol string
	Host     string
	Port     int
	Secret   string
	Params   map[string]string
	Remark   string
}

func npvParseShareURI(raw string) (*npvShareURI, error) {
	raw = strings.TrimSpace(raw)
	scheme, rest, ok := strings.Cut(raw, "://")
	if !ok {
		return nil, fmt.Errorf("share uri: missing scheme: %q", raw)
	}

	switch scheme {
	case "vless", "trojan":
		u, err := url.Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("share uri: %s: %w", scheme, err)
		}
		secret := ""
		if u.User != nil {
			secret = u.User.Username()
		}
		if secret == "" {
			return nil, fmt.Errorf("share uri: %s: missing secret", scheme)
		}
		if u.Hostname() == "" {
			return nil, fmt.Errorf("share uri: %s: missing host", scheme)
		}
		port, err := strconv.Atoi(u.Port())
		if err != nil || port <= 0 || port > 65535 {
			return nil, fmt.Errorf("share uri: %s: bad port %q", scheme, u.Port())
		}
		return &npvShareURI{
			Protocol: scheme,
			Host:     u.Hostname(),
			Port:     port,
			Secret:   secret,
			Params:   npvQueryMap(u),
			Remark:   npvFragment(u),
		}, nil

	case "vmess":
		payload, _, _ := strings.Cut(rest, "#")
		dec, err := npvBase64Decode(payload)
		if err != nil {
			return nil, fmt.Errorf("share uri: vmess: %w", err)
		}
		var obj map[string]any
		if err := json.Unmarshal(dec, &obj); err != nil {
			return nil, fmt.Errorf("share uri: vmess: bad json: %w", err)
		}
		params := make(map[string]string, len(obj))
		for k, v := range obj {
			params[k] = npvGen2Text(v)
		}
		if params["add"] == "" || params["id"] == "" {
			return nil, errors.New("share uri: vmess: missing add/id")
		}
		port := npvGen2Int(params["port"])
		if port <= 0 || port > 65535 {
			return nil, fmt.Errorf("share uri: vmess: bad port %d", port)
		}
		return &npvShareURI{
			Protocol: "vmess",
			Host:     params["add"],
			Port:     port,
			Secret:   params["id"],
			Params:   params,
			Remark:   params["ps"],
		}, nil

	case "ss":
		body, frag, _ := strings.Cut(rest, "#")
		userinfo, hostPort, ok := strings.Cut(body, "@")
		if !ok {
			dec, err := npvBase64Decode(body)
			if err != nil {
				return nil, fmt.Errorf("share uri: ss: %w", err)
			}
			userinfo, hostPort, ok = strings.Cut(string(dec), "@")
			if !ok {
				return nil, errors.New("share uri: ss: malformed payload")
			}
		} else if dec, err := npvBase64Decode(userinfo); err == nil {
			userinfo = string(dec)
		}
		if !strings.Contains(userinfo, ":") {
			return nil, errors.New("share uri: ss: missing method:password")
		}
		host, portStr, ok := strings.Cut(hostPort, ":")
		if !ok {
			return nil, fmt.Errorf("share uri: ss: missing port in %q", hostPort)
		}
		port, err := strconv.Atoi(portStr)
		if err != nil || port <= 0 || port > 65535 {
			return nil, fmt.Errorf("share uri: ss: bad port %q", portStr)
		}
		return &npvShareURI{
			Protocol: "ss",
			Host:     host,
			Port:     port,
			Secret:   userinfo,
			Params:   map[string]string{},
			Remark:   npvUrlUnescape(frag),
		}, nil
	}

	return nil, fmt.Errorf("share uri: unsupported protocol %q", scheme)
}

func npvVerifyShareURIs(links []string) error {
	for i, link := range links {
		if _, err := npvParseShareURI(link); err != nil {
			return fmt.Errorf("link %d: %w", i, err)
		}
	}
	return nil
}

func npvQueryMap(u *url.URL) map[string]string {
	q := u.Query()
	out := make(map[string]string, len(q))
	for k, v := range q {
		if len(v) > 0 {
			out[k] = v[0]
		}
	}
	return out
}

func npvFragment(u *url.URL) string {
	return npvUrlUnescape(u.Fragment)
}

func npvUrlUnescape(s string) string {
	out, err := url.PathUnescape(s)
	if err != nil {
		return s
	}
	return out
}

func npvBase64Decode(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.URLEncoding,
		base64.RawStdEncoding,
		base64.RawURLEncoding,
	}
	for _, enc := range encodings {
		if dec, err := enc.DecodeString(s); err == nil {
			return dec, nil
		}
	}
	return nil, errors.New("invalid base64")
}
