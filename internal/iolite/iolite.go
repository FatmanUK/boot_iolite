package iolite

import (
	"fmt"
	idhcp "iolite/internal/dhcp"
	itftp "iolite/internal/tftp"
	ihttp "iolite/internal/http"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"net"
)

var db *gorm.DB

const MSG_LISTEN_S = "Listening for DHCP on port 67 (interface %s)..."
const MSG_DB_MIGRATED_OK = "Database migration completed."

const ERR_DB_CONN_V = "Failed to connect to database: %v"
const ERR_DB_MIGR_V = "Failed to migrate database: %v"

const QRY_PRF_MAC = "hardware_address = ?"

const FMT_PROFILE_ENV = `#!/bin/sh
HOSTNAME='%s'
MAC='%s'
IP='%s'
DISTRO='%s'
DISKLAYOUT='%s'
`

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
		p := ProfileFactory(m.Request.ClientHWAddr)
		m.ClientIP = p.IPAddress
		if m.ClientIP == "" {
			continue
		}
		m.SubMask = d.IP.Mask
		m.ServerIP = d.IP.IP.String()
		m.ProcessPackets(d.Interface, d.BootScript, logs)
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
	DbName string
}

func (h HTTPServer) Run(logs chan string) {
	panicIfNotNull(ihttp.Server(h.IP, h.DocRoot, logs))
}

type Profile struct {
	gorm.Model
	HardwareAddress string `gorm:"unique;not null"`
	IPAddress string `gorm:"unique;not null"`
	FQDN string `gorm:"unique;not null"`
	Distro string
	IsBuildEnabled bool
	DiskLayout string
}

func ProfileFactory(m net.HardwareAddr) Profile {
	var p Profile
	macBytes := idhcp.StringFromMACBytes(m)
	db.Where(QRY_PRF_MAC, macBytes).First(&p)
	if p.IsBuildEnabled {
		return p
	}
	return Profile{}
}

func LoadProfiles(b string, d string, logs chan string) error {
	var err error
	gc := gorm.Config{}
	db, err = gorm.Open(sqlite.Open(b), &gc)
	if err != nil {
		return fmt.Errorf(ERR_DB_CONN_V, err)
	}
	err = db.AutoMigrate(&Profile{})
	if err != nil {
		return fmt.Errorf(ERR_DB_MIGR_V, err)
	}
	logs <- MSG_DB_MIGRATED_OK
	// Get all records
	var allProfiles []Profile
	db.Find(&allProfiles)
	for _, p := range allProfiles {
		if p.IsBuildEnabled {
			c := []byte(fmt.Sprintf(
				FMT_PROFILE_ENV,
				p.FQDN,
				p.HardwareAddress,
				p.IPAddress,
				p.Distro,
				p.DiskLayout,
			))
			f := filepath.Join(d, p.HardwareAddress)
			panicIfNotNull(os.WriteFile(f, c, 0644))
		}
	}
	return nil
}
