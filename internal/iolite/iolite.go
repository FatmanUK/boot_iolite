package iolite

import (
	"fmt"
	idhcp "iolite/internal/dhcp"
	itftp "iolite/internal/tftp"
	ihttp "iolite/internal/http"
	"net"
)

const MSG_LISTEN_S = "Listening for DHCP on port 67 (interface %s)..."

func panicIfNotNull(err error) {
	if err == nil {
		return
	}
	panic(err)
}

type DHCPServer struct {
	IP net.IPNet
	Interface string
	BootScript string
}

func (d DHCPServer) Run(logs chan string) {
	var err error
	var m idhcp.DHCP4Message
	var c *net.UDPConn
	m.Interface, err = idhcp.ResolveInterface(d.Interface)
	panicIfNotNull(err)
	c, err = idhcp.BindAll4()
	panicIfNotNull(err)
	defer c.Close()
	m.PacketConnection, err = idhcp.PacketConnectionFactory(c)
	panicIfNotNull(err)
	logs <- fmt.Sprintf(MSG_LISTEN_S, d.Interface)
	for {
		if m.ReadPacket() != nil {
			continue
		}
		m.ClientIP = GetIPFromProfile(m.Request.ClientHWAddr)
		if m.ClientIP == "" {
			continue
		}
		m.SubMask = d.IP.Mask
		m.ServerIP = d.IP.IP.String()
		m.ProcessPackets(d.Interface,
				d.BootScript, logs)
	}
}

type TFTPServer struct {
	IP net.IPNet
	DocRoot string
}

func (t TFTPServer) Run(logs chan string) {
	panicIfNotNull(itftp.Server(t.IP, t.DocRoot, logs))
}

type HTTPServer struct {
	IP net.IPNet
	DocRoot string
}

func (h HTTPServer) Run(logs chan string) {
	panicIfNotNull(ihttp.Server(h.IP, h.DocRoot, logs))
}

func GetIPFromProfile(m net.HardwareAddr) string {
	var ip string
	macBytes := idhcp.StringFromMACBytes(m)
	// TODO: Search profiles --- is build enabled on profile?
	// debug
	switch macBytes {
		case "52:54:00:76:de:e5": {
			ip = "172.168.16.32"
		}
		default: {
			return ""
		}
	}
	// debug
	return ip
}
