package services

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

// The Java Edition Server List Ping, as minecraft.wiki documents it (read
// 2026-09-28): a TCP connection, a Handshake packet (id 0x00: protocol version
// varint, address string, port uint16, next state varint 1), an empty Status
// Request packet (id 0x00), and a Status Response packet (id 0x00) carrying a
// JSON string. Every packet is length-prefixed with a varint, and so is every
// string. By convention a client that is only asking sends protocol -1.
//
// This is the whole of what the launcher does with a server before Prism
// takes over: it asks, once per interval, whether the server is up. It never
// sends anything a server could act on, and it only ever asks the manifest's
// own address (agent_docs/SECURITY_CHECKLIST.md, S8).

const (
	pingTimeout        = 5 * time.Second
	pingMaxResponse    = 1 << 20 // 1 MiB; a status JSON with a favicon is ~10 KB
	pingProtocolAsking = -1
	defaultMinecraft   = 25565
)

// lookupSRV is the resolver's SRV lookup, injected so the SRV path is testable
// without DNS.
var lookupSRV = net.DefaultResolver.LookupSRV

// resolveTarget is where a ping connects, the way the game itself decides: an
// address with a port is dialled as given; without one, the host's
// _minecraft._tcp SRV record names the target and port, as tunnels such as
// playit.gg publish them; without a usable record, the host on 25565. The
// target comes from DNS, so it must itself be a plain host name; anything
// else falls back to the manifest's host. The handshake still names the
// manifest's host, as a client does, so a proxy routing by name still works.
func resolveTarget(ctx context.Context, host string, port int) (string, int) {
	if port != 0 {
		return host, port
	}
	_, records, err := lookupSRV(ctx, "minecraft", "tcp", host)
	if err != nil || len(records) == 0 || records[0].Port == 0 {
		return host, defaultMinecraft
	}
	target, p, err := ParseServerAddress(strings.TrimSuffix(records[0].Target, "."))
	if err != nil || p != 0 {
		return host, defaultMinecraft
	}
	return target, int(records[0].Port)
}

// PingResult is what a status query returns, before it is folded into a
// models.ServerStatus for the UI.
type PingResult struct {
	Online     bool
	Players    int
	MaxPlayers int
	Version    string
	MOTD       string
	Latency    time.Duration
}

// Ping performs one Server List Ping against host[:port]. A refused
// connection, a timeout and a malformed response all come back as an error;
// the caller decides that "error" means "offline" for the UI.
func Ping(ctx context.Context, address string) (PingResult, error) {
	host, port, err := ParseServerAddress(address)
	if err != nil {
		return PingResult{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	dialHost, port := resolveTarget(ctx, host, port)

	started := time.Now()
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(dialHost, strconv.Itoa(port)))
	if err != nil {
		return PingResult{}, fmt.Errorf("ping %s: %w", address, err)
	}
	defer func() {
		_ = conn.Close() //nolint:errcheck // nothing left to do with a close error on a read-only probe
	}()
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			return PingResult{}, fmt.Errorf("ping %s: %w", address, err)
		}
	}

	if _, err := conn.Write(handshakePacket(host, port)); err != nil {
		return PingResult{}, fmt.Errorf("ping %s: handshake: %w", address, err)
	}
	if _, err := conn.Write(packet(0x00, nil)); err != nil {
		return PingResult{}, fmt.Errorf("ping %s: status request: %w", address, err)
	}

	body, err := readPacket(conn)
	if err != nil {
		return PingResult{}, fmt.Errorf("ping %s: %w", address, err)
	}
	latency := time.Since(started)

	id, rest, err := readVarint(body)
	if err != nil || id != 0x00 {
		return PingResult{}, fmt.Errorf("ping %s: unexpected packet id", address)
	}
	raw, err := readString(rest)
	if err != nil {
		return PingResult{}, fmt.Errorf("ping %s: %w", address, err)
	}
	result, err := parseStatus(raw)
	if err != nil {
		return PingResult{}, fmt.Errorf("ping %s: %w", address, err)
	}
	result.Latency = latency
	return result, nil
}

