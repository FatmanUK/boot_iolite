package dhcp

import (
	"fmt"
	"github.com/insomniacslk/dhcp/dhcpv4"
	//"github.com/insomniacslk/dhcp/iana"
	"golang.org/x/net/ipv4"
	"net"
)

const MAXMTU = 1500

const ERR_RESOLVE_SV = "failed to find interface %s: %v"
const ERR_BIND_V = "failed to listen on UDP port 67: %v"
const ERR_PACKET_V = "failed to set control message flag: %v"
const ERR_CONTROL_MESSAGE = "No control message."
const ERR_WRONG_INTERFACE = "Wrong interface."
const ERR_OFFER_NONEXISTENT = "No offer exists. Ignoring."
const ERR_OFFER_ALREADY = "Already offered. Ignoring."
const ERR_DHCPACK_V = "Failed to build DHCP ack: %v"
const ERR_WRITE_UDP_V = "Failed to write UDP response: %v"
const ERR_DHCPOFFER_V = "Failed to build DHCP offer: %v"

const MSG_RECEIVED_SS = "Received %s from MAC: %s"
const MSG_DHCPACK_SENT_SS = "Sent DHCP Ack of %s to MAC %s"
const MSG_DHCPOFFER_SENT_SS = "Sent DHCP Offer of %s to MAC %s"
const MSG_TFTP_REPLY_S = "TFTP reply sent to %s"

// BUG: this system isn't right.
// use gorm for this?
// use a Callwheel to time out the offer.
var offersMade map[string]bool

type DHCP4Message struct {
	Interface        *net.Interface
	PacketConnection *ipv4.PacketConn
	ControlMessage   *ipv4.ControlMessage
	Source           net.Addr
	Request          *dhcpv4.DHCPv4
	MessageType      dhcpv4.MessageType
	ClientIP         string
	ServerIP         string
	SubMask          net.IPMask
}

func (m *DHCP4Message) CheckControlMessage() error {
	if m.ControlMessage == nil {
		return fmt.Errorf(ERR_CONTROL_MESSAGE)
	}
	if m.ControlMessage.IfIndex != m.Interface.Index {
		return fmt.Errorf(ERR_WRONG_INTERFACE)
	}
	return nil
}

// Read payload along with out-of-band control metadata
// Filter traffic to isolate packets targeting our index
// Parse extracted payload array into a DHCPv4 payload
func (m *DHCP4Message) ReadPacket() error {
	var err error
	var n int
	buf := make([]byte, MAXMTU)
	p := m.PacketConnection
	n, m.ControlMessage, m.Source, err = p.ReadFrom(buf)
	if err != nil {
		return err
	}
	err = m.CheckControlMessage()
	if err != nil {
		return err
	}
	m.Request, err = dhcpv4.FromBytes(buf[:n])
	return err
}

func (m *DHCP4Message) ProcessPackets(iface string, bootScript string,
	logs chan string) {
	logs <- fmt.Sprintf(
		MSG_RECEIVED_SS,
		m.Request.MessageType(),
		m.Request.ClientHWAddr,
	)
	switch m.Request.MessageType() {
	case dhcpv4.MessageTypeDiscover:
		{
			m.handleDiscover(logs)
		}
	case dhcpv4.MessageTypeRequest:
		{
			// After grabbing the correct bootloader from
			// tftp, another dhcp request is sent with
			// Option 77 (User Class) set.
			if m.IsIPXE() {
				m.handleIPXE(bootScript, logs)
			} else {
				m.handleRequest(bootScript, logs)
			}
		}
	}
}

func (m *DHCP4Message) SendBytes(dhcpBytes []byte) error {
	dstAddr := &net.UDPAddr{IP: net.IPv4bcast, Port: 68}
	// Re-use and modify the control message to dictate the
	// outbound interface, resolving routing deadlocks.
	wcm := &ipv4.ControlMessage{
		IfIndex: m.ControlMessage.IfIndex,
	}
	_, err := m.PacketConnection.WriteTo(dhcpBytes, wcm, dstAddr)
	return err
}

func (m *DHCP4Message) IsIPXE() bool {
	for _, uc := range m.Request.UserClass() {
		if uc == "iPXE" {
			return true
		}
	}
	return false
}

/*
const ERR_ARCH_UNK_V = "Unknown Option 93 Architecture: %v, defaulting to BIOS"

func (m *DHCP4Message) parseOption93BootFile() string {
	var bootFile string = "undionly.kpxe"
	o93Type := dhcpv4.OptionClientSystemArchitectureType
	o93 := m.Request.Options.Get(o93Type)
	if len(o93) >= 2 {
		// Option 93 contains uint16 architecture profiles
		arch := iana.Arch(uint16(o93[0])<<8 | uint16(o93[1]))
		switch arch {
			case 0: {
				bootFile = "undionly.kpxe"
			}
			case 6, 7, 9: {
				bootFile = "ipxe.efi"
			}
			case 11: {
				bootFile = "ipxe-arm64.efi"
			}
			//default: {
			//	log.Printf(ERR_ARCH_UNK_V, arch)
			//}
		}
	}
	return bootFile
}
*/

