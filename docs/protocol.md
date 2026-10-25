# Protocol: x360-xexds

## Protocol

All multi-byte header fields are little-endian, except the mapped port. Source of truth:
internal research notes, "XEXDS probe reply layout".

Request: `u16 type (1)`, `u16 subtype`, `u16 total length`, `u16 body length (total - 36)`, then a NUL-terminated text.

| Probe | subtype | total length | text at offset 8 |
|---|---|---|---|
| MTU | 0 | 1336 (0x538) | `Microsoft Corporation Xbox Live MTU Test\0` (41 bytes) |
| NAT (ICMP) | 0 | 200 | `Microsoft Corporation Xbox Live ICMP Test\0` (42 bytes) |
| follow-up | 3 | 32 | none: `01 00 03 00` + 28 cookie bytes |

Reply to the two type-1 probes, exactly as long as the request:

| Offset | Size | Content |
|---|---|---|
| 0x00 | 20 | the request header (8 bytes) + 12 random bytes |
| 0x14 | 8 | HMAC-SHA1 (key `X360_XEXDS_KEY`) over the 20 bytes above + client IPv4 + client port, truncated |
| 0x1C | 4 | the client's IPv4 address as this server sees it |
| 0x20 | 2 | the client's source port, network order |
| 0x22 | 2 | zero |
| 0x24 | text | the request text, zero padded to the total length |

The dashboard checks only the length, the text at 0x24, and that every NAT-probe reply carries the same mapped
address. The first 28 bytes are an opaque cookie it hands back verbatim in the follow-up. A full test is one MTU
probe, 6 NAT probes and 5 follow-ups. The follow-up gets a 36-byte answer (`01 00 04 00 24 00 00 00` + 28 zero bytes);
the dashboard accepts any answer and also a timeout. A follow-up whose cookie tag does not verify is still answered
and the result is logged only (debug level).

Inputs that do not match are ignored silently: a probe whose datagram length differs from its advertised total,
an unknown total, a wrong text, a follow-up not exactly 32 bytes, a type other than 1, an IPv6 source.
Replies are never longer than the request, so the endpoint cannot be used for amplification.
