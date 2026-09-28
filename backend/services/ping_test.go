package services

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

// fakeServer answers one Server List Ping the way a Minecraft server does,
// and records the handshake it received so the test can check the client's
// half of the exchange.
func fakeServer(t *testing.T, statusJSON string) (addr string, handshake <-chan []byte) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() }) //nolint:errcheck // test cleanup
	got := make(chan []byte, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }() //nolint:errcheck // test server
		hs, err := readPacket(conn)
		if err != nil {
			return
		}
		got <- hs
		if _, err := readPacket(conn); err != nil { // the empty status request
			return
		}
		if statusJSON == "" {
			return // hang up without answering
		}
		body := append(varint(int32(len(statusJSON))), statusJSON...)
		if _, err := conn.Write(packet(0x00, body)); err != nil {
			return
		}
	}()
	return ln.Addr().String(), got
}

func TestPingReadsAStatusResponse(t *testing.T) {
	status := `{"version":{"name":"Paper 1.20.6","protocol":766},"players":{"max":20,"online":3,"sample":[]},` +
		`"description":{"text":"§aLichdenstein","extra":[{"text":" · "},{"text":"survival","color":"gray"}]},"enforcesSecureChat":false}`
	addr, hs := fakeServer(t, status)

	got, err := Ping(context.Background(), addr)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Online || got.Players != 3 || got.MaxPlayers != 20 || got.Version != "Paper 1.20.6" {
		t.Fatalf("%+v", got)
	}
	if got.MOTD != "Lichdenstein · survival" {
		t.Fatalf("motd %q", got.MOTD)
	}
	if got.Latency <= 0 || got.Latency > 5*time.Second {
		t.Fatalf("latency %v", got.Latency)
	}

	// The handshake the server saw: id 0, protocol -1, our host, its port, next state 1.
	raw := <-hs
	id, rest, err := readVarint(raw)
	if err != nil || id != 0 {
		t.Fatalf("handshake id %d %v", id, err)
	}
	proto, rest, err := readVarint(rest)
	if err != nil || proto != -1 {
		t.Fatalf("protocol %d %v", proto, err)
	}
	host, err := readString(rest)
	if err != nil || string(host) != "127.0.0.1" {
		t.Fatalf("host %q %v", host, err)
	}
	rest = rest[len(varint(int32(len(host))))+len(host):]
	_, portStr, _ := net.SplitHostPort(addr)
	if int(binary.BigEndian.Uint16(rest[:2])) != atoi(portStr) {
		t.Fatalf("port %d want %s", binary.BigEndian.Uint16(rest[:2]), portStr)
	}
	next, _, err := readVarint(rest[2:])
	if err != nil || next != 1 {
		t.Fatalf("next state %d %v", next, err)
	}
}

func TestPingReportsAnUnreachableServer(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Ping(context.Background(), addr); err == nil {
		t.Fatal("a closed port must be an error")
	}
}

func TestPingRefusesABadAddressBeforeDialling(t *testing.T) {
	if _, err := Ping(context.Background(), "https://play.example"); err == nil {
		t.Fatal("an address with a scheme must be refused")
	}
}

func TestPingHandlesAServerThatHangsUp(t *testing.T) {
	addr, _ := fakeServer(t, "")
	if _, err := Ping(context.Background(), addr); err == nil {
		t.Fatal("no response must be an error, not an empty status")
	}
}

func TestReadPacketBoundsTheLength(t *testing.T) {
	huge := varint(pingMaxResponse + 1)
	if _, err := readPacket(bytes.NewReader(huge)); err == nil || !strings.Contains(err.Error(), "out of bounds") {
		t.Fatalf("got %v", err)
	}
	if _, err := readPacket(bytes.NewReader(varint(0))); err == nil {
		t.Fatal("a zero-length packet must be refused")
	}
	if _, err := readPacket(io.MultiReader(bytes.NewReader(varint(10)), bytes.NewReader([]byte{1, 2}))); err == nil {
		t.Fatal("a truncated packet must be an error")
	}
}

func TestVarintRoundTrip(t *testing.T) {
	for _, v := range []int32{0, 1, 127, 128, 255, 25565, 2147483647, -1, -2147483648} {
		got, rest, err := readVarint(varint(v))
		if err != nil || got != v || len(rest) != 0 {
			t.Fatalf("%d: got %d rest %d err %v", v, got, len(rest), err)
		}
	}
	if _, _, err := readVarint([]byte{0x80, 0x80, 0x80, 0x80, 0x80, 0x80}); err == nil {
		t.Fatal("a varint longer than five bytes must be refused")
	}
}

func TestComponentText(t *testing.T) {
	cases := map[string]string{
		`"plain"`:               "plain",
		`"§6gold§r and §lbold"`: "gold and bold",
		`{"text":"a","extra":["b",{"text":"c","extra":["d"]}]}`: "abcd",
		`{"translate":"x"}`: "",
		`42`:                "",
	}
	for in, want := range cases {
		if got := componentText([]byte(in)); got != want {
			t.Errorf("%s: got %q want %q", in, got, want)
		}
	}
}

func atoi(s string) int {
	n := 0
	for _, r := range s {
		n = n*10 + int(r-'0')
	}
	return n
}
