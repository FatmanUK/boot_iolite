package main

import (
	"net"
	"log"
	"iolite/internal/iolite"
)

func main() {
	var srvIP = net.IPNet{
		IP: net.ParseIP("172.168.16.1"),
		Mask: net.CIDRMask(16, 32),
	}
	d := iolite.DHCPServer{
		IP: srvIP,
		Interface: "virbr0",
		BootScript: "/boot/boot.ipxe",
	}
	t := iolite.TFTPServer{
		IP: srvIP,
		DocRoot: "./tftpboot",
	}
	h := iolite.HTTPServer{
		IP: srvIP,
		DocRoot: "./tinypxe/output",
		DbName: "profiles.db",
	}
	logs := make(chan string, 1)
	go d.Run(logs)
	go t.Run(logs)
	go h.Run(logs)
	for msg := range logs {
		log.Println(msg)
	}
}
