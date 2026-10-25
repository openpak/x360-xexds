# Deploy: x360-xexds

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

## Layout and releases

`cmd/x360-xexds` main; `cmd/x360-xexds-probe` client; `internal/config`; `internal/xexds` (`Responder.Reply` is the
whole protocol as a pure function, `Server` the receive loop, `Probe` the client cycle). License: AGPL-3.0-only, like the
rest of the OpenPak servers. Releases: tag `v*.*.*` publishes `ghcr.io/openpak/x360-xexds` and deployed hosts auto-update
from it, so nothing is tagged until a deploy is wanted.
