package main

import (
	"fmt"
	"log"
	idhcp "iolite/internal/dhcp"
	"github.com/insomniacslk/dhcp/dhcpv4"
)

const ERR_RESOLVE_SV = "failed to find interface %s: %v"
const ERR_BIND_V = "failed to listen on UDP port 67: %v"
const ERR_PACKET_V = "failed to set control message flag: %v"

const MSG_LISTEN_S = "DHCP server listening on port 67 (Filtering for %s)..."
const MSG_RECEIVED_SSS = "[%s] Received %s from MAC: %s"

func main() {
	iface := "virbr0"
	ifi, err := idhcp.ResolveInterface(iface)
	if err != nil {
		log.Fatalf(ERR_RESOLVE_SV, iface, err)
	}
	conn, err := idhcp.BindAll4()
	if err != nil {
		log.Fatalf(ERR_BIND_V, err)
	}
	defer conn.Close()
	p, err := idhcp.PacketFactory(conn)
	if err != nil {
		log.Fatalf(ERR_PACKET_V, err)
	}
	fmt.Println(fmt.Sprintf(MSG_LISTEN_S, iface))
	for {
		req, pm, skip := idhcp.ReadPacket(ifi.Index, p)
		if skip {
			continue
		}
		log.Printf(MSG_RECEIVED_SSS, iface, req.MessageType(),
			req.ClientHWAddr)
		if req.MessageType() == dhcpv4.MessageTypeDiscover {
			//handleDiscover(p, src, cm, req)
			fmt.Println("yay")
		}
	}
}

// 8. Process the request if it is a Discover message
/*
func handleDiscover(p *ipv4.PacketConn, srcAddr net.Addr, cm *ipv4.ControlMessage, req *dhcpv4.DHCPv4) {
	// Define your static configuration allocation profile parameters
	offeredIP := net.ParseIP("192.168.122.50").To4()
	serverIP := net.ParseIP("192.168.122.1").To4()
	subnetMask := net.CIDRMask(24, 32)

	// Assemble the DHCP Offer configuration reply structure
	dhcpOffer, err := dhcpv4.NewReplyFromRequest(req,
		dhcpv4.WithMessageType(dhcpv4.MessageTypeOffer),
		dhcpv4.WithYourIP(offeredIP),
		dhcpv4.WithServerIP(serverIP),
		dhcpv4.WithNetmask(subnetMask),
		dhcpv4.WithRouter(serverIP),
		dhcpv4.WithDNS(net.ParseIP("8.8.8.8")),
		dhcpv4.WithLeaseTime(3600),
	)
	if err != nil {
		log.Printf("Failed to build DHCP offer: %v", err)
		return
	}

	// Convert the completed DHCP layout to a standard byte sequence
	dhcpBytes := dhcpOffer.ToBytes()

	// 9. Route the response using a broadcast target address (255.255.255.255)
	// target port 68 (DHCP Client port)
	dstAddr := &net.UDPAddr{IP: net.IPv4bcast, Port: 68}

	// 10. Re-use and modify the control message to dictate the outbound pipeline.
	// This forces the Linux kernel to send the broadcast frame through the same 
	// interface index the request arrived on, resolving routing deadlocks.
	wcm := &ipv4.ControlMessage{
		IfIndex: cm.IfIndex,
	}

	// Write the raw UDP data payload safely back onto the wire
	_, err = p.WriteTo(dhcpBytes, wcm, dstAddr)
	if err != nil {
		log.Printf("Failed to write UDP response: %v", err)
		return
	}
	log.Printf("Sent DHCP Offer containing IP %s to MAC %s", offeredIP, req.ClientHWAddr)
}
*/
