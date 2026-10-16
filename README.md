# x360-xexds

OpenPak's Xbox 360 `xexds.xboxlive.com` endpoint. Today it answers one thing: the connection probe the
360 dashboard (firmware 17559) sends over UDP port 3074 during **Network Settings -> Test Xbox Live
Connection**, which decides the **Internet** line of that page. Microsoft's own host no longer answers those
datagrams, so a real 360 on today's Internet fails that step with "Internet: Failed"; this service makes it pass.

Naming rule: every OpenPak Xbox 360 service is called `x360-<thing>`. Sibling: [`x360-notice`](../x360-notice).

Architecture: **a real Xbox 360 and the Wharf emulator hit the same endpoint.** Both resolve
`xexds.xboxlive.com`; the name is redirected to this service by DNS (OpenPak's resolver for consoles, the
`nic_dns_hosts` setting for Wharf). There is no emulator-only path, port or packet.

Status, 2026-10-16: the reply layout is derived from the dashboard's own code (OpenPak's decompile and live
reads, no Microsoft capture, no Microsoft code). The firmware's connectivity test shows **Internet: Connected**
against the stand-in with the same layout (`../scratch/wharf/udpcap.py`, mode `xexds`, two live runs in Wharf).
This Go server is verified by unit tests,
and by `x360-xexds-probe` against this server and against that stand-in. It has **not** yet been pointed at by the emulator itself, not tried by a
real console, and not deployed.

## What XEXDS is

`xexds.xboxlive.com` is one of the Xbox Live hostnames the 360 firmware resolves (the XAM service list in
[`docs/xbox360/wharf-base-plan.md`](../docs/xbox360/wharf-base-plan.md) names it next to `XEAS`, `XEMACS` and
`XETGS`). What OpenPak's documents establish about it:

- The **dashboard** contacts it first during the connectivity test (this repo's only job).
- The **Live logon engine** inside `xam.xex` (XNetLogon, socket on port 0x4E9) is documented to contact it too, in the
  step after this one. That traffic is **not implemented**.
- What the acronym stands for and everything else the real service did is not established, and is not guessed here.

Implemented: only the MTU and NAT probe. Everything else, including the XNetLogon contact, is not yet.

Protocol knowledge comes only from OpenPak's own documents and the firmware's observed behaviour; see the
[clean-room policy](../docs/clean-room-policy.md).

## Protocol

All multi-byte header fields are little-endian, except the mapped port. Source of truth:
[`docs/xbox360/wharf-base-plan.md`](../docs/xbox360/wharf-base-plan.md), "XEXDS probe reply layout".

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

## Run

```sh
cp example.env .env
set -a; source .env; set +a
go run ./cmd/x360-xexds                    # listens on UDP :3074
go run ./cmd/x360-xexds-probe 127.0.0.1:3074
```

`x360-xexds-probe host:port` sends the dashboard's cycle and prints one `PASS`/`FAIL` line per step and a final
`RESULT PASS|FAIL`; exit status 0 only when every step passed. Flags: `-timeout` (per reply, default 2s), `-tries`
(sends per request, default 3). A follow-up with no answer is reported as a note, not a failure.

Source ports at or below 1023 need privileges; the default port 3074 does not. Whoever deploys this must keep the
client's real source address and port visible to the process (host networking, or a published port that does not
rewrite them), because the server echoes them back and the dashboard compares replies against each other.

```sh
go test ./...        # no network beyond loopback UDP
go vet ./... && gofmt -l .
docker build -t x360-xexds .
```

## Configuration

| Variable | Required | Default | Meaning |
|---|---|---|---|
| `X360_XEXDS_ADDR` | no | `:3074` | UDP listen address |
| `X360_XEXDS_KEY` | no | random per process | cookie HMAC key, at least 16 bytes; a restart with a random key only makes in-flight cookies fail the non-fatal check |
| `X360_XEXDS_DEBUG` | no | `false` | log every answered datagram |

## Layout

`cmd/x360-xexds` main; `cmd/x360-xexds-probe` client; `internal/config`; `internal/xexds` (`Responder.Reply` is the
whole protocol as a pure function, `Server` the receive loop, `Probe` the client cycle). License: AGPL-3.0-only, like the
rest of the OpenPak servers. Releases: tag `v*.*.*` publishes `ghcr.io/openpak/x360-xexds` and deployed hosts auto-update
from it, so nothing is tagged until a deploy is wanted.
