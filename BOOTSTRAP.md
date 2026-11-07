# iolite — Bootstrap Markdown Manifest

## 1. Current Goal

Build **iolite**: a single Go binary that PXE-boots bare metal and VMs into a tiny
self-built Linux initramfs, which then reads a per-machine profile from the server
and installs an OS onto local disks according to a stored partition layout.

### Next 3 Steps
1. Phase 1a: `internal/model`, `internal/store`, `internal/render` with tests.
2. Phase 1b: `internal/dhcp` (stateless, option 93 arch → bootfile) with tests.
3. Phase 1c: `internal/tftp`, `internal/httpd`, `main`; first manual run.

## 2. State of Play

### Architecture

On the iolite host, currently using virbr0 = 172.168.16.1/16:
DHCP (67): serve a temporary IP, answering only known MAC addresses.
TFTP (69): serve tftpboot directory over tftp.
HTTP (80): serve web root directory over http.

Protocol split (PXE 2.1 spec, RFC 2131/2132, RFC 4578, RFC 1350): the firmware
PXE ROM can only fetch its first stage by TFTP, so TFTP serves the iPXE
binaries (`undionly.kpxe` for BIOS arch 0, `ipxe.efi` for UEFI arch 6/7/9).
iPXE then re-DHCPs with user class `iPXE` and is handed an HTTP URL; kernel,
initrd, profiles and Void packages all go over HTTP. QEMU's NIC ROM is already
iPXE, which is why HTTP-only worked on QEMU but not on metal. HTTPS is a later
phase (iPXE must be built with `DOWNLOAD_PROTO_HTTPS` and `TRUST=ca.crt`).

Database:
profiles.db (SQLite via GORM): Profile → Layout → Template → MBRPartition

The database currently only specifies disk layouts per profiled MAC address.
Currently we only have MBRPartition but GPT and EFI should work too.

Later we'll add an admin web interface to manipulate profiles.db. Also we
should switch to Postgres.

The initrd contains a Linux kernel and statically-compiled versions of busybox,
jq, runit, util-linux and xfsprogs. All stable versions. See the legacy code for
exact versions. Exact compilation methods are quite arcane and detailed in the
contained Ansible project.

The initrd should also do the following:
- start runit as PID 1
- have runit start udhcpc
- have runit start a rescue shell on tty4
- have runit start a build script on tty1, which:
  - fetches `http://<gateway>/<MAC>` and sources it
  - writes `/etc/hostname`
  - decodes the disk layout
  - decodes each disk's sfdisk script
  - pipes each sfdisk script to `sfdisk /dev/<disk>`

### Data model (GORM, SQLite)

- `Profile` — `HardwareAddress` (unique), `IPAddress` (unique), `FQDN` (unique),
  `Distro`, `IsBuildEnabled`, `LayoutID`.
- `Layout` — `Name` (unique); many-to-many to `Template` through `LayoutTemplate`,
  which carries `MapKey` (the device name, e.g. `vda`, `sda`). Hooks `AfterSave` /
  `AfterFind` sync a `map[string]*Template` convenience view.
- `Template` — `Name` (unique), `SectorSize`; has many `MBRPartition` (cascade delete).
- `MBRPartition` — `Start`, `Size` (sectors), `Type` (e.g. `0x83`), `IsBootable`.
- Serialisation to the target: `Template.String(dev)` renders an `sfdisk --dump`
  style script, base64-encoded; `Layout.String()` wraps them as
  `{"layout":[{"<dev>":"<b64>"},...]}`; `Profile.Write` emits a shell env file.

### Build pipeline for the target image (legacy Ansible; replaced by `image/Makefile` in Phase 2)

`tinypxe.bash` → `ansible-playbook -i localhost, tinypxe.yml`. Plays, each cloning
upstream and building **static** binaries into `source/initramfs/`:

