package xexds

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"
)

var (
	src = netip.MustParseAddrPort("192.0.2.77:49152")
	key = []byte("0123456789abcdef0123456789abcdef")
)

// The request packets as observed on the wire (docs/xbox360/wharf-base-plan.md,
// "XEXDS probe reply layout"), built here from the documented header bytes and
// texts rather than through the package's own builders.
func mtuReq() []byte {
	b := make([]byte, 1336)
	copy(b, []byte{0x01, 0x00, 0x00, 0x00, 0x38, 0x05, 0x14, 0x05})
	copy(b[8:], "Microsoft Corporation Xbox Live MTU Test\x00")
	return b
}

func natReq() []byte {
	b := make([]byte, 200)
	copy(b, []byte{0x01, 0x00, 0x00, 0x00, 0xc8, 0x00, 0xa4, 0x00})
	copy(b[8:], "Microsoft Corporation Xbox Live ICMP Test\x00")
	return b
}

func TestRequestBuildersMatchDocumentedBytes(t *testing.T) {
	if !bytes.Equal(MTURequest(), mtuReq()) || !bytes.Equal(NATRequest(), natReq()) {
		t.Fatal("request builders differ from the documented packets")
	}
	if got := hex.EncodeToString(mtuReq()[:12]); got != "01000000380514054d696372" {
		t.Fatalf("mtu prefix %s", got)
	}
	if got := hex.EncodeToString(natReq()[:10]); got != "01000000c800a4004d69" {
		t.Fatalf("nat prefix %s", got)
	}
}

func TestMTUReply(t *testing.T) {
	r := &Responder{Key: key}
	reply, kind, _ := r.Reply(mtuReq(), src)
	if kind != MTUProbe || len(reply) != 1336 {
		t.Fatalf("kind %v len %d", kind, len(reply))
	}
	text := "Microsoft Corporation Xbox Live MTU Test\x00"
	if len(text) != 0x29 {
		t.Fatal("text length")
	}
	if string(reply[0x24:0x24+len(text)]) != text {
		t.Fatalf("text not at 0x24: % x", reply[0x20:0x50])
	}
	// bytes after the text are zero padding
	if !bytes.Equal(reply[0x24+len(text):], make([]byte, 1336-0x24-len(text))) {
		t.Fatal("padding not zero")
	}
	if !bytes.Equal(reply[:8], mtuReq()[:8]) {
		t.Fatal("request header not echoed")
	}
	assertMapped(t, reply)
}

func TestNATReply(t *testing.T) {
	r := &Responder{Key: key}
	reply, kind, _ := r.Reply(natReq(), src)
	if kind != NATProbe || len(reply) != 200 {
		t.Fatalf("kind %v len %d", kind, len(reply))
	}
	text := "Microsoft Corporation Xbox Live ICMP Test\x00"
	if len(text) != 0x2a || string(reply[0x24:0x24+len(text)]) != text {
		t.Fatalf("text not at 0x24: % x", reply[0x20:0x50])
	}
	if !bytes.Equal(reply[:8], natReq()[:8]) {
		t.Fatal("request header not echoed")
	}
	assertMapped(t, reply)
	// tag: first 8 bytes of HMAC-SHA1(key, reply[0:20] + ip + port)
	m := hmac.New(sha1.New, key)
	m.Write(reply[:20])
	m.Write([]byte{192, 0, 2, 77, 0xc0, 0x00})
	if !bytes.Equal(reply[0x14:0x1c], m.Sum(nil)[:8]) {
		t.Fatal("cookie tag mismatch")
	}
}

func assertMapped(t *testing.T, reply []byte) {
	t.Helper()
	if !bytes.Equal(reply[0x1c:0x20], []byte{192, 0, 2, 77}) {
		t.Fatalf("mapped ip % x", reply[0x1c:0x20])
	}
	if binary.BigEndian.Uint16(reply[0x20:]) != 49152 {
		t.Fatalf("mapped port % x", reply[0x20:0x22])
	}
	if reply[0x22] != 0 || reply[0x23] != 0 {
		t.Fatal("0x22..0x24 not zero")
	}
}

func TestNonceVaries(t *testing.T) {
	r := &Responder{Key: key}
	a, _, _ := r.Reply(natReq(), src)
	b, _, _ := r.Reply(natReq(), src)
	if bytes.Equal(a[8:20], b[8:20]) {
		t.Fatal("cookie nonce repeated")
	}
}

func TestMappedAddressFollowsSource(t *testing.T) {
	r := &Responder{Key: key}
	other := netip.MustParseAddrPort("198.51.100.9:3074")
	reply, _, _ := r.Reply(natReq(), other)
	if got := MappedAddr(reply); got != other {
		t.Fatalf("mapped %v", got)
	}
	// IPv4-mapped IPv6 sources are unmapped.
	v6 := netip.MustParseAddrPort("[::ffff:198.51.100.9]:3074")
	reply, kind, _ := r.Reply(natReq(), v6)
	if kind != NATProbe || MappedAddr(reply) != other {
		t.Fatalf("v4-in-v6 source: kind %v", kind)
	}
	if _, kind, _ := r.Reply(natReq(), netip.MustParseAddrPort("[2001:db8::1]:3074")); kind != Ignored {
		t.Fatal("native IPv6 source answered")
	}
}

