# iolite

[![Test](https://github.com/FatmanUK/boot_iolite/actions/workflows/test.yml/badge.svg?branch=vmother)](https://github.com/FatmanUK/boot_iolite/actions/workflows/test.yml)
[![Build](https://github.com/FatmanUK/boot_iolite/actions/workflows/build.yml/badge.svg?branch=vmother)](https://github.com/FatmanUK/boot_iolite/actions/workflows/build.yml)

Single-binary DHCP + TFTP + HTTP server that PXE-boots known machines
into a tiny self-built Linux initramfs, which partitions their disks
from a stored layout and installs Void Linux.

- `main/` — entrypoint; `internal/` — server packages.
- `image/` — `make -C image` builds kernel, initramfs and iPXE.
- `seed/profiles.sql` — example profiles and disk layouts.
- `test/` — libvirt definitions for BIOS and EFI test VMs.

See `BOOTSTRAP.md` for the project record and `CLAUDE.md` for the
working rules.

```
make lint test build
make run ARGS="-iface virbr0 -ip 172.168.16.1/16"
```