| play / role           | tag(s)            | source + version                         |
|-----------------------|-------------------|------------------------------------------|
| `kernel_build`        | never,linux,kernel| linux-stable, latest from kernel.org finger banner; `allnoconfig` + enable list |
| `busybox_build`       | never,busybox     | busybox 1_36_1, `defconfig` + STATIC etc. |
| `xfsprogs_build`      | never,xfsprogs    | xfsprogs 7.1.1 (mkfs.xfs, xfs_repair)    |
| `jq_build`            | never,jq          | jq 1.8.2 with builtin oniguruma          |
| `util-linux_build`    | never,util-linux  | util-linux 2.42.2 (sfdisk only)          |
| `runit_build` + `runit_services_create` | never,runit | runit 2.3.1 (g-pape/runit) |
| `system_files_create` + `initramfs_build` | initramfs,initrd (default) | templates + cpio/gzip via custom module `library/initrd_build.py` |
| `iso_build`           | mkisofs,iso       | isolinux ISO for non-PXE testing |

`never`-tagged plays run only when their tag is passed; a plain run only rebuilds the
initramfs from already-built binaries. Legacy needed setuid `mknod` for `/dev` nodes; the
replacement packs the initramfs with the kernel's `usr/gen_init_cpio`, whose
file list has `nod` lines, so no root and no mknod.

### Decisions made

- One Go binary, no external dnsmasq/tftpd. Module name `iolite`, entrypoint `./main`.
- DHCP answers **only** MACs present in the DB with `is_build_enabled` true. Unknown
  MACs are ignored (this is the "shun unknown MACs" feature, done implicitly).
- Profiles in SQLite via GORM (`glebarez/sqlite`, pure Go, CGO off). PostgreSQL was
  noted as a possible later swap.
- Per-machine profile delivered as a sourceable `/bin/sh` env file at `/<MAC>`.
- Partition layouts are expressed as sfdisk scripts.
- Target image is built from source by `make -C image` (kernel `allnoconfig`
  + explicit enable list; static busybox/runit/sfdisk+blkid/xfsprogs/mke2fs;
  Void static `xbps-install`), not from a distro.
- Branch name `vmother`. CI (GitHub Actions) runs `make test` and builds/pushes a
  Podman image to `ghcr.io/fatmanuk/iolite` on every push to `vmother`.

### Resolved design questions
- What does "install" mean?
  - It means create a persistent self-booting system which starts without using
    the network. The first step is to bootstrap a package system using a
    statically-compiled xbps-install. From there we can install other package
    managers and trample over the xbps package system (or not, if we like xbps).
- MBR only, or GPT + EFI from the start? (Affects data model, kernel config, iPXE
  binaries, bootloader choice.)
  - MBR is good for testing but the future is GPT/EFI. We need everything from the
    start.
- Keep SQLite/GORM, or replace with a plain config file (YAML/TOML) plus a generated
  directory of profile files? The DB has 2 profiles.
  - Keep SQLite/GORM, with an eye on Postgres in future. We need to add a web UI
    to manipulate the database.
- Keep Ansible for the image build, or a Makefile/shell pipeline?
  - Switch to make/shell. Ansible was good to prototype but we have the processes
    nailed now.
- Should the server expose an HTTP API (add/enable profiles, report install status)?
  - This is the web UI mentioned elsewhere. Keep it as simple as possible.
- Serial console and/or tty0?
  - Ideally both but I'm not sure if that's possible. If not, choose tty0.
- Keep the custom callwheel, or use `time.AfterFunc`?
  - Ditch the callwheel.

### Superseded
Ansible build pipeline (now make); `fatgo/callwheel` and the DHCP offer table
(now stateless DHCP: OFFER per DISCOVER, ACK/NAK per REQUEST); `jq` in the
initramfs (profile env file is plain shell). Legacy build tree is in `legacy/`;
legacy Go sources are on branch `mother` (`git show mother:<path>`).

## 3. Dependency Map & Version Log

Go (`go.mod`: `go 1.25.0`; host has go1.26.5):

