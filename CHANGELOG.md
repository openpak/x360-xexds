# Changelog: x360-xexds

## Unreleased

- 2026-10-16: first cut. UDP endpoint for the Xbox 360 dashboard's XEXDS connection probe: the
  1336-byte "Xbox Live MTU Test" reply, the 200-byte "Xbox Live ICMP Test" NAT-probe reply (28-byte
  HMAC-tagged cookie, the client's IPv4 and port at 0x1C/0x20) and the 32-byte follow-up. Unit tests
  plus `cmd/x360-xexds-probe`. Not deployed, no tag, not yet tried by the emulator itself.