func (m *DHCP4Message) handleDiscover(logs chan string) {
	var err error
	macStr := StringFromMACBytes(m.Request.ClientHWAddr)
	if IsOffered(macStr) {
		logs <- ERR_OFFER_ALREADY
		return
	}
	RecordOffer(macStr)
	sIP := net.ParseIP(m.ServerIP).To4()
	dhcpOffer, err := dhcpv4.NewReplyFromRequest(m.Request,
		dhcpv4.WithMessageType(dhcpv4.MessageTypeOffer),
		dhcpv4.WithYourIP(net.ParseIP(m.ClientIP).To4()),
		dhcpv4.WithServerIP(sIP),
		dhcpv4.WithNetmask(m.SubMask),
		dhcpv4.WithRouter(sIP),
		dhcpv4.WithLeaseTime(3600),
	)
	if err != nil {
		logs <- fmt.Sprintf(ERR_DHCPOFFER_V, err)
		return
	}
	err = m.SendBytes(dhcpOffer.ToBytes())
	if err != nil {
		logs <- fmt.Sprintf(ERR_WRITE_UDP_V, err)
		return
	}
	logs <- fmt.Sprintf(MSG_DHCPOFFER_SENT_SS, m.ClientIP, macStr)
}

// apparently we're not using this --- I'm a little confused due to
// taking protocol advice from Gemini, but this seems to work
func (m *DHCP4Message) handleRequest(b string, logs chan string) {
	var err error
	macStr := StringFromMACBytes(m.Request.ClientHWAddr)
	if !IsOffered(macStr) {
		logs <- ERR_OFFER_NONEXISTENT
		return
	}
	RescindOffer(macStr)
	sIP := net.ParseIP(m.ServerIP).To4()
	tftpNameOpt := dhcpv4.OptTFTPServerName(sIP.String())
	dhcpAck, err := dhcpv4.NewReplyFromRequest(m.Request,
		dhcpv4.WithMessageType(dhcpv4.MessageTypeAck),
		dhcpv4.WithYourIP(net.ParseIP(m.ClientIP).To4()),
		dhcpv4.WithServerIP(sIP),
		dhcpv4.WithNetmask(m.SubMask),
		dhcpv4.WithRouter(sIP),
		dhcpv4.WithLeaseTime(3600),
		dhcpv4.WithOption(tftpNameOpt),
		dhcpv4.WithOption(dhcpv4.OptBootFileName(b)),
	)
	if err != nil {
		logs <- fmt.Sprintf(ERR_DHCPACK_V, err)
		return
	}
	err = m.SendBytes(dhcpAck.ToBytes())
	if err != nil {
		logs <- fmt.Sprintf(ERR_WRITE_UDP_V, err)
		return
	}
	logs <- fmt.Sprintf(MSG_DHCPACK_SENT_SS, m.ClientIP, macStr)
}

func (m *DHCP4Message) handleIPXE(b string, logs chan string) {
	var err error
	macStr := StringFromMACBytes(m.Request.ClientHWAddr)
	if !IsOffered(macStr) {
		logs <- ERR_OFFER_NONEXISTENT
		return
	}
	RescindOffer(macStr)
	b = fmt.Sprintf("http://%s%s", m.ServerIP, b)
	sIP := net.ParseIP(m.ServerIP).To4()
	tftpNameOpt := dhcpv4.OptTFTPServerName(sIP.String())
	dhcpAck, err := dhcpv4.NewReplyFromRequest(m.Request,
		dhcpv4.WithMessageType(dhcpv4.MessageTypeAck),
		dhcpv4.WithYourIP(net.ParseIP(m.ClientIP).To4()),
		dhcpv4.WithServerIP(sIP),
		dhcpv4.WithNetmask(m.SubMask),
		dhcpv4.WithRouter(sIP),
		dhcpv4.WithLeaseTime(3600),
		dhcpv4.WithOption(tftpNameOpt),
		dhcpv4.WithOption(dhcpv4.OptBootFileName(b)),
	)
	if err != nil {
		logs <- fmt.Sprintf(ERR_DHCPACK_V, err)
		return
	}
	err = m.SendBytes(dhcpAck.ToBytes())
	if err != nil {
		logs <- fmt.Sprintf(ERR_WRITE_UDP_V, err)
		return
	}
	logs <- fmt.Sprintf(MSG_DHCPACK_SENT_SS, m.ClientIP, macStr)
}

// Resolve target interface to obtain its system index
func ResolveInterface(n string) (*net.Interface, error) {
	var err error
	ni, err := net.InterfaceByName(n)
	if err != nil {
		err = fmt.Errorf(ERR_RESOLVE_SV, n, err)
	}
	return ni, err
}

// TODO: consider adding ipv6?
// Bind UDP listener to port 67 across all local addresses (0.0.0.0)
func BindAll4() (*net.UDPConn, error) {
	var err error
	laddr := &net.UDPAddr{IP: net.IPv4zero, Port: 67}
	nu, err := net.ListenUDP("udp4", laddr)
	if err != nil {
		err = fmt.Errorf(ERR_BIND_V, err)
	}
	return nu, err
}

// Wrap connection to enable out-of-band message handling
// CRITICAL: Pass IP_PKTINFO control data with packets
func PacketConnectionFactory(conn *net.UDPConn) (*ipv4.PacketConn,
	error) {
	p := ipv4.NewPacketConn(conn)
	err := p.SetControlMessage(ipv4.FlagInterface, true)
	if err != nil {
		err = fmt.Errorf(ERR_PACKET_V, err)
	}
	return p, err
}

func RecordOffer(s string) {
	if offersMade == nil {
		offersMade = make(map[string]bool)
	}
	offersMade[s] = true
}

func RescindOffer(s string) {
	if offersMade == nil {
		offersMade = make(map[string]bool)
	}
	offersMade[s] = false
}

func IsOffered(s string) bool {
	rv, ok := offersMade[s]
	if !ok {
		return false
	}
	return rv
}

func StringFromMACBytes(mac []byte) string {
	return net.HardwareAddr(mac).String()
}
