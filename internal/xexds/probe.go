package xexds

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/netip"
	"time"
)

// Request builds a type-1 probe request of the given total length with text
// at offset 8, zero padded.
func Request(total int, text string) []byte {
	b := make([]byte, total)
	binary.LittleEndian.PutUint16(b[0:], 1)
	binary.LittleEndian.PutUint16(b[4:], uint16(total))
	binary.LittleEndian.PutUint16(b[6:], uint16(total-36))
	copy(b[HeaderLen:], text)
	return b
}

// MTURequest and NATRequest are the two probes exactly as the dashboard sends them.
func MTURequest() []byte { return Request(MTULen, MTUText) }
func NATRequest() []byte { return Request(NATLen, NATText) }

// FollowUpRequest builds the 32-byte type-3 request from a type-1 reply.
func FollowUpRequest(reply []byte) []byte {
	b := make([]byte, FollowUpLen)
	binary.LittleEndian.PutUint16(b[0:], 1)
	binary.LittleEndian.PutUint16(b[2:], 3)
	copy(b[4:], reply[:CookieLen])
	return b
}

// MappedAddr reads the client address a type-1 reply carries.
func MappedAddr(reply []byte) netip.AddrPort {
	return netip.AddrPortFrom(netip.AddrFrom4([4]byte(reply[OffIP:OffIP+4])), binary.BigEndian.Uint16(reply[OffPort:]))
}

// CheckReply applies the checks the dashboard makes on a type-1 reply: exact
// length and the probe text at offset 0x24.
func CheckReply(reply []byte, total int, text string) error {
	if len(reply) != total {
		return fmt.Errorf("reply is %d bytes, want %d", len(reply), total)
	}
	if !bytes.Equal(reply[OffText:OffText+len(text)], []byte(text)) {
		return fmt.Errorf("probe text not at offset 0x24")
	}
	return nil
}

// Probe runs the dashboard's cycle against target (host:port) and writes a
// line per step to w: one MTU probe, then the NAT probe with 6 type-1 requests
// (every reply must carry the same mapped address) and 5 follow-ups. Each
// request is retried up to tries times, wait apiece. A follow-up that gets no
// answer is reported but not a failure: the dashboard accepts that too.
func Probe(target string, wait time.Duration, tries int, w io.Writer) bool {
	ok := true
	step := func(name string, err error, detail string) {
		if err != nil {
			ok = false
			fmt.Fprintf(w, "FAIL %s: %v\n", name, err)
			return
		}
		fmt.Fprintf(w, "PASS %s %s\n", name, detail)
	}

	mtu, err := net.Dial("udp4", target)
	if err != nil {
		fmt.Fprintf(w, "FAIL dial: %v\n", err)
		return false
	}
	defer mtu.Close()
	reply, err := exchange(mtu, MTURequest(), wait, tries)
	if err == nil {
		err = CheckReply(reply, MTULen, MTUText)
	}
	step("mtu probe", err, fmt.Sprintf("(%d bytes)", len(reply)))

	nat, err := net.Dial("udp4", target)
	if err != nil {
		fmt.Fprintf(w, "FAIL dial: %v\n", err)
		return false
	}
	defer nat.Close()
	var first netip.AddrPort
	for i := 1; i <= 6; i++ {
		reply, err := exchange(nat, NATRequest(), wait, tries)
		if err == nil {
			err = CheckReply(reply, NATLen, NATText)
		}
		if err != nil {
			step(fmt.Sprintf("nat probe %d/6", i), err, "")
			fmt.Fprintln(w, "RESULT FAIL")
			return false
		}
		mapped := MappedAddr(reply)
		if i == 1 {
			first = mapped
		} else if mapped != first {
			step(fmt.Sprintf("nat probe %d/6", i), fmt.Errorf("mapped address %s differs from first %s", mapped, first), "")
			fmt.Fprintln(w, "RESULT FAIL")
			return false
		}
		step(fmt.Sprintf("nat probe %d/6", i), nil, fmt.Sprintf("(%d bytes, mapped %s, local %s)", len(reply), mapped, nat.LocalAddr()))
		if i == 6 {
			break
		}
		fu, err := exchange(nat, FollowUpRequest(reply), wait, 1)
		if err != nil {
			fmt.Fprintf(w, "note follow-up %d/5: no answer (the dashboard accepts a timeout)\n", i)
			continue
		}
		fmt.Fprintf(w, "PASS follow-up %d/5 (%d bytes)\n", i, len(fu))
	}
	if ok {
		fmt.Fprintln(w, "RESULT PASS")
	} else {
		fmt.Fprintln(w, "RESULT FAIL")
	}
	return ok
}

// exchange sends req and reads one datagram, retrying on timeout.
func exchange(c net.Conn, req []byte, wait time.Duration, tries int) ([]byte, error) {
	buf := make([]byte, 65535)
	var err error
	for range max(tries, 1) {
		if _, err = c.Write(req); err != nil {
			return nil, err
		}
		c.SetReadDeadline(time.Now().Add(wait))
		var n int
		if n, err = c.Read(buf); err == nil {
			return append([]byte(nil), buf[:n]...), nil
		}
	}
	return nil, fmt.Errorf("no reply after %d tries: %w", max(tries, 1), err)
}
