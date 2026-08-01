package main

import (
	"fmt"
	"log"
	"net"
	"iolite/internal/iolite"
	idhcp "iolite/internal/dhcp"
	itftp "iolite/internal/tftp"
	ihttp "iolite/internal/http"
	"github.com/insomniacslk/dhcp/dhcpv4"
)

const ERR_RESOLVE_SV = "failed to find interface %s: %v"
const ERR_BIND_V = "failed to listen on UDP port 67: %v"
const ERR_PACKET_V = "failed to set control message flag: %v"
const ERR_OFFER_NONEXISTENT = "No offer exists. Ignoring."
const ERR_OFFER_ALREADY = "Already offered. Ignoring."
const ERR_DHCPACK_V = "Failed to build DHCP ack: %v"
const ERR_TFTPREPLY_V = "Failed to build TFTP reply: %v"
const ERR_WRITE_UDP_V = "Failed to write UDP response: %v"
const ERR_DHCPOFFER_V = "Failed to build DHCP offer: %v"

const MSG_LISTEN_S = "Listening on port 67 (filtering for %s)..."
const MSG_RECEIVED_SSS = "[%s] Received %s from MAC: %s"
const MSG_DHCPACK_SENT_SS = "Sent DHCP Ack of %s to MAC %s"
const MSG_DHCPOFFER_SENT_SS = "Sent DHCP Offer of %s to MAC %s"
const MSG_TFTP_REPLY_S = "TFTP reply sent to %s"

var iface = "virbr0"
var srvIP = "172.168.16.1"
var httpDocRoot = "./tinypxe/output"
var tftpDocRoot = "./tftpboot"
var bootScript = "/boot/boot.ipxe"

func getSubMask() net.IPMask {
	return net.CIDRMask(16, 32)  // TODO: understand this better
}

func main() {
	go HTTPServer()
	go TFTPServer()
	go DHCPServer()
	for { }
}

func HTTPServer() {
	ihttp.Server(srvIP, httpDocRoot)
}

func TFTPServer() {
	itftp.Server(tftpDocRoot)
}

func DHCPServer() {
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
			// After grabbing the correct bootloader from
			// tftp, another dhcp request is sent with
			// Option 77 (User Class) set.
			if pwm.IsIPXE() {
				handleIPXERequest(pwm)
			} else {
				handleRequest(pwm)
			}
		}
	}
}

// TODO rethink BuildMessage function
func handleIPXERequest(cd idhcp.DHCP4Message) {
	macStr := idhcp.StringFromMACBytes(cd.Request.ClientHWAddr)
	cd.MessageType = dhcpv4.MessageTypeAck
	cd.ClientIP = iolite.GetIPFromProfile(cd)
	cd.ServerIP = srvIP
	cd.SubMask = getSubMask()
	tftpReply, err := cd.BuildMessage(3600)

	m := fmt.Sprintf("http://%s%s", srvIP, bootScript)
	tftpReply.Options.Update(dhcpv4.OptBootFileName(m))

	if err != nil {
		log.Printf(ERR_TFTPREPLY_V, err)
		return
	}
	err = cd.SendBytes(tftpReply.ToBytes())
	if err != nil {
		log.Printf(ERR_WRITE_UDP_V, err)
		return
	}
	log.Printf(MSG_TFTP_REPLY_S, cd.ClientIP)
	idhcp.RescindOffer(macStr)
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