| module | version | used for |
|---|---|---|
| github.com/insomniacslk/dhcp/dhcpv4 | v0.0.0-20260719225207-c76316d4aa82 | DHCPv4 parse/build |
| golang.org/x/net/ipv4 | v0.57.0 | `PacketConn` with `IP_PKTINFO` control messages |
| github.com/pin/tftp | v2.1.0+incompatible | TFTP read server |
| gorm.io/gorm | v1.31.2 | ORM |
| github.com/glebarez/sqlite | v1.11.0 | pure-Go SQLite driver for GORM |

Host toolchain: Go, make, gcc, git, cpio, xorriso/mkisofs, isolinux
(`/usr/lib/ISOLINUX/isolinux.bin`, `/usr/lib/syslinux/modules/bios/ldlinux.c32`),
autotools, curl, xz, qemu + OVMF (`/usr/share/OVMF/OVMF_CODE_4M.fd`), libvirt
(`qemu:///session`), podman; for xfsprogs: `libinih-dev liburcu-dev uuid-dev
libblkid-dev`. Boot ROMs are built from iPXE source into `tftpboot/` (ignored).

### Version Log (pinned in `image/versions.mk`)

| component | version | source |
|---|---|---|
| linux | v7.1.7 | git.kernel.org linux-stable |
| busybox | 1_36_1 | github.com/vda-linux/busybox_mirror |
| runit | v2.3.1 | github.com/g-pape/runit |
| util-linux | v2.42.2 | github.com/util-linux/util-linux |
| xfsprogs | v7.1.1 | kernel.googlesource.com xfsprogs-dev |
| e2fsprogs | v1.47.4 | git.kernel.org e2fsprogs (confirm on first build) |
| xbps-static | pin on first fetch | repo-default.voidlinux.org/static |
| iPXE | pin on first clone | github.com/ipxe/ipxe |

## 4. Golden Code Blocks

Only the parts worth carrying forward. Everything else can be rewritten.

### 4.1 Data model (`internal/iolite/{profile,layout,template,mbrPartition}.go`)

```go
type Profile struct {
	ID              uint    `gorm:"primaryKey"`
	LayoutID        uint    `gorm:"not null;index"`
	Layout          *Layout `gorm:"foreignKey:LayoutID"`
	HardwareAddress string  `gorm:"unique;not null"`
	IPAddress       string  `gorm:"unique;not null"`
	FQDN            string  `gorm:"unique;not null"`
	Distro          string
	IsBuildEnabled  bool
}

type Layout struct {
	ID         uint                 `gorm:"primaryKey"`
	Name       string               `gorm:"unique;not null"`
	TemplatesM []*Template          `gorm:"many2many:layout_templates;"`
	Templates  map[string]*Template `gorm:"-"` // keyed by device name; synced via hooks
}

// Join table; MapKey is the device name. No OnDelete:CASCADE so a
// shared Template survives Layout deletion. GORM needs this type
// registered in AutoMigrate even though nothing references it directly.
type LayoutTemplate struct {
	ID         uint   `gorm:"primaryKey"`
	LayoutID   uint   `gorm:"not null;index"`
	TemplateID uint   `gorm:"not null;index"`
	MapKey     string `gorm:"not null;index"`
	Template   *Template
}

type Template struct {
	ID         uint            `gorm:"primaryKey"`
	Name       string          `gorm:"unique;not null"`
	SectorSize uint16          `gorm:"not null"`
	Partitions []*MBRPartition `gorm:"constraint:OnDelete:CASCADE"`
}

type MBRPartition struct {
	ID         uint   `gorm:"primaryKey"`
	TemplateID uint   `gorm:"not null;index"`
	Start      uint64 `gorm:"not null"` // sectors
	Size       uint64 `gorm:"not null"` // sectors
	Type       string `gorm:"not null"` // e.g. "0x83"
	IsBootable bool   `gorm:"not null"`
}
```

Hooks on `Layout`:

