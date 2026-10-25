# Notes: x360-xexds

## Background

OpenPak's Xbox 360 `xexds.xboxlive.com` endpoint. Today it answers one thing: the connection probe the
360 dashboard (firmware 17559) sends over UDP port 3074 during **Network Settings -> Test Xbox Live
Connection**, which decides the **Internet** line of that page. Microsoft's own host no longer answers those
datagrams, so a real 360 on today's Internet fails that step with "Internet: Failed"; this service makes it pass.

## Naming rule

Naming rule: every OpenPak Xbox 360 service is called `x360-<thing>`. Sibling: [`x360-notice`](https://github.com/openpak/x360-notice).

## Architecture

Architecture: **a real Xbox 360 and the Wharf emulator hit the same endpoint.** Both resolve
`xexds.xboxlive.com`; the name is redirected to this service by DNS (OpenPak's resolver for consoles, the
`nic_dns_hosts` setting for Wharf). There is no emulator-only path, port or packet.

## Status

Status, 2026-10-16: the reply layout is derived from the dashboard's own code (OpenPak's decompile and live
reads, no Microsoft capture, no Microsoft code). The firmware's connectivity test shows **Internet: Connected**
against the stand-in with the same layout (private notes, mode `xexds`, two live runs in Wharf).
This Go server is verified by unit tests,
and by `x360-xexds-probe` against this server and against that stand-in. It has **not** yet been pointed at by the emulator itself, not tried by a
real console, and not deployed.