// statusResponse is the subset of the response JSON the UI shows. description
// is a text component: a plain string, or an object with text and extra.
type statusResponse struct {
	Version struct {
		Name string `json:"name"`
	} `json:"version"`
	Players struct {
		Online int `json:"online"`
		Max    int `json:"max"`
	} `json:"players"`
	Description json.RawMessage `json:"description"`
}

func parseStatus(raw []byte) (PingResult, error) {
	var s statusResponse
	if err := json.Unmarshal(raw, &s); err != nil {
		return PingResult{}, fmt.Errorf("status json: %w", err)
	}
	return PingResult{
		Online:     true,
		Players:    s.Players.Online,
		MaxPlayers: s.Players.Max,
		Version:    s.Version.Name,
		MOTD:       componentText(s.Description),
	}, nil
}

// componentText flattens a chat component into plain text: a string as is,
// an object's text followed by each of its extras, recursively. Formatting
// codes (§x) are stripped and the result trimmed once; the launcher shows
// the words, not the colours.
func componentText(raw json.RawMessage) string {
	return strings.TrimSpace(stripFormatting(flattenComponent(raw)))
}

func flattenComponent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var str string
	if json.Unmarshal(raw, &str) == nil {
		return str
	}
	var obj struct {
		Text  string            `json:"text"`
		Extra []json.RawMessage `json:"extra"`
	}
	if json.Unmarshal(raw, &obj) != nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(obj.Text)
	for _, e := range obj.Extra {
		b.WriteString(flattenComponent(e))
	}
	return b.String()
}

func stripFormatting(s string) string {
	var b strings.Builder
	skip := false
	for _, r := range s {
		if skip {
			skip = false
			continue
		}
		if r == '§' {
			skip = true
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// ── Packet encoding ─────────────────────────────────────────────────────────

func handshakePacket(host string, port int) []byte {
	var b bytes.Buffer
	b.Write(varint(pingProtocolAsking))
	b.Write(varint(int32(len(host))))
	b.WriteString(host)
	var p [2]byte
	binary.BigEndian.PutUint16(p[:], uint16(port))
	b.Write(p[:])
	b.Write(varint(1))
	return packet(0x00, b.Bytes())
}

// packet frames an id and a body with the varint length prefix.
func packet(id int32, body []byte) []byte {
	inner := append(varint(id), body...)
	return append(varint(int32(len(inner))), inner...)
}

func varint(v int32) []byte {
	u := uint32(v)
	var out []byte
	for {
		b := byte(u & 0x7f)
		u >>= 7
		if u != 0 {
			out = append(out, b|0x80)
			continue
		}
		return append(out, b)
	}
}

var errVarint = errors.New("malformed varint")

func readVarint(b []byte) (int32, []byte, error) {
	var result uint32
	for i := 0; i < 5 && i < len(b); i++ {
		result |= uint32(b[i]&0x7f) << (7 * i)
		if b[i]&0x80 == 0 {
			return int32(result), b[i+1:], nil
		}
	}
	return 0, nil, errVarint
}

func readString(b []byte) ([]byte, error) {
	n, rest, err := readVarint(b)
	if err != nil {
		return nil, err
	}
	if n < 0 || int(n) > len(rest) {
		return nil, errors.New("string length exceeds packet")
	}
	return rest[:n], nil
}

// readPacket reads one length-prefixed packet, refusing anything over
// pingMaxResponse so a hostile server cannot make the client allocate at will.
func readPacket(r io.Reader) ([]byte, error) {
	var lenBytes []byte
	one := make([]byte, 1)
	for i := 0; i < 5; i++ {
		if _, err := io.ReadFull(r, one); err != nil {
			return nil, fmt.Errorf("read length: %w", err)
		}
		lenBytes = append(lenBytes, one[0])
		if one[0]&0x80 == 0 {
			break
		}
	}
	n, _, err := readVarint(lenBytes)
	if err != nil {
		return nil, err
	}
	if n <= 0 || n > pingMaxResponse {
		return nil, fmt.Errorf("packet length %d out of bounds", n)
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, fmt.Errorf("read packet: %w", err)
	}
	return body, nil
}
