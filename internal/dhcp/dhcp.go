// Package dhcp answers DHCPv4 for known MAC addresses only and hands
// each client the right next stage: an iPXE binary by TFTP for
// firmware PXE ROMs (chosen by RFC 4578 option 93), or the HTTP boot
// script when the client is already iPXE (user class option 77).
//
// The server is stateless: every DISCOVER gets an OFFER and every
// REQUEST gets an ACK when the requested address matches the
// profile, else a NAK.
package dhcp

import (
	"errors"
	"fmt"
	"net"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/iana"
	"golang.org/x/net/ipv4"
)

const maxMTU = 1500

// Config is what the server needs to answer.
type Config struct {
	Interface string
	ServerIP  net.IP
	Mask      net.IPMask
	DNS       []net.IP
	LeaseSecs uint32
	HTTPBoot  string // URL of boot.ipxe for iPXE clients
}

// Lookup returns the IP for a MAC, or false if unknown or disabled.
type Lookup func(mac net.HardwareAddr) (net.IP, bool)

// ErrUnknownArch reports an option 93 value we have no binary for.
var ErrUnknownArch = errors.New("unknown client architecture")

// Bootfile picks what the client should load next.
func Bootfile(req *dhcpv4.DHCPv4, httpBoot string) (string, error) {
	for _, uc := range req.UserClass() {
		if uc == "iPXE" {
			return httpBoot, nil
		}
	}
	arch := iana.INTEL_X86PC
	if a := req.ClientArch(); len(a) > 0 {
		arch = a[0]
	}
	switch arch {
	case iana.INTEL_X86PC:
		return "undionly.kpxe", nil
	case iana.EFI_IA32, iana.EFI_BC, iana.EFI_X86_64:
		return "ipxe.efi", nil
	case iana.EFI_ARM64:
		return "ipxe-arm64.efi", nil
	}
	return "", fmt.Errorf("%w: %s", ErrUnknownArch, arch)
}

// Reply builds the response to req, or nil when none is due.
func Reply(req *dhcpv4.DHCPv4, cfg Config, yiaddr net.IP,
	bootfile string) (*dhcpv4.DHCPv4, error) {
	switch req.MessageType() {
	case dhcpv4.MessageTypeDiscover:
		return offer(req, cfg, yiaddr, bootfile,
			dhcpv4.MessageTypeOffer)
	case dhcpv4.MessageTypeRequest:
		want := req.RequestedIPAddress()
		if want == nil || want.IsUnspecified() {
			want = req.ClientIPAddr
		}
		if want.Equal(yiaddr) {
			return offer(req, cfg, yiaddr, bootfile,
				dhcpv4.MessageTypeAck)
		}
		return nak(req, cfg)
	}
	return nil, nil
}

func offer(req *dhcpv4.DHCPv4, cfg Config, yiaddr net.IP,
	bootfile string, mt dhcpv4.MessageType,
) (*dhcpv4.DHCPv4, error) {
	sip := cfg.ServerIP.To4()
	tftp := dhcpv4.OptTFTPServerName(sip.String())
	mods := []dhcpv4.Modifier{
		dhcpv4.WithMessageType(mt),
		dhcpv4.WithYourIP(yiaddr.To4()),
		dhcpv4.WithServerIP(sip),
		dhcpv4.WithOption(dhcpv4.OptServerIdentifier(sip)),
		dhcpv4.WithNetmask(cfg.Mask),
		dhcpv4.WithRouter(sip),
		dhcpv4.WithLeaseTime(cfg.LeaseSecs),
		dhcpv4.WithOption(tftp),
		dhcpv4.WithOption(dhcpv4.OptBootFileName(bootfile)),
	}
	if len(cfg.DNS) > 0 {
		mods = append(mods, dhcpv4.WithDNS(cfg.DNS...))
	}
	rep, err := dhcpv4.NewReplyFromRequest(req, mods...)
	if err != nil {
		return nil, err
	}
	// PXE ROMs read the BOOTP header fields, not only option 67.
	rep.BootFileName = bootfile
	return rep, nil
}

func nak(req *dhcpv4.DHCPv4, cfg Config) (*dhcpv4.DHCPv4, error) {
	sip := cfg.ServerIP.To4()
	return dhcpv4.NewReplyFromRequest(req,
		dhcpv4.WithMessageType(dhcpv4.MessageTypeNak),
		dhcpv4.WithServerIP(sip),
		dhcpv4.WithOption(dhcpv4.OptServerIdentifier(sip)),
	)
}

// Server is the UDP listener.
type Server struct {
	cfg    Config
	lookup Lookup
	logs   chan<- string
}

// New prepares a server; Run starts it.
func New(cfg Config, lookup Lookup, logs chan<- string) *Server {
	return &Server{cfg: cfg, lookup: lookup, logs: logs}
}

func (s *Server) logf(format string, args ...any) {
	s.logs <- fmt.Sprintf(format, args...)
}

// Run serves forever; it returns only on a setup error.
func (s *Server) Run() error {
	ifi, err := net.InterfaceByName(s.cfg.Interface)
	if err != nil {
		return fmt.Errorf("interface %s: %w", s.cfg.Interface, err)
	}
	laddr := &net.UDPAddr{IP: net.IPv4zero, Port: 67}
	conn, err := net.ListenUDP("udp4", laddr)
	if err != nil {
		return fmt.Errorf("listen udp/67: %w", err)
	}
	defer conn.Close()
	pc := ipv4.NewPacketConn(conn)
	// CRITICAL: IP_PKTINFO tells us which interface a request
	// arrived on and lets us reply on that same interface.
	err = pc.SetControlMessage(ipv4.FlagInterface, true)
	if err != nil {
		return fmt.Errorf("control message: %w", err)
	}
	s.logf("Listening for DHCP on port 67 (interface %s)",
		s.cfg.Interface)
	buf := make([]byte, maxMTU)
	for {
		n, cm, _, err := pc.ReadFrom(buf)
		if err != nil {
			s.logf("DHCP read: %v", err)
			continue
		}
		if cm == nil || cm.IfIndex != ifi.Index {
			continue
		}
		s.handle(pc, cm.IfIndex, buf[:n])
	}
}

func (s *Server) handle(pc *ipv4.PacketConn, ifIndex int, b []byte) {
	req, err := dhcpv4.FromBytes(b)
	if err != nil {
		s.logf("Bad DHCP packet: %v", err)
		return
	}
	mac := req.ClientHWAddr
	mt := req.MessageType()
	ip, ok := s.lookup(mac)
	if !ok {
		s.logf("Ignoring %s from unknown MAC %s", mt, mac)
		return
	}
	bootfile, err := Bootfile(req, s.cfg.HTTPBoot)
	if err != nil {
		s.logf("Ignoring %s from %s: %v", mt, mac, err)
		return
	}
	rep, err := Reply(req, s.cfg, ip, bootfile)
	if err != nil {
		s.logf("Building reply for %s: %v", mac, err)
		return
	}
	if rep == nil {
		return
	}
	dst := &net.UDPAddr{IP: net.IPv4bcast, Port: 68}
	wcm := &ipv4.ControlMessage{IfIndex: ifIndex}
	if _, err := pc.WriteTo(rep.ToBytes(), wcm, dst); err != nil {
		s.logf("Sending %s to %s: %v", rep.MessageType(), mac, err)
		return
	}
	s.logf("Sent %s of %s to %s (%s)", rep.MessageType(), ip, mac,
		bootfile)
}