```go
// AfterSave persists each Template and upserts a join row per map key.
func (l *Layout) AfterSave(tx *gorm.DB) error {
	for key, tmpl := range l.Templates {
		if err := tx.Save(tmpl).Error; err != nil {
			return fmt.Errorf("save template %q: %w", key, err)
		}
		join := LayoutTemplate{LayoutID: l.ID, TemplateID: tmpl.ID, MapKey: key}
		if err := tx.Where("layout_id = ? AND map_key = ?", l.ID, key).
			Assign(join).FirstOrCreate(&LayoutTemplate{}).Error; err != nil {
			return fmt.Errorf("link template %q: %w", key, err)
		}
	}
	return nil
}

// AfterFind rebuilds the map from the join table's MapKey.
func (l *Layout) AfterFind(tx *gorm.DB) error {
	var joins []LayoutTemplate
	if err := tx.Where("layout_id = ?", l.ID).Find(&joins).Error; err != nil {
		return err
	}
	byID := make(map[uint]*Template, len(l.TemplatesM))
	for _, t := range l.TemplatesM {
		byID[t.ID] = t
	}
	l.Templates = make(map[string]*Template, len(joins))
	for _, j := range joins {
		if t, ok := byID[j.TemplateID]; ok {
			l.Templates[j.MapKey] = t
		}
	}
	return nil
}
```

Load with `db.Preload("Layout.TemplatesM.Partitions").Find(&allProfiles)`;
`AutoMigrate(&Profile{}, &Layout{}, &LayoutTemplate{}, &Template{}, &MBRPartition{})`.

Example decoded sfdisk script produced for `sda` of layout "Plain VM":

```
label: dos
device: /dev/sda
unit: sectors
sector-size: 512

/dev/sda1 : start=          63, size=    10485697, type=0x83, bootable
```

### 4.2 Kernel config

`make allnoconfig`, then `./scripts/config --enable CONFIG_<x>` for each, then
`make olddefconfig`, `make`, stage `arch/x86/boot/bzImage` as `vmlinuz`.

```yaml
kernel_enable_options:
  - '64BIT'               # required for 64bit support
  - 'BINFMT_ELF'          # required to run, y'know, stuff
  - 'BINFMT_SCRIPT'       # required to execute runit stage scripts
  - 'EARLY_PRINTK'        # required to debug early kernel messages
  - 'TTY'                 # required to see/type anything
  - 'SERIAL_8250'         # required for serial console
  - 'SERIAL_8250_CONSOLE' # required for serial console
  - 'VGA_CONSOLE'         # required to render text on VGA hardware
  - 'BLK_DEV_INITRD'      # required for initrd to work
  - 'DEVTMPFS'            # required for tmpfs
  - 'DEVTMPFS_MOUNT'      # required for tmpfs
  - 'TMPFS'               # required for tmpfs
  - 'TMPFS_POSIX_ACL'     # not required but highly recommended
  - 'TMPFS_XATTR'         # not required but highly recommended
  - 'VT'                  # required for virtual terminals
  - 'RTC_CLASS'           # required for hwclock
  - 'RTC_INTF_DEV'        # required for hwclock
  - 'RTC_DRV_CMOS'        # required for hwclock
  - 'NET'                 # required for networking
  - 'INET'                # required for networking
  - 'NETDEVICES'          # required for networking
  - 'ETHERNET'            # required for networking
  - 'IP_PNP'              # required for kernel IP assignment at boot
  - 'PM'                  # required for ACPI
  - 'ACPI'                # required for power-off ACPI events
  - 'PCI'                 # required to enumerate hardware
  - 'VIRTIO_MENU'         # required for the virtio subsystem
  - 'VIRTIO_PCI'          # required to find virtio devices
  - 'VIRTIO_NET'          # required for virtio networking
  - 'E1000'               # required for Intel PRO/1000 GbE
  - 'E1000E'              # required for PCIe Intel GbE NICs
  - '8139CP'              # required for older Realtek NICs
  - 'R8169'               # required for PCIe Realtek GbE NICs
  - 'IGC'                 # required for modern Intel 2.5GbE
  - 'IGB'                 # required for modern Intel GbE
  - 'PACKET'              # required for raw sockets
  - 'INPUT'               # required for the input subsystem
  - 'INPUT_KEYBOARD'      # required for keyboards
  - 'SERIO'               # required for serial I/O bus
  - 'SERIO_I8042'         # required for the keyboard controller
  - 'KEYBOARD_ATKBD'      # required for the keyboard protocol
  - 'BLOCK'               # required for the block device subsystem
  - 'BLK_DEV'             # required for generic block device support
  - 'PARTITION_ADVANCED'  # required for reading disk partition tables
  - 'MSDOS_PARTITION'     # required for reading traditional MBRs
  - 'EFI_PARTITION'       # required for modern GPT partition tables
  - 'VIRTIO_BLK'          # required for VirtIO block device driver
  - 'ATA'                 # required for the ATA/SATA subsystem
  - 'SATA_AHCI'           # required for AHCI SATA
  - 'BLK_DEV_SD'          # required for ATA as SCSI (/dev/sd)
  - 'EXT4_FS'             # required for ext2, ext3, and ext4
  - 'FS_MBCACHE'          # auto-selected by ext4
  - 'JBD2'                # auto-selected by ext4
  - 'XFS_FS'              # required for XFS
  - 'LIBCRC32C'           # auto-selected by xfs
  - 'CRYPTO_CRC32C'       # auto-selected by xfs
  - 'EXPORTFS'            # auto-selected by xfs
  - 'SCSI'                # required for Core SCSI subsystem
  - 'SCSI_LOWLEVEL'       # required for low-level SCSI drivers
  - 'SCSI_VIRTIO'         # required for VirtIO-SCSI
  - 'SCSI_LSI53C895A'     # required for QEMU's legacy SCSI controller
  - 'USB_SUPPORT'         # required for USB menu/subsystem
  - 'USB'                 # required for Core USB driver
  - 'USB_PCI'             # required for USB controllers on PCI bus
  - 'USB_XHCI_HCD'        # required for USB 3.0 (xHCI) controller
  - 'USB_EHCI_HCD'        # required for USB 2.0 (EHCI) controller
  - 'USB_STORAGE'         # required for USB flash/external drives
```

