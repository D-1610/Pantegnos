package impl

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"Pantegnos/internal/modules"
)

// .bpf is the binary profile export the sing-box clients (SFA/SFI/SFM) write
// when a profile is shared. It is a container, not a cipher: the profile
// metadata and the sing-box JSON are gzip-compressed and length-prefixed, so
// recovering the config is a decompress and a walk through a handful of
// varints.
//
//	byte 0      message type, always 0x03 (profile content)
//	byte 1      container version, 0 or 1
//	byte 2..    gzip stream of:
//	              uvarint  name length, then the UTF-8 name
//	              int32BE  profile type: 0 local, 1 iCloud, 2 remote
//	              uvarint  config length, then the sing-box JSON
//	              remote only: uvarint source length, then the subscription URL
//	              remote only: byte    auto-update flag
//	              remote only, v1+:  int32BE auto-update interval in minutes
//	              remote only: int64BE last-updated timestamp
const (
	bpfMsgProfileContent = 0x03
	bpfVersionCurrent    = 1

	bpfProfileLocal  = 0
	bpfProfileICloud = 1
	bpfProfileRemote = 2

	// bpfMaxInflated caps the decompressed container. Profiles are kilobytes,
	// so a multi-megabyte result means a gzip bomb in a file that arrived from
	// a link or a channel, and it must not be allowed to exhaust a phone.
	bpfMaxInflated = 4 << 20

	// bpfMaxFieldLen caps a single length prefix for the same reason, and
	// keeps a corrupt length from turning into a huge allocation.
	bpfMaxFieldLen = 1 << 20

	// bpfMaxUvarintBytes matches the protobuf ceiling: ten bytes is the most
	// that can carry a 64-bit value, so anything longer is malformed.
	bpfMaxUvarintBytes = 10
)

// bpfProfile is one decoded .bpf container.
type bpfProfile struct {
	Version           int
	Name              string
	Type              int32
	Config            string
	RemotePath        string
	AutoUpdate        bool
	AutoUpdateMinutes int32
	LastUpdated       int64
}

func init() {
	modules.Register(modules.Module{
		Name:      "sing-box profile export for Android/iOS/macOS (.bpf)",
		ApkAuthor: "https://github.com/SagerNet/sing-box/releases",
		Proto:     []string{"bpf"},
		Extension: ".bpf",
		Decrypt:   decryptBPF,
	})
}

// bpfReader walks the inflated container. Every read is bounds checked, so a
// truncated or hostile file produces an error instead of a panic.
type bpfReader struct {
	b   []byte
	off int
}

func (r *bpfReader) take(n int) ([]byte, error) {
	if n < 0 || n > len(r.b)-r.off {
		return nil, fmt.Errorf("truncated at offset %d: need %d more bytes, have %d", r.off, n, len(r.b)-r.off)
	}
	out := r.b[r.off : r.off+n]
	r.off += n
	return out, nil
}

func (r *bpfReader) readByte() (byte, error) {
	b, err := r.take(1)
	if err != nil {
		return 0, err
	}
	return b[0], nil
}

func (r *bpfReader) readUvarint() (uint64, error) {
	var value uint64
	for i := range bpfMaxUvarintBytes {
		b, err := r.readByte()
		if err != nil {
			return 0, err
		}
		if i == bpfMaxUvarintBytes-1 && b > 1 {
			return 0, fmt.Errorf("uvarint at offset %d overflows 64 bits", r.off-1)
		}
		value |= uint64(b&0x7f) << (7 * i)
		if b&0x80 == 0 {
			return value, nil
		}
	}
	return 0, fmt.Errorf("uvarint is longer than %d bytes", bpfMaxUvarintBytes)
}

// readString reads a varint length followed by that many UTF-8 bytes. The
// length is a uint64 on the wire, so it is narrowed against the cap rather
// than trusted: a crafted 2^63 length must not reach the allocator.
func (r *bpfReader) readString(what string) (string, error) {
	n, err := r.readUvarint()
	if err != nil {
		return "", fmt.Errorf("%s length: %w", what, err)
	}
	if n > bpfMaxFieldLen {
		return "", fmt.Errorf("%s length %d exceeds the %d byte limit", what, n, bpfMaxFieldLen)
	}
	b, err := r.take(int(n))
	if err != nil {
		return "", fmt.Errorf("%s: %w", what, err)
	}
	return string(b), nil
}

func (r *bpfReader) readInt32BE() (int32, error) {
	b, err := r.take(4)
	if err != nil {
		return 0, err
	}
	return int32(binary.BigEndian.Uint32(b)), nil
}

func (r *bpfReader) readInt64BE() (int64, error) {
	b, err := r.take(8)
	if err != nil {
		return 0, err
	}
	return int64(binary.BigEndian.Uint64(b)), nil
}

func bpfGunzip(b []byte) ([]byte, error) {
	zr, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("payload is not a gzip stream: %w", err)
	}
	defer zr.Close()

	// One byte past the cap is enough to tell "at the limit" from "over it"
	// without buffering an unbounded amount first.
	out, err := io.ReadAll(io.LimitReader(zr, bpfMaxInflated+1))
	if err != nil {
		return nil, fmt.Errorf("inflate failed: %w", err)
	}
	if len(out) > bpfMaxInflated {
		return nil, fmt.Errorf("inflated payload exceeds the %d byte limit", bpfMaxInflated)
	}
	return out, nil
}

