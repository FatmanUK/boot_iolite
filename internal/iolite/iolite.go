package iolite

import (
	idhcp "iolite/internal/dhcp"
	"net"
)

func GetIPFromProfile(cd idhcp.DHCP4Message) string {
	var ip string
	macBytes := net.HardwareAddr(cd.Request.ClientHWAddr).String()
	// TODO: Search profiles --- is build enabled on profile?
	if idhcp.IsOfferMade(macBytes) {
		idhcp.RescindOffer(macBytes)
	}
	switch macBytes {
		case "52:54:00:76:de:e5": {
			idhcp.MakeOffer(macBytes)
			ip = "172.168.16.32"
		}
	}
	return ip
}
