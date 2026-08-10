package iolite

import (
	"fmt"
	idhcp "iolite/internal/dhcp"
	"net"
	"os"
	"path/filepath"
)

const FMT_PROFILE_ENV = `#!/bin/sh
HOSTNAME='%s'
MAC='%s'
IP='%s'
DISTRO='%s'
DISKLAYOUT='%s'
`

/*
Note from Claude:
If you want to attach a Profile to an existing Layout without
re-saving/duplicating it, set LayoutID directly instead of the nested
struct, and skip the association save:

profile := Profile{
      LayoutID:        existingLayout.ID,
      HardwareAddress: "...",
      // ...
  }
  db.Omit("Layout").Create(&profile) // don't touch the Layout row
*/

type Profile struct {
	ID              uint    `gorm:"primaryKey"`
	LayoutID        uint    `gorm:"not null;index"`
	Layout          *Layout `gorm:"foreignKey:LayoutID"`
	HardwareAddress string  `gorm:"unique;not null"`
	IPAddress       string  `gorm:"unique;not null"`
	FQDN            string  `gorm:"unique;not null"`
	Distro          string
	IsBuildEnabled  bool
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

func (p Profile) Write(docRoot string, logs chan string) {
	logs <- p.Layout.String()
	conts := []byte(fmt.Sprintf(
		FMT_PROFILE_ENV,
		p.FQDN,
		p.HardwareAddress,
		p.IPAddress,
		p.Distro,
		p.Layout.String(),
	))
	fileName := filepath.Join(docRoot, p.HardwareAddress)
	panicIfNotNull(os.WriteFile(fileName, conts, 0644))
}

/*
func UnzipMap[T comparable, U any](m map[T]U) ([]T, []U) {
	keys := []T{}
	values := []U{}
	for k, v := range m {
		keys = append(keys, k)
		values = append(values, v)
	}
	return keys, values
}

func updateProfile(db *gorm.DB) {
	db.Model(&Profile{}).Where("username = ?", "alice_dev").Update("bio", "Senior Go Architect")
	var updated Profile
	db.Where("username = ?", "alice_dev").First(&updated)
	fmt.Printf("Updated Bio: %s\n", updated.Bio)
}

func deleteProfile(db *gorm.DB) {
	// GORM utilizes Soft Delete by default if gorm.Model is used (sets DeletedAt timestamp)
	db.Where("username = ?", "bob_design").Delete(&Profile{})
	// Verification check
	var bob Profile
	result := db.Where("username = ?", "bob_design").First(&bob)
	if result.Error == gorm.ErrRecordNotFound {
		fmt.Println("Bob was successfully soft-deleted.")
	}
}
*/