// bpfEnvelope validates the two header bytes and inflates the body.
func bpfEnvelope(raw []byte) (version int, body []byte, err error) {
	if len(raw) < 2 {
		return 0, nil, fmt.Errorf("file is %d bytes, too small to be a .bpf profile", len(raw))
	}
	if raw[0] != bpfMsgProfileContent {
		return 0, nil, fmt.Errorf("message type 0x%02x, expected 0x%02x (profile content)", raw[0], bpfMsgProfileContent)
	}

	version = int(raw[1])
	if version > bpfVersionCurrent {
		return 0, nil, fmt.Errorf("container version %d is newer than the supported version %d", version, bpfVersionCurrent)
	}

	body, err = bpfGunzip(raw[2:])
	if err != nil {
		return 0, nil, err
	}
	return version, body, nil
}

func parseBPF(raw []byte) (*bpfProfile, error) {
	version, body, err := bpfEnvelope(raw)
	if err != nil {
		return nil, err
	}

	r := &bpfReader{b: body}

	name, err := r.readString("profile name")
	if err != nil {
		return nil, err
	}
	kind, err := r.readInt32BE()
	if err != nil {
		return nil, fmt.Errorf("profile type: %w", err)
	}
	switch kind {
	case bpfProfileLocal, bpfProfileRemote:
	case bpfProfileICloud:
		return nil, fmt.Errorf("%q is an iCloud profile: the clients stream the config from the account instead of storing it in the export, so recover it from iCloud", name)
	default:
		return nil, fmt.Errorf("unknown profile type %d", kind)
	}
	config, err := r.readString("config")
	if err != nil {
		return nil, err
	}

	p := &bpfProfile{Version: version, Name: name, Type: kind, Config: config}
	if kind == bpfProfileRemote {
		if p.RemotePath, err = r.readString("remote source"); err != nil {
			return nil, err
		}
		flag, err := r.readByte()
		if err != nil {
			return nil, fmt.Errorf("auto-update flag: %w", err)
		}
		p.AutoUpdate = flag != 0
		// Version 0 predates the interval field; version 1 added it ahead of
		// the timestamp.
		if version >= 1 {
			if p.AutoUpdateMinutes, err = r.readInt32BE(); err != nil {
				return nil, fmt.Errorf("auto-update interval: %w", err)
			}
		}
		if p.LastUpdated, err = r.readInt64BE(); err != nil {
			return nil, fmt.Errorf("last-updated timestamp: %w", err)
		}
	}

	return p, nil
}

func bpfProfileTypeName(kind int32) string {
	switch kind {
	case bpfProfileLocal:
		return "local"
	case bpfProfileRemote:
		return "remote"
	case bpfProfileICloud:
		return "iCloud"
	default:
		return "unknown"
	}
}

func decryptBPF(req modules.Request) (modules.Result, error) {
	profile, err := parseBPF(req.Data)
	if err != nil {
		return modules.Result{}, fmt.Errorf("bpf: %w", err)
	}

	name := profile.Name
	if name == "" {
		name = modules.Stem(req.FileName, ".bpf")
	}

	var sb strings.Builder
	sb.WriteString("// ---- sing-box profile ----\n")
	fmt.Fprintf(&sb, "// name: %s\n", name)
	fmt.Fprintf(&sb, "// type: %s (container version %d)\n", bpfProfileTypeName(profile.Type), profile.Version)
	if profile.Type == bpfProfileRemote {
		fmt.Fprintf(&sb, "// source: %s\n", profile.RemotePath)
		switch {
		case !profile.AutoUpdate:
			sb.WriteString("// auto-update: off\n")
		case profile.AutoUpdateMinutes > 0:
			fmt.Fprintf(&sb, "// auto-update: every %d min\n", profile.AutoUpdateMinutes)
		default:
			sb.WriteString("// auto-update: on\n")
		}
		if profile.LastUpdated > 0 {
			fmt.Fprintf(&sb, "// last updated: %s\n", time.UnixMilli(profile.LastUpdated).UTC().Format(time.RFC3339))
		}
	}
	sb.WriteString("\n")
	sb.WriteString(bpfRenderConfig(profile.Config))
	sb.WriteString("\n")

	return modules.Result{
		Text:     sb.String(),
		FileName: modules.OutputName(req.FileName, ".bpf"),
		Echo:     true,
	}, nil
}

// bpfRenderConfig re-indents the sing-box JSON so the result is a file
// `sing-box check` accepts and a reader can follow. A subscription may hand
// back something that is not JSON, and showing it verbatim still beats
// refusing to show it.
func bpfRenderConfig(config string) string {
	trimmed := strings.TrimSpace(config)
	if trimmed == "" {
		return "// (the profile carries no config of its own)"
	}

	var out bytes.Buffer
	if err := json.Indent(&out, []byte(trimmed), "", "  "); err != nil {
		return config
	}
	return out.String()
}
