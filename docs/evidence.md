# Evidence: x360-xexds

## What XEXDS is

`xexds.xboxlive.com` is one of the Xbox Live hostnames the 360 firmware resolves (the XAM service list in
internal research notes names it next to `XEAS`, `XEMACS` and
`XETGS`). What OpenPak's documents establish about it:

- The **dashboard** contacts it first during the connectivity test (this repo's only job).
- The **Live logon engine** inside `xam.xex` (XNetLogon, socket on port 0x4E9) is documented to contact it too, in the
  step after this one. That traffic is **not implemented**.
- What the acronym stands for and everything else the real service did is not established, and is not guessed here.

Implemented: only the MTU and NAT probe. Everything else, including the XNetLogon contact, is not yet.

Protocol knowledge comes only from OpenPak's own documents and the firmware's observed behaviour; see the
[clean-room policy](https://docs.openpak.org/contributors/clean-room/).
