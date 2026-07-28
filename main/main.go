package main

import (
	"fmt"
	"log"
	idhcp "iolite/internal/dhcp"
	"iolite/internal/iolite"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"net"
)

const ERR_RESOLVE_SV = "failed to find interface %s: %v"
const ERR_BIND_V = "failed to listen on UDP port 67: %v"
const ERR_PACKET_V = "failed to set control message flag: %v"
const ERR_OFFER_NONEXISTENT = "No offer exists. Ignoring."
const ERR_OFFER_ALREADY = "Already offered. Ignoring."
const ERR_DHCPACK_V = "Failed to build DHCP ack: %v"
const ERR_WRITE_UDP_V = "Failed to write UDP response: %v"
const ERR_DHCPOFFER_V = "Failed to build DHCP offer: %v"

const MSG_DHCPACK_SENT_SS = "Sent DHCP Ack of %s to MAC %s"
const MSG_DHCPOFFER_SENT_SS = "Sent DHCP Offer of %s to MAC %s"
const MSG_LISTEN_S = "Listening on port 67 (filtering for %s)..."
const MSG_RECEIVED_SSS = "[%s] Received %s from MAC: %s"

var iface = "virbr0"
var srvIP = "172.168.16.1"

func main() {
	go dhcpServer()
	for { }
}

func dhcpServer() {
	var err error
	var pm idhcp.DHCP4Message
	pm.Interface, err = idhcp.ResolveInterface(iface)
	if err != nil {
		log.Fatalf(ERR_RESOLVE_SV, iface, err)
	}
	conn, err := idhcp.BindAll4()
	if err != nil {
		log.Fatalf(ERR_BIND_V, err)
	}
	defer conn.Close()
	pm.PacketConnection, err = idhcp.PacketConnectionFactory(conn)
	if err != nil {
		log.Fatalf(ERR_PACKET_V, err)
	}
	fmt.Println(fmt.Sprintf(MSG_LISTEN_S, iface))
	for {
		processInboundPackets(pm)
	}
}

func processInboundPackets(pwm idhcp.DHCP4Message) {
	if pwm.ReadPacket() != nil {
		return
	}
	log.Println(fmt.Sprintf(MSG_RECEIVED_SSS,
		iface,
		pwm.Request.MessageType(),
		pwm.Request.ClientHWAddr,
	))
	switch pwm.Request.MessageType() {
		case dhcpv4.MessageTypeDiscover: {
			handleDiscover(pwm)
		}
		case dhcpv4.MessageTypeRequest: {
			handleRequest(pwm)
		}
	}
}

func getSubMask() net.IPMask {
	return net.CIDRMask(16, 32)
}

// Process the request if it is a Request message
func handleRequest(cd idhcp.DHCP4Message) {
	macStr := idhcp.StringFromMACBytes(cd.Request.ClientHWAddr)
	if !idhcp.IsOfferMade(macStr) {
		log.Printf(ERR_OFFER_NONEXISTENT)
		return
	}
	cd.MessageType = dhcpv4.MessageTypeAck
	cd.ClientIP = iolite.GetIPFromProfile(cd)
	cd.ServerIP = srvIP
	cd.SubMask = getSubMask()
	dhcpAck, err := cd.BuildMessage(3600)
	if err != nil {
		log.Printf(ERR_DHCPACK_V, err)
		return
	}
	err = cd.SendBytes(dhcpAck.ToBytes())
	if err != nil {
		log.Printf(ERR_WRITE_UDP_V, err)
		return
	}
	log.Printf(MSG_DHCPACK_SENT_SS, cd.ClientIP, macStr)
	idhcp.RescindOffer(macStr)
}

// Process the request if it is a Discover message
func handleDiscover(cd idhcp.DHCP4Message) {
	macStr := idhcp.StringFromMACBytes(cd.Request.ClientHWAddr)
	if idhcp.IsOfferMade(macStr) {
		log.Printf(ERR_OFFER_ALREADY)
		return
	}
	cd.MessageType = dhcpv4.MessageTypeOffer
	cd.ClientIP = iolite.GetIPFromProfile(cd)
	cd.ServerIP = srvIP
	cd.SubMask = getSubMask()
	dhcpOffer, err := cd.BuildMessage(3600)
	if err != nil {
		log.Printf(ERR_DHCPOFFER_V, err)
		return
	}
	err = cd.SendBytes(dhcpOffer.ToBytes())
	if err != nil {
		log.Printf(ERR_WRITE_UDP_V, err)
		return
	}
	log.Printf(MSG_DHCPOFFER_SENT_SS, cd.ClientIP, macStr)
}
