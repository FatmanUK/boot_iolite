// Package render turns model rows into the text the target machine
// consumes: sfdisk scripts, the profile env file and boot.ipxe.
// Output is deterministic so it can be diffed and golden-tested.
package render

import (
	"encoding/base64"
	"fmt"
	"sort"
	"strings"

	"iolite/internal/model"
)

// Defaults fills profile fields left empty in the database.
type Defaults struct {
	Repo string
}

// PartDev names partition n of dev the way the kernel does:
// sda → sda1, nvme0n1 → nvme0n1p1.
func PartDev(dev string, n int) string {
	if dev == "" {
		return fmt.Sprintf("%d", n)
	}
	last := dev[len(dev)-1]
	if last >= '0' && last <= '9' {
		return fmt.Sprintf("%sp%d", dev, n)
	}
	return fmt.Sprintf("%s%d", dev, n)
}

// Sfdisk renders an sfdisk(8) script for one disk.
func Sfdisk(dev string, t *model.Template) string {
	var b strings.Builder
	fmt.Fprintf(&b, "label: %s\ndevice: /dev/%s\nunit: sectors\n",
		t.Label, dev)
	fmt.Fprintf(&b, "sector-size: %d\n\n", t.SectorSize)
	for i, p := range t.Partitions {
		b.WriteString(partLine(dev, i+1, t.Label, p))
	}
	return b.String()
}

func partLine(dev string, n int, label string,
	p *model.Partition) string {
	fields := []string{fmt.Sprintf("start=%d", p.Start)}
	if p.Size > 0 {
		fields = append(fields, fmt.Sprintf("size=%d", p.Size))
	}
	fields = append(fields, "type="+p.Type)
	if label == "gpt" && p.Name != "" {
		fields = append(fields, fmt.Sprintf("name=%q", p.Name))
	}
	if p.IsBootable {
		fields = append(fields, "bootable")
	}
	return fmt.Sprintf("/dev/%s : %s\n", PartDev(dev, n),
		strings.Join(fields, ", "))
}

// ProfileEnv renders the sourceable shell file served at
// /profiles/<mac>. SFDISK_<dev> holds a base64 sfdisk script per
// disk; PARTS lists dev:fs:mount:opts in mount order, swap last.
func ProfileEnv(p *model.Profile, d Defaults) string {
	repo := p.Repo
	if repo == "" {
		repo = d.Repo
	}
	devs := devices(p.Layout)
	var b strings.Builder
	b.WriteString("#!/bin/sh\n")
	put(&b, "HOSTNAME", p.FQDN)
	put(&b, "MAC", p.HardwareAddress)
	put(&b, "IP", p.IPAddress)
	put(&b, "DISTRO", p.Distro)
	put(&b, "REPO", repo)
	put(&b, "PACKAGES", p.Packages)
	put(&b, "AUTHORIZED_KEYS", p.AuthorizedKeys)
	put(&b, "ROOT_PASSWORD_HASH", p.RootPasswordHash)
	put(&b, "DISKS", strings.Join(devs, " "))
	for _, dev := range devs {
		script := []byte(Sfdisk(dev, p.Layout.Templates[dev]))
		enc := base64.StdEncoding.EncodeToString(script)
		put(&b, "SFDISK_"+dev, enc)
	}
	put(&b, "PARTS", strings.Join(parts(p.Layout, devs), " "))
	return b.String()
}

// put writes a single-quoted shell assignment.
func put(b *strings.Builder, key, val string) {
	val = strings.ReplaceAll(val, "'", `'\''`)
	fmt.Fprintf(b, "%s='%s'\n", key, val)
}

func devices(l *model.Layout) []string {
	if l == nil {
		return nil
	}
	devs := make([]string, 0, len(l.Templates))
	for d := range l.Templates {
		devs = append(devs, d)
	}
	sort.Strings(devs)
	return devs
}

type part struct {
	dev, fs, mount, opts string
}

// parts lists filesystems to create: mounts by depth, then swap.
func parts(l *model.Layout, devs []string) []string {
	var ps []part
	for _, dev := range devs {
		for i, p := range l.Templates[dev].Partitions {
			if p.Filesystem == "" || p.Filesystem == "none" {
				continue
			}
			ps = append(ps, part{
				dev:   PartDev(dev, i+1),
				fs:    p.Filesystem,
				mount: p.MountPoint,
				opts:  p.MountOptions,
			})
		}
	}
	sort.SliceStable(ps, func(i, j int) bool {
		return less(ps[i], ps[j])
	})
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = strings.Join(
			[]string{p.dev, p.fs, p.mount, p.opts}, ":")
	}
	return out
}

func less(a, b part) bool {
	da, db := depth(a.mount), depth(b.mount)
	if da != db {
		return da < db
	}
	return a.mount < b.mount
}

// depth orders "/" before "/boot" before "/boot/efi"; unmounted
// filesystems (swap) sort last.
func depth(mount string) int {
	if mount == "" {
		return 1 << 30
	}
	return strings.Count(mount, "/")
}

// BootScript renders boot.ipxe for iPXE clients.
func BootScript(host, kernelArgs string) string {
	return "#!ipxe\n" +
		"echo Bootstrapping iolite...\n" +
		fmt.Sprintf("kernel http://%s/boot/vmlinuz %s\n",
			host, kernelArgs) +
		fmt.Sprintf("initrd http://%s/boot/initrd.img\n", host) +
		"boot\n"
}
