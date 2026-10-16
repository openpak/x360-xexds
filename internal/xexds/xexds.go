// Package xexds answers the Xbox 360 dashboard's connection probes to
// xexds.xboxlive.com (UDP 3074): the "Xbox Live MTU Test" and the
// "Xbox Live ICMP Test" NAT probe with its 32-byte follow-up.
//
// The layout comes from OpenPak's own analysis of the dashboard
// (docs/xbox360/wharf-base-plan.md, "XEXDS probe reply layout"); nothing is
// taken from Microsoft's service. Every multi-byte header field is
// little-endian, except the mapped port, which is network order.
//
// Request (client to server), 8-byte header then a NUL-terminated text:
//
//	0  u16 type (1)   2  u16 subtype (0, or 3 for the follow-up)
//	4  u16 total datagram length   6  u16 body length (total - 36)
//	8  text, zero padded to the total length
//
// Reply to a type-1 request, same length as the request:
//
//	0x00 20 bytes  request header + 12 random bytes (cookie part 1)
//	0x14  8 bytes  truncated HMAC-SHA1 over the first 20 bytes, the client IPv4
//	               and the client port (cookie part 2)
//	0x1C  4 bytes  the client's IPv4 address as the server sees it
//	0x20  2 bytes  the client's source port, network order
//	0x22  2 bytes  zero
//	0x24  text     the request text, zero padded to the total length
//
// The follow-up is 01 00 03 00 plus the 28 cookie bytes (reply[0x00..0x1C)).
// The dashboard accepts any datagram, or none, as its answer.
package xexds

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/binary"
	"net/netip"
)

const (
	// HeaderLen is the request header; ReplyHeaderLen the server header in front of the text in a reply.
	HeaderLen      = 8
	ReplyHeaderLen = 36
	// CookieLen is the opaque block of a type-1 reply that the follow-up returns.
	CookieLen = 28
	// FollowUpLen is the length of the type-3 follow-up request.
	FollowUpLen = 4 + CookieLen
	// MTULen and NATLen are the total lengths of the two probes.
	MTULen = 0x538
	NATLen = 200

	// Offsets in a type-1 reply.
	OffTag  = 0x14
	OffIP   = 0x1C
	OffPort = 0x20
	OffText = 0x24
	tagLen  = 8
)

// MTUText and NATText are the NUL-terminated probe texts, copied back into the reply.
const (
	MTUText = "Microsoft Corporation Xbox Live MTU Test\x00"  // 0x29 bytes
	NATText = "Microsoft Corporation Xbox Live ICMP Test\x00" // 0x2A bytes
)

// Kind classifies a datagram.
type Kind int

const (
	Ignored Kind = iota
	MTUProbe
	NATProbe
	FollowUp
)

func (k Kind) String() string {
	switch k {
	case MTUProbe:
		return "mtu"
	case NATProbe:
		return "nat"
	case FollowUp:
		return "followup"
	}
	return "ignored"
}

// Responder builds replies. Key signs the cookie tag; a restart with a new key
// only makes in-flight follow-ups fail the tag check, which is never fatal.
type Responder struct {
	Key []byte
}

// NewKey returns a random 32-byte cookie key.
func NewKey() []byte {
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		panic(err)
	}
	return k
}

// Reply answers one datagram from src. It returns the reply (nil when the
// datagram is ignored), what the datagram was and, for a follow-up, whether
// its cookie tag verified. Ignored: anything shorter than the header, a type
// other than 1, a probe whose length does not equal its advertised total,
// whose total is not one of the two probe sizes, or whose text is wrong, and
// a follow-up that is not exactly 32 bytes. Replies are never longer than the
// request, so the endpoint cannot amplify.
func (r *Responder) Reply(req []byte, src netip.AddrPort) (reply []byte, kind Kind, tagOK bool) {
	if len(req) < HeaderLen || binary.LittleEndian.Uint16(req[0:]) != 1 {
		return nil, Ignored, false
	}
	src = netip.AddrPortFrom(src.Addr().Unmap(), src.Port())
	if !src.Addr().Is4() {
		return nil, Ignored, false
	}
	sub := binary.LittleEndian.Uint16(req[2:])
	total := int(binary.LittleEndian.Uint16(req[4:]))
	switch {
	case sub == 3:
		if len(req) != FollowUpLen {
			return nil, Ignored, false
		}
		ok := hmac.Equal(req[4+OffTag:4+OffTag+tagLen], r.tag(req[4:4+OffTag], src))
		// The answer is 36 bytes: header (type 1, subtype 4, length 36, body 0) and 28 zero bytes.
		out := make([]byte, ReplyHeaderLen)
		binary.LittleEndian.PutUint16(out[0:], 1)
		binary.LittleEndian.PutUint16(out[2:], 4)
		binary.LittleEndian.PutUint16(out[4:], ReplyHeaderLen)
		return out, FollowUp, ok
	case sub != 0 || len(req) != total:
		return nil, Ignored, false
	}
	var text string
	switch total {
	case MTULen:
		text, kind = MTUText, MTUProbe
	case NATLen:
		text, kind = NATText, NATProbe
	default:
		return nil, Ignored, false
	}
	if len(req) < HeaderLen+len(text) || string(req[HeaderLen:HeaderLen+len(text)]) != text {
		return nil, Ignored, false
	}
	out := make([]byte, total)
	copy(out, req[:HeaderLen]) // type, subtype, total, body length echoed
	if _, err := rand.Read(out[HeaderLen:OffTag]); err != nil {
		panic(err)
	}
	copy(out[OffTag:], r.tag(out[:OffTag], src))
	ip := src.Addr().As4()
	copy(out[OffIP:], ip[:])
	binary.BigEndian.PutUint16(out[OffPort:], src.Port())
	copy(out[OffText:], text)
	return out, kind, false
}

// tag is the 8-byte truncated HMAC-SHA1 over the 20-byte cookie head, the
// client IPv4 and its port in network order.
func (r *Responder) tag(head []byte, src netip.AddrPort) []byte {
	m := hmac.New(sha1.New, r.Key)
	m.Write(head)
	ip := src.Addr().As4()
	m.Write(ip[:])
	m.Write([]byte{byte(src.Port() >> 8), byte(src.Port())})
	return m.Sum(nil)[:tagLen]
}
