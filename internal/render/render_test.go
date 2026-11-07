package render

import (
	"encoding/base64"
	"strings"
	"testing"

	"iolite/internal/model"
)

func gptTemplate() *model.Template {
	return &model.Template{
		Name: "gpt-efi-swap-root", Label: "gpt", SectorSize: 512,
		Partitions: []*model.Partition{
			{Start: 2048, Size: 1048576, Type: "U", Name: "efi",
				Filesystem: "vfat", MountPoint: "/boot/efi",
				MountOptions: "defaults"},
			{Start: 1050624, Size: 4194304, Type: "S",
				Name: "swap", Filesystem: "swap"},
			{Start: 5244928, Type: "L", Name: "root",
				Filesystem: "ext4", MountPoint: "/",
				MountOptions: "defaults"},
		},
	}
}

func dosTemplate() *model.Template {
	return &model.Template{
		Name: "dos-swap-root", Label: "dos", SectorSize: 512,
		Partitions: []*model.Partition{
			{Start: 2048, Size: 4194304, Type: "82",
				Filesystem: "swap"},
			{Start: 4196352, Type: "83", Filesystem: "ext4",
				MountPoint: "/", MountOptions: "defaults",
				IsBootable: true},
		},
	}
}

func TestPartDev(t *testing.T) {
	cases := map[string]string{
		"sda": "sda2", "vda": "vda2", "nvme0n1": "nvme0n1p2",
		"mmcblk0": "mmcblk0p2",
	}
	for dev, want := range cases {
		if got := PartDev(dev, 2); got != want {
			t.Errorf("PartDev(%q) = %q, want %q", dev, got, want)
		}
	}
}

func TestSfdiskDos(t *testing.T) {
	want := "label: dos\n" +
		"device: /dev/vda\n" +
		"unit: sectors\n" +
		"sector-size: 512\n\n" +
		"/dev/vda1 : start=2048, size=4194304, type=82\n" +
		"/dev/vda2 : start=4196352, type=83, bootable\n"
	if got := Sfdisk("vda", dosTemplate()); got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestSfdiskGpt(t *testing.T) {
	got := Sfdisk("nvme0n1", gptTemplate())
	wantLines := []string{
		"label: gpt",
		`/dev/nvme0n1p1 : start=2048, size=1048576, type=U, name="efi"`,
		`/dev/nvme0n1p3 : start=5244928, type=L, name="root"`,
	}
	for _, l := range wantLines {
		if !strings.Contains(got, l+"\n") {
			t.Errorf("missing %q in:\n%s", l, got)
		}
	}
}

func TestProfileEnv(t *testing.T) {
	p := &model.Profile{
		HardwareAddress: "52:54:00:76:de:e5",
		IPAddress:       "172.168.16.32",
		FQDN:            "glarg01.dreamtrack.net",
		Distro:          "void",
		Packages:        "it's",
		Layout: &model.Layout{
			Templates: map[string]*model.Template{
				"vdb": dosTemplate(),
				"vda": gptTemplate(),
			},
		},
	}
	got := ProfileEnv(p, Defaults{Repo: "http://h/void/current"})
	want := []string{
		"#!/bin/sh\n",
		"HOSTNAME='glarg01.dreamtrack.net'\n",
		"REPO='http://h/void/current'\n",
		`PACKAGES='it'\''s'` + "\n",
		"DISKS='vda vdb'\n",
		"PARTS='vda3:ext4:/:defaults vdb2:ext4:/:defaults " +
			"vda1:vfat:/boot/efi:defaults vda2:swap:: " +
			"vdb1:swap::'\n",
	}
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("missing %q in:\n%s", w, got)
		}
	}
	enc := base64.StdEncoding.EncodeToString(
		[]byte(Sfdisk("vda", gptTemplate())))
	if !strings.Contains(got, "SFDISK_vda='"+enc+"'\n") {
		t.Errorf("SFDISK_vda wrong in:\n%s", got)
	}
}

func TestBootScript(t *testing.T) {
	got := BootScript("172.168.16.1", "console=tty0")
	want := "#!ipxe\necho Bootstrapping iolite...\n" +
		"kernel http://172.168.16.1/boot/vmlinuz console=tty0\n" +
		"initrd http://172.168.16.1/boot/initrd.img\nboot\n"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}
