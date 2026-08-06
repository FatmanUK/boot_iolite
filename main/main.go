package main

import (
	"iolite/internal/iolite"
	"log"
	"net"
)

func main() {
	var srvIP = net.IPNet{
		IP:   net.ParseIP("172.168.16.1"),
		Mask: net.CIDRMask(16, 32),
	}
	var bootScript = "/boot/boot.ipxe"
	d := iolite.DHCPServer{
		IP:         srvIP,
		Interface:  "virbr0",
		BootScript: bootScript,
	}
	t := iolite.TFTPServer{
		IP:      srvIP,
		DocRoot: "./tftpboot",
	}
	h := iolite.HTTPServer{
		IP:         srvIP,
		DocRoot:    "./tinypxe/output",
		BootScript: bootScript,
		DbName:     "profiles.db",
	}
	logs := make(chan string, 1)
	go d.Run(logs)
	go t.Run(logs)
	go h.Run(logs)
	for msg := range logs {
		log.Println(msg)
	}
}
