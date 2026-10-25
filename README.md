# Xbox 360 XEXDS Server

OpenPak's Xbox 360 XEXDS Server.

| | |
| --- | --- |
| Network | Xbox 360 |
| Role | XEXDS connection probe (MTU and NAT tests) for the 360 dashboard's Test Xbox Live Connection, UDP 3074 |
| Image | `ghcr.io/openpak/x360-xexds` |
| Licence | AGPL-3.0-only |

## What it serves

| Endpoint | What |
| --- | --- |
| UDP 3074 (`xexds.xboxlive.com`) | Connection probe: MTU test, NAT (ICMP) test and follow-up, decides the dashboard's Internet line |

## Run

```sh
go run ./cmd/x360-xexds   # listens on UDP :3074
```

## Docs

- [Protocol](docs/protocol.md)
- [Evidence](docs/evidence.md)
- [Deploy](docs/deploy.md)
- [Notes](docs/notes.md)

## Licence

AGPL-3.0-only. See [LICENSE](LICENSE).
