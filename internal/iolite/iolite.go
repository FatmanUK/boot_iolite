package iolite

import (
	cw "github.com/FatmanUK/fatgo/callwheel"
	"fmt"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	idhcp "iolite/internal/dhcp"
	ihttp "iolite/internal/http"
	itftp "iolite/internal/tftp"
	"net"
	"os"
	"path/filepath"
	//"strings"
	"time"
)

var db *gorm.DB

var ipxeMessage = "Bootstrapping iolite..."
var ipxeKernelOptions = "console=tty0 earlyprintk=tty0 tsc=reliable"

const MSG_LISTEN_S = "Listening for DHCP on port 67 (interface %s)..."
const MSG_IPXE_CREATED_S = "Created %s."
const MSG_DB_MIGRATED_OK = "Database migration completed."

const ERR_DB_CONN_V = "Failed to connect to database: %v"
const ERR_DB_MIGR_V = "Failed to migrate database: %v"

const QRY_PRF_MAC = "hardware_address = ?"

const FMT_IPXE_BANG_PATH = `#!ipxe
`
const FMT_IPXE_MESSAGE = `echo %s
`
const FMT_KERNEL_LINE = `kernel http://%s/boot/vmlinuz %s
`
const FMT_INITRD_LINE = `initrd http://%s/boot/initrd.img
`
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
	IP         net.IPNet
	Interface  string
	BootScript string
}

func (d DHCPServer) Run(logs chan string) {
	var err error
	var m idhcp.DHCP4Message
	var c *net.UDPConn
	epoch := time.Now()
	idhcp.DhcpOfferTimeouts = cw.CallWheelFactory(10)
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
		thisEpoch := time.Now()
		ticks := thisEpoch.Sub(epoch).Milliseconds() / 1000
		epoch = thisEpoch
		for ticks > 0 {
			idhcp.DhcpOfferTimeouts.Tick()
			ticks--
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
	IP      net.IPNet
	DocRoot string
}

func (t TFTPServer) Run(logs chan string) {
	panicIfNotNull(itftp.Server(t.IP, t.DocRoot, logs))
}

type HTTPServer struct {
	IP         net.IPNet
	DocRoot    string
	BootScript string
	DbName     string
}

func (h HTTPServer) createBootScript() string {
	f := filepath.Join(h.DocRoot, h.BootScript)
	c := FMT_IPXE_BANG_PATH
	c += fmt.Sprintf(FMT_IPXE_MESSAGE, ipxeMessage)
	c += fmt.Sprintf(FMT_KERNEL_LINE, h.IP.IP, ipxeKernelOptions)
	c += fmt.Sprintf(FMT_INITRD_LINE, h.IP.IP)
	c += `boot`
	panicIfNotNull(os.WriteFile(f, []byte(c), 0644))
	return f
}

func (h HTTPServer) Run(logs chan string) {
	panicIfNotNull(LoadProfiles(h.DbName, h.DocRoot, logs))
	//createLayouts(logs)
	//createProfiles(logs)
	logs <- fmt.Sprintf(MSG_IPXE_CREATED_S, h.createBootScript())
	panicIfNotNull(ihttp.Server(h.IP, h.DocRoot, logs))
}

func MakeTestData(logs chan string) error {
	var err error
	bootTemplate := Template{
		Name:       "Boot Template",
		SectorSize: 512,
		Partitions: []MBRPartition{
			{Start: 2048, Size: 1048576, Type: "0x0C", IsBootable: true},
			{Start: 1050624, Size: 20971520, Type: "0x83", IsBootable: false},
		},
	}

	layout1 := Layout{
		Name: "standard-dos-layout",
		Templates: map[string]Template{
			"boot": bootTemplate,
			"data": {
				Name:       "Data Template",
				SectorSize: 4096,
				Partitions: []MBRPartition{
					{Start: 22020096, Size: 104857600, Type: "0x07", IsBootable: false},
				},
			},
		},
	}

	layout2 := Layout{
		Name: "minimal-layout",
		Templates: map[string]Template{
			"primary-boot": bootTemplate, // same template, reused under a different key
		},
	}

/*
	fiveLvm := DiskTemplate{
		Name: "MBR_5_LVM",
		Type: "mbr",
		SectorSize: 512,
		Partitions: []MBRPartitionTemplate{
			MBRPartitionTemplate{
				Start: 63,
				Size: 10485697,
				Type: "8e",
				IsBootable: false,
			},
		},
	}
*/
	err = db.Create(&layout1).Error
	if err != nil {
		return fmt.Errorf("create layout1: %v", err)
	}
	fmt.Printf("stored layout1 id=%d\n", layout1.ID)
	// layout1.Templates["boot"].ID is now set (from BeforeSave) — reuse it
	// explicitly so layout2 points at the same Template row instead of
	// creating a duplicate.
	layout2.Templates["primary-boot"] = layout1.Templates["boot"]

	err = db.Create(&layout2).Error
	if err != nil {
		return fmt.Errorf("create layout2: %v", err)
	}
	fmt.Printf("stored layout2 id=%d\n", layout2.ID)
	return nil
}

func WriteProfileEnvFiles(logs chan string) {
/*
	// Get all records
	var allProfiles []Profile
	db.Find(&allProfiles)
	for _, p := range allProfiles {
		if p.IsBuildEnabled {
			l := DiskLayout{}.String(p.Layout)
			c := []byte(fmt.Sprintf(
				FMT_PROFILE_ENV,
				p.FQDN,
				p.HardwareAddress,
				p.IPAddress,
				p.Distro,
				l,
			))
			f := filepath.Join(d, p.HardwareAddress)
			panicIfNotNull(os.WriteFile(f, c, 0644))
		}
	}
*/
}

func LoadProfiles(b string, d string, logs chan string) error {
	var err error
	gc := gorm.Config{}
	// TODO: optionally replace with postgresql
	// Then have repeated tries to connect instead of error
	db, err = gorm.Open(sqlite.Open(b), &gc)
	if err != nil {
		return fmt.Errorf(ERR_DB_CONN_V, err)
	}
	err = db.AutoMigrate(
		&Profile{},
		&Layout{},
		&LayoutTemplate{},
		&Template{},
		&MBRPartition{},
	)
	if err != nil {
		return fmt.Errorf(ERR_DB_MIGR_V, err)
	}
	logs <- MSG_DB_MIGRATED_OK
	MakeTestData(logs)
	WriteProfileEnvFiles(logs)
	return nil
}
