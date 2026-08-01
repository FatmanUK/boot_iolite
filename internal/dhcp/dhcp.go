package dhcp

import (
	"log"
	"fmt"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"golang.org/x/net/ipv4"
	"net"
	"github.com/insomniacslk/dhcp/iana"
)

const MAXMTU = 1500

const ERR_ARCH_UNK_V = "Unknown Option 93 Architecture: %v, defaulting to BIOS"
const ERR_CONTROL_MESSAGE = "No control message."
const ERR_WRONG_INTERFACE = "Wrong interface."

// BUG: this system isn't right.
// use gorm for this?
var offersMade map[string]bool

func IsOfferMade(s string) bool {
	rv, ok := offersMade[s]
	if !ok {
		return false
	}
	return rv
}

func MakeOffer(s string) {
	offersMade[s] = true
}

func RescindOffer(s string) {
	offersMade[s] = false
}

type DHCP4Message struct {
	Interface *net.Interface
	PacketConnection *ipv4.PacketConn
	ControlMessage *ipv4.ControlMessage
	Source net.Addr
	Request *dhcpv4.DHCPv4
	//
	MessageType dhcpv4.MessageType
	ClientIP string
	ServerIP string
	SubMask net.IPMask
}

func (cd *DHCP4Message) CheckControlMessage() error {
	if cd.ControlMessage == nil {
		return fmt.Errorf(ERR_CONTROL_MESSAGE)
	}
	if cd.ControlMessage.IfIndex != cd.Interface.Index {
		return fmt.Errorf(ERR_WRONG_INTERFACE)
	}
	return nil
}

// Read payload along with out-of-band control metadata
// Filter traffic to isolate packets targeting our index
// Parse extracted payload array into a DHCPv4 payload
func (cd *DHCP4Message) ReadPacket() error {
	var err error
	var n int
	buf := make([]byte, MAXMTU)
	p := cd.PacketConnection
	n, cd.ControlMessage, cd.Source, err = p.ReadFrom(buf)
	if err != nil {
		return err
	}
	err = cd.CheckControlMessage()
	if err != nil {
		return err
	}
	cd.Request, err = dhcpv4.FromBytes(buf[:n])
	return err
}

func (cd *DHCP4Message) BuildMessage(lease uint32) (*dhcpv4.DHCPv4,
		error) {
	// Define static configuration allocation profile parameters
	cIP := net.ParseIP(cd.ClientIP).To4()
	sIP := net.ParseIP(cd.ServerIP).To4()
	var bootFile string = "undionly.kpxe"
	o93Type := dhcpv4.OptionClientSystemArchitectureType
	o93 := cd.Request.Options.Get(o93Type)
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
			default: {
				log.Printf(ERR_ARCH_UNK_V, arch)
			}
		}
	}
	return dhcpv4.NewReplyFromRequest(cd.Request,
		dhcpv4.WithMessageType(cd.MessageType),
		dhcpv4.WithYourIP(cIP),
		dhcpv4.WithServerIP(sIP),
		dhcpv4.WithNetmask(cd.SubMask),
		dhcpv4.WithRouter(sIP),
		dhcpv4.WithLeaseTime(lease),
		dhcpv4.WithOption(dhcpv4.OptTFTPServerName(sIP.String())),
		dhcpv4.WithOption(dhcpv4.OptBootFileName(bootFile)),
	)
}

func (cd *DHCP4Message) IsIPXE() bool {
	ub := cd.Request.UserClass()
	for _, uc := range ub {
		if uc == "iPXE" {
			return true
		}
	}
	return false
}

func (cd *DHCP4Message) SendBytes(dhcpBytes []byte) error {
	dstAddr := &net.UDPAddr{IP: net.IPv4bcast, Port: 68}
	// Re-use and modify the control message to dictate the
	// outbound pipeline. This forces the Linux kernel to send the
	// broadcast frame through the same interface index the
	// request arrived on, resolving routing deadlocks.
	wcm := &ipv4.ControlMessage{
		IfIndex: cd.ControlMessage.IfIndex,
	}
	_, err := cd.PacketConnection.WriteTo(dhcpBytes, wcm, dstAddr)
	return err
}

// Resolve target interface to obtain its system index
func ResolveInterface(n string) (*net.Interface, error) {
	return net.InterfaceByName(n)
}

// TODO: consider adding ipv6?
// Bind UDP listener to port 67 across all local addresses (0.0.0.0)
func BindAll4() (*net.UDPConn, error) {
	offersMade = make(map[string]bool)
	laddr := &net.UDPAddr{IP: net.IPv4zero, Port: 67}
	return net.ListenUDP("udp4", laddr)
}

// Wrap connection to enable out-of-band message handling
// CRITICAL: Pass IP_PKTINFO control data with packets
func PacketConnectionFactory(conn *net.UDPConn) (*ipv4.PacketConn,
		error) {
	p := ipv4.NewPacketConn(conn)
	err := p.SetControlMessage(ipv4.FlagInterface, true)
	return p, err
}

func StringFromMACBytes(mac []byte) string {
	return net.HardwareAddr(mac).String()
}
