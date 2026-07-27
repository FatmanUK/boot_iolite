package dhcp

import (
	"net"
	"golang.org/x/net/ipv4"
	"github.com/insomniacslk/dhcp/dhcpv4"
)

// Resolve target interface to obtain its system index
func ResolveInterface(n string) (*net.Interface, error) {
	return net.InterfaceByName(n)
}

// Bind UDP listener to port 67 across all local addresses (0.0.0.0)
func BindAll4() (*net.UDPConn, error) {
	laddr := &net.UDPAddr{IP: net.IPv4zero, Port: 67}
	return net.ListenUDP("udp4", laddr)
}

// Wrap connection to enable out-of-band message handling
// CRITICAL: Pass IP_PKTINFO control data with packets
func PacketFactory(conn *net.UDPConn) (*ipv4.PacketConn, error) {
	p := ipv4.NewPacketConn(conn)
	err := p.SetControlMessage(ipv4.FlagInterface, true)
	return p, err
}

type PacketMetadata struct {
	srcAddr net.Addr
	cm *ipv4.ControlMessage
}

func ReadPacket(ifi int, p *ipv4.PacketConn) (*dhcpv4.DHCPv4, PacketMetadata, bool) {
	buf := make([]byte, 1500) // 1500 = maxMTU?
	// Read payload along with out-of-band control metadata
	var err error
	var pm PacketMetadata
	var n int
	n, pm.cm, pm.srcAddr, err = p.ReadFrom(buf)
	if err != nil {
		//log.Printf("error reading message: %v", err)
		return nil, pm, true
	}

	// Filter traffic to isolate packets targeting our index
	if pm.cm == nil || pm.cm.IfIndex != ifi {
		return nil, pm, true
	}

	// Parse extracted payload array into a DHCPv4 payload
	req, err := dhcpv4.FromBytes(buf[:n])
	if err != nil {
		return nil, pm, true
	}
	return req, pm, false
}