Kernel source: `git://git.kernel.org/pub/scm/linux/kernel/git/stable/linux-stable.git`,
tag `v<latest stable>` parsed from `https://www.kernel.org/finger_banner`.

### 4.3 Busybox config

`make defconfig`; enable `STATIC MOUNT UMOUNT FDISK MKE2FS MKFS_EXT2`; disable
`TC FEATURE_TC_INGRESS`; `make oldconfig`; `make install`; copy `busybox` to
`/bin/busybox` and create ~330 hand-listed symlinks (`/linuxrc`, `/bin/*`, `/sbin/*`,
`/usr/bin/*`, `/usr/sbin/*`) pointing at it. Regenerate the list with
`busybox --list-full` rather than hand-maintaining it. Source:
`https://github.com/vda-linux/busybox_mirror.git` tag `1_36_1`.

### 4.4 Static tool builds

- runit 2.3.1 (`https://github.com/g-pape/runit`): write `gcc -static` to
  `src/conf-ld`, run `package/compile`, copy `chpst runit runit-init runsv runsvchdir
  runsvdir sv svlogd utmpset` to `/bin`; `/init → bin/runit-init`,
  `/sbin/runit → ../bin/runit`.
- util-linux 2.42.2: `./autogen.sh && ./configure LDFLAGS=-static
  --enable-static-programs=sfdisk --disable-irqtop --without-python && make`,
  strip `sfdisk.static` → `/usr/bin/sfdisk`.
- xfsprogs 7.1.1 (`https://kernel.googlesource.com/pub/scm/fs/xfs/xfsprogs-dev`):
  `make configure LOCAL_CONFIGURE_OPTIONS='--enable-static --disable-shared'`,
  `make DEBUG=-DNDEBUG OPTIMIZER=-O2`, then hand-link:
  `gcc -static -o mkfs.xfs_static mkfs/xfs_mkfs.o $(find mkfs/ -maxdepth 1 -name "*.o" ! -name "xfs_mkfs.o") libfrog/.libs/libfrog.a libxfs/.libs/libxfs.a -lpthread -luuid -lblkid -lurcu -linih`
  and likewise `xfs_repair` with `repair/*.o libxcmd/.libs/libxcmd.a libxlog/.libs/libxlog.a`.
