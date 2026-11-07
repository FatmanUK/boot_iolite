package dhcp

import (
	"errors"
	"net"
	"testing"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/iana"
)

var mac = net.HardwareAddr{0x52, 0x54, 0, 0x76, 0xde, 0xe5}

var cfg = Config{
	ServerIP:  net.ParseIP("172.168.16.1"),
	Mask:      net.CIDRMask(16, 32),
	DNS:       []net.IP{net.ParseIP("172.168.16.1")},
	LeaseSecs: 3600,
	HTTPBoot:  "http://172.168.16.1/boot/boot.ipxe",
}

var client = net.ParseIP("172.168.16.32")

func discover(t *testing.T, mods ...dhcpv4.Modifier) *dhcpv4.DHCPv4 {
	t.Helper()
	d, err := dhcpv4.NewDiscovery(mac, mods...)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestBootfile(t *testing.T) {
	x64 := dhcpv4.OptClientArch(iana.EFI_X86_64)
	cases := []struct {
		name string
		mods []dhcpv4.Modifier
		want string
	}{
		{"no arch", nil, "undionly.kpxe"},
		{"bios", []dhcpv4.Modifier{dhcpv4.WithOption(
			dhcpv4.OptClientArch(iana.INTEL_X86PC))},
			"undionly.kpxe"},
		{"efi bc", []dhcpv4.Modifier{dhcpv4.WithOption(
			dhcpv4.OptClientArch(iana.EFI_BC))}, "ipxe.efi"},
		{"efi x64", []dhcpv4.Modifier{dhcpv4.WithOption(
			dhcpv4.OptClientArch(iana.EFI_X86_64))}, "ipxe.efi"},
		{"arm64", []dhcpv4.Modifier{dhcpv4.WithOption(
			dhcpv4.OptClientArch(iana.EFI_ARM64))},
			"ipxe-arm64.efi"},
		{"ipxe", []dhcpv4.Modifier{
			dhcpv4.WithOption(x64),
			dhcpv4.WithUserClass("iPXE", false)},
			cfg.HTTPBoot},
	}
	for _, c := range cases {
		req := discover(t, c.mods...)
		got, err := Bootfile(req, cfg.HTTPBoot)
		if err != nil || got != c.want {
			t.Errorf("%s: got %q, %v; want %q", c.name, got, err,
				c.want)
		}
	}
}

func TestBootfileUnknownArch(t *testing.T) {
	req := discover(t, dhcpv4.WithOption(
		dhcpv4.OptClientArch(iana.DEC_ALPHA)))
	_, err := Bootfile(req, cfg.HTTPBoot)
	if !errors.Is(err, ErrUnknownArch) {
		t.Fatalf("want ErrUnknownArch, got %v", err)
	}
}

func TestOffer(t *testing.T) {
	rep, err := Reply(discover(t), cfg, client, "undionly.kpxe")
	if err != nil {
		t.Fatal(err)
	}
	if rep.MessageType() != dhcpv4.MessageTypeOffer {
		t.Fatalf("type %s", rep.MessageType())
	}
	if !rep.YourIPAddr.Equal(client) {
		t.Errorf("yiaddr %s", rep.YourIPAddr)
	}
	if !rep.ServerIPAddr.Equal(cfg.ServerIP) {
		t.Errorf("siaddr %s", rep.ServerIPAddr)
	}
	if rep.BootFileName != "undionly.kpxe" {
		t.Errorf("file %q", rep.BootFileName)
	}
	if got := rep.BootFileNameOption(); got != "undionly.kpxe" {
		t.Errorf("option 67 %q", got)
	}
	if !rep.ServerIdentifier().Equal(cfg.ServerIP) {
		t.Errorf("option 54 %s", rep.ServerIdentifier())
	}
	if !rep.Router()[0].Equal(cfg.ServerIP) {
		t.Errorf("router %v", rep.Router())
	}
	if len(rep.DNS()) != 1 {
		t.Errorf("dns %v", rep.DNS())
	}
}

func TestRequestAckNak(t *testing.T) {
	off, err := Reply(discover(t), cfg, client, "ipxe.efi")
	if err != nil {
		t.Fatal(err)
	}
	req, err := dhcpv4.NewRequestFromOffer(off)
	if err != nil {
		t.Fatal(err)
	}
	ack, err := Reply(req, cfg, client, "ipxe.efi")
	if err != nil || ack.MessageType() != dhcpv4.MessageTypeAck {
		t.Fatalf("ack: %v %v", ack, err)
	}
	other := net.ParseIP("172.168.16.99")
	nak, err := Reply(req, cfg, other, "ipxe.efi")
	if err != nil || nak.MessageType() != dhcpv4.MessageTypeNak {
		t.Fatalf("nak: %v %v", nak, err)
	}
}

func TestIgnoredTypes(t *testing.T) {
	inf := discover(t, dhcpv4.WithMessageType(
		dhcpv4.MessageTypeInform))
	rep, err := Reply(inf, cfg, client, "undionly.kpxe")
	if err != nil || rep != nil {
		t.Fatalf("inform: %v %v", rep, err)
	}
}