func TestFollowUp(t *testing.T) {
	r := &Responder{Key: key}
	reply, _, _ := r.Reply(natReq(), src)
	fu := make([]byte, 32)
	copy(fu, []byte{0x01, 0x00, 0x03, 0x00})
	copy(fu[4:], reply[:28])
	ans, kind, ok := r.Reply(fu, src)
	if kind != FollowUp || !ok || len(ans) != 36 {
		t.Fatalf("kind %v ok %v len %d", kind, ok, len(ans))
	}
	if !bytes.Equal(FollowUpRequest(reply), fu) {
		t.Fatal("FollowUpRequest differs from documented layout")
	}
	// Same cookie from another source address fails the tag but is still answered.
	ans, kind, ok = r.Reply(fu, netip.MustParseAddrPort("192.0.2.78:49152"))
	if kind != FollowUp || ok || len(ans) != 36 {
		t.Fatalf("wrong-source cookie: kind %v ok %v len %d", kind, ok, len(ans))
	}
	// A cookie signed with another key fails too.
	if _, _, ok := (&Responder{Key: []byte("another-key-another-key-another!")}).Reply(fu, src); ok {
		t.Fatal("cookie verified under another key")
	}
}

func TestIgnored(t *testing.T) {
	r := &Responder{Key: key}
	wrongText := natReq()
	wrongText[20] ^= 0xff
	badType := natReq()
	badType[0] = 2
	badSub := natReq()
	badSub[2] = 7
	cases := map[string][]byte{
		"empty":          nil,
		"short":          {1, 0, 0},
		"bad type":       badType,
		"bad subtype":    badSub,
		"wrong text":     wrongText,
		"truncated":      natReq()[:150],
		"long":           append(natReq(), 0),
		"mtu truncated":  mtuReq()[:1300],
		"short followup": append([]byte{1, 0, 3, 0}, make([]byte, 20)...),
		"long followup":  append([]byte{1, 0, 3, 0}, make([]byte, 40)...),
	}
	other := Request(400, NATText)
	cases["unknown total"] = other
	for name, req := range cases {
		if reply, kind, _ := r.Reply(req, src); reply != nil || kind != Ignored {
			t.Errorf("%s: answered (kind %v, %d bytes)", name, kind, len(reply))
		}
	}
}

func TestNeverLargerThanRequest(t *testing.T) {
	r := &Responder{Key: key}
	for _, req := range [][]byte{mtuReq(), natReq(), FollowUpRequest(mustReply(r, natReq()))} {
		reply, _, _ := r.Reply(req, src)
		if len(reply) > len(req)+4 { // the 32-byte follow-up gets a 36-byte answer
			t.Fatalf("reply %d bytes to a %d-byte request", len(reply), len(req))
		}
	}
}

func mustReply(r *Responder, req []byte) []byte {
	reply, _, _ := r.Reply(req, src)
	return reply
}

// The whole cycle over a real UDP socket: server on loopback, Probe against it.
func TestProbeAgainstServer(t *testing.T) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Skip("no loopback UDP:", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	srv := &Server{Responder: &Responder{Key: key}, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	go func() { done <- srv.Serve(ctx, conn) }()
	var out strings.Builder
	if !Probe(conn.LocalAddr().String(), time.Second, 2, &out) {
		t.Fatalf("probe failed:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "RESULT PASS") || strings.Count(out.String(), "PASS follow-up") != 5 || strings.Count(out.String(), "PASS nat probe") != 6 {
		t.Fatalf("unexpected probe output:\n%s", out.String())
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestProbeFailsWithoutServer(t *testing.T) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Skip(err)
	}
	addr := conn.LocalAddr().String()
	conn.Close()
	var out strings.Builder
	if Probe(addr, 50*time.Millisecond, 1, &out) {
		t.Fatalf("probe passed against nothing:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "RESULT FAIL") {
		t.Fatal(out.String())
	}
}

// A server that echoes the request (what the dashboard rejected) must fail the probe.
func TestProbeRejectsEcho(t *testing.T) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Skip(err)
	}
	defer conn.Close()
	go func() {
		buf := make([]byte, 65535)
		for {
			n, a, err := conn.ReadFromUDPAddrPort(buf)
			if err != nil {
				return
			}
			conn.WriteToUDPAddrPort(buf[:n], a)
		}
	}()
	var out strings.Builder
	if Probe(conn.LocalAddr().String(), 200*time.Millisecond, 1, &out) {
		t.Fatalf("echo passed:\n%s", out.String())
	}
}
