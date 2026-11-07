# iolite — agent instructions

Re-read `RULES.md` before starting work; it holds the behaviour policy.
`BOOTSTRAP.md` is the project record; the single plan file is forward
work only.

## Conventions

- Go code: 70 columns, 8-char tabs, functions ≤40 lines. `make lint`
  enforces it (`tools/lint`). Long string literals are exempt.
- Commit with the agent key, never push:
  `git -c user.name="Claude (agent)" -c user.email="fatman.uk@gmail.com"
  -c user.signingkey=91D1F5FFE7745FB4 commit -S`
- Column names in `seed/profiles.sql` must match GORM's naming of the
  structs in `internal/model`; the seed runs after `AutoMigrate`.

## Traps

- DHCP replies must go out on the interface the request arrived on:
  bind 0.0.0.0:67, enable `ipv4.FlagInterface` (IP_PKTINFO), drop
  packets whose `IfIndex` is not ours, and reply with `WriteTo` carrying
  the same `IfIndex` to 255.255.255.255:68. Without this, replies on a
  bridge are lost or deadlock routing.
- Firmware PXE ROMs fetch only by TFTP. TFTP serves iPXE binaries chosen
  by option 93 arch; everything else is HTTP after iPXE re-DHCPs with
  user class `iPXE`. QEMU NICs already run iPXE, so a QEMU-only test
  never exercises the TFTP path.
- Install loop: the installer must POST `state=done` to
  `/profiles/<mac>/status`, which disables the profile; otherwise the
  machine reinstalls on every reboot.
- The test VM must have a disk; the legacy one had none.
- libvirt: use `virsh -c qemu:///session`; `qemu:///system` times out.
- iolite binds ports 67/69/80: run with `sudo` (`make run`).
- `legacy/` holds the old build tree and DB (8.7 GB, git-ignored); the
  old Go sources are on branch `mother`: `git show mother:<path>`.