- jq 1.8.2: `autoreconf -i && ./configure --with-oniguruma=builtin && make LDFLAGS=-all-static`.

### 4.5 runit stages and services

/etc/runit/1:
```sh
#!/bin/sh
echo "Stage 1: System boot"
/bin/mount -t proc none /proc
/bin/mount -t sysfs none /sys
/bin/mount -t devtmpfs none /dev 2>/dev/null
/bin/mkdir -p /dev/pts /dev/shm
/bin/mount -t devpts devpts /dev/pts
/bin/mount -t tmpfs tmpfs /dev/shm
/sbin/hwclock -s
/sbin/ip link set dev lo up
exit 0
```

/etc/runit/2:
```sh
#!/bin/sh
echo "Stage 2: System supervision"
exec /usr/bin/env - PATH=$PATH /bin/runsvdir -P /var/service
```

/etc/runit/3:
```sh
#!/bin/sh
echo "Stage 3: System shutdown"
/bin/sv force-stop /var/service/*
/bin/sv exit /var/service/*
/bin/sync
/bin/umount -a -r
```

/sbin/reboot:
```sh
#!/bin/sh
/usr/bin/install /dev/null /etc/runit/reboot -m0100
/usr/bin/install /dev/null /etc/runit/stopit -m0100
/bin/kill -CONT 1
```

/etc/sv/udhcpc/run:
```sh
#!/bin/sh
LEASE_S=3600
while true; do
	/sbin/udhcpc -i eth0
	/bin/sleep ${LEASE_S}
done
```

Every runit service gets /etc/sv/<name>/log/run:
```sh
#!/bin/sh
exec /usr/sbin/svlogd -tt /var/log/<name>
```

/etc/passwd:
```
root:x:0:0:root:/root:/bin/sh
```

/etc/hostname:
```
<hostname>
```

/usr/share/udhcpc/default.script:
```sh
#!/bin/sh
case "$1" in
    deconfig)
        ip addr flush dev $interface
        ip link set dev $interface up
        ;;
    bound|renew)
        ip addr add $ip/$mask dev $interface
        [ -n "$router" ] && ip route add default via $router dev $interface
        if [ -n "$dns" ]; then
            > /etc/resolv.conf
            for i in $dns; do echo "nameserver $i" >> /etc/resolv.conf; done
        fi
        ;;
esac
```

### 4.6 initramfs packing

Walk the stage dir, feed NUL-separated paths to
`cpio --null -ov --format=newc --owner root:root`, gzip level 9 to
`output/boot/initrd.img`. Device nodes created beforehand with `mknod -m 0666`:
`/dev/null c 1 3`, `/dev/zero c 1 5`, `/dev/random c 1 8`, `/dev/console c 5 1`,
`/dev/tty c 5 0`. (Alternative avoiding setuid mknod: the kernel's
`usr/gen_init_cpio` with a file list containing `nod` lines.)

### 4.7 ISO fallback (`isolinux.cfg`)

```
DEFAULT smallest_linux
LABEL smallest_linux
  KERNEL /boot/vmlinuz
  INITRD /boot/initrd.img
  APPEND console=ttyS0 earlyprintk=ttyS0 tsc=reliable mitigations=off
```

`mkisofs -o smallest.iso -b boot/isolinux/isolinux.bin -c boot/isolinux/boot.cat
-no-emul-boot -boot-load-size 4 -boot-info-table .`

### 4.8 Test VM and network (libvirt)

```xml
<network>
  <name>build-net</name>
  <bridge name='virbr0' stp='on' delay='0'/>
  <ip address='172.168.16.1' netmask='255.255.0.0'/>
</network>
```

## 5. Tested & Passing Status Confirmation

None yet.
