package iolite

import (
	"gorm.io/gorm"
	idhcp "iolite/internal/dhcp"
	"net"
)

type Profile struct {
	gorm.Model
	HardwareAddress string `gorm:"unique;not null"`
	IPAddress       string `gorm:"unique;not null"`
	FQDN            string `gorm:"unique;not null"`
	Distro          string
	IsBuildEnabled  bool
	//Layout          string
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

func createLayouts(logs chan string) {
	var t [4]DiskTemplate
	db.Where("name = ?", "MBR_5_Swap+LVM").First(&(t[0]))
	db.Where("name = ?", "MBR_5_Linux_Bootable").First(&(t[1]))
	db.Where("name = ?", "MBR_5_LVM").First(&(t[2]))
	db.Where("name = ?", "MBR_5_LVM").First(&(t[3]))
	templateMap := map[string]DiskTemplate{
		"vda": t[0],
		"sda": t[1],
		"sdb": t[2],
		"sdc": t[3],
	}
	templateKeys, templateValues := UnzipMap(templateMap)
	layout := DiskLayout{
		Name: "TestVMLayout",
		TemplateKeys: templateKeys,
		TemplateValues: templateValues,
	}
	db.Create(&layout)
	logs <- "Layouts saved successfully."
}

func createProfiles(logs chan string) {
	glarg := Profile{
		HardwareAddress: "52:54:00:76:de:e5",
		IPAddress: "172.168.16.32",
		FQDN: "glarg01.dreamtrack.net",
		Distro: "voidLinux",
		IsBuildEnabled: true,
		Layout: "TestVMLayout",
	}
	vlissides := Profile{
		HardwareAddress: "94:c6:91:a0:36:98",
		IPAddress: "172.168.16.214",
		FQDN: "vlissides.dreamtrack.net",
		Distro: "voidLinux",
		IsBuildEnabled: false,
		Layout: "TestVMLayout",
	}
	db.Create(&glarg)
	db.Create(&vlissides)
	logs <- "Profiles saved successfully."
}

func createTemplates(logs chan string) {
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
	fiveLinux := DiskTemplate{
		Name: "MBR_5_Linux_Bootable",
		Type: "mbr",
		SectorSize: 512,
		Partitions: []MBRPartitionTemplate{
			MBRPartitionTemplate{
				Start: 63,
				Size: 10485697,
				Type: "83",
				IsBootable: true,
			},
		},
	}
	fiveSwapOneLvmRest := DiskTemplate{
		Name: "MBR_5_Swap+LVM",
		Type: "mbr",
		SectorSize: 512,
		Partitions: []MBRPartitionTemplate{
			MBRPartitionTemplate{
				Start: 63,
				Size: 2097152,
				Type: "82",
				IsBootable: false,
			},
			MBRPartitionTemplate{
				Start: 2097215,
				Size: 8388545,
				Type: "8e",
				IsBootable: false,
			},
		},
	}
	db.Create(&fiveLvm)
	db.Create(&fiveLinux)
	db.Create(&fiveSwapOneLvmRest)
	logs <- "Templates saved successfully."
}

type MBRPartitionTemplate struct {
	ID uint64 `gorm:"unique;not null;autoIncrement"`
	Disk uint
	Start uint64 `gorm:"not null"`
	Size uint64 `gorm:"not null"`
	Type string `gorm:"not null"`
	IsBootable bool `gorm:"default:false"`
}

func (l MBRPartitionTemplate) String(n string, i uint8) string {
	mbrLinePrefixFmt := `/dev/%s%d : `
	rv := fmt.Sprintf(mbrLinePrefixFmt, n, i)
	rv += fmt.Sprintf(
		`start=%12d, size=%12d, type=%s`,
		l.Start,
		l.Size,
		l.Type,
	)
	if l.IsBootable {
		rv += `, bootable`
	}
	return rv
}

type DiskTemplate struct {
	gorm.Model
	Name string `gorm:"unique;not null"`
	Layout uint
	Type string `gorm:"not null;default:'mbr'"`
	SectorSize uint16 `gorm:"not null;default:512"`
	Partitions []MBRPartitionTemplate `gorm:"foreignKey:Disk"`
}

func (d DiskTemplate) String(n string) string {
	mbrHeaderFmt := `label: dos
device: /dev/%s
unit: sectors
sector-size: %d

`
	header := fmt.Sprintf(mbrHeaderFmt, n, d.SectorSize)
	lines := []string{}
	for i := 0; i < len(d.Partitions); i++ {
		s := d.Partitions[i].String(n, uint8(i + 1))
		lines = append(lines, s)
	}
	return header + strings.Join(lines, "\n")
}

// zip keys and values into a map[string]DiskTemplate to use
type DiskLayout struct {
	gorm.Model
	Name string              `gorm:"unique;not null"`
	TemplateKeys []string  `gorm:"serializer:json"`
	TemplateValues []DiskTemplate `gorm:"foreignKey:Layout"`
}

func (d DiskLayout) String(n string) string {
	rv := []string{}
	db.Where("name = ?", n).First(&d)
	for i, t := range d.TemplateValues {
		rv = append(rv, t.String(d.TemplateKeys[i]))
	}
	return `{"disks":["` + strings.Join(rv, `","`) + `]"}`
}

func readProfiles(db *gorm.DB) {
	var allProfiles []Profile
	db.Find(&allProfiles)
	for _, p := range allProfiles {
		fmt.Printf("ID: %d | User: %s | Age: %d | Bio: %s\n", p.ID, p.Username, p.Age, p.Bio)
	}
	// Get a single record by field
	var singleProfile Profile
	db.Where("username = ?", "alice_dev").First(&singleProfile)
	fmt.Printf("Found single user: %s (%s)\n", singleProfile.Username, singleProfile.Email)
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

swap/root

/ # echo "bGFiZWw6IGRvcwpsYWJlbC1pZDogMHgwMDAwMDAwMApkZXZpY2U6IC9kZXYvc2RhCnVuaXQ6IHNlY3RvcnMKc2VjdG9yLXNpemU6IDUxMgoKL2Rldi9zZGExIDogc3RhcnQ9ICAgICAgICAgIDYzLCBzaXplPSAgICAxMDQ4NTY5NywgdHlwZT04MywgYm9vdGFibGUK" | base64 -d | sfdisk /dev/sda

`label: dos
device: /dev/sda
unit: sectors
sector-size: 512

/dev/sda1 : start=          63, size=    10485697, type=83, bootable
`

boot

/ # echo "bGFiZWw6IGRvcwpsYWJlbC1pZDogMHgwMDAwMDAwMApkZXZpY2U6IC9kZXYvc2RhCnVuaXQ6IHNlY3RvcnMKc2VjdG9yLXNpemU6IDUxMgoKL2Rldi9zZGExIDogc3RhcnQ9ICAgICAgICAgIDYzLCBzaXplPSAgICAgMjA5NzE1MiwgdHlwZT04MgovZGV2L3NkYTIgOiBzdGFydD0gICAgIDIwOTcyMTUsIHNpemU9ICAgICA4Mzg4NTQ1LCB0eXBlPThlCg==" | base64 -d | sfdisk /dev/sda

`label: dos
device: /dev/sda
unit: sectors
sector-size: 512

/dev/sda1 : start=          63, size=     2097152, type=82
/dev/sda2 : start=     2097215, size=     8388545, type=8e
`

var

/ # echo "bGFiZWw6IGRvcwpsYWJlbC1pZDogMHgwMDAwMDAwMApkZXZpY2U6IC9kZXYvc2RhCnVuaXQ6IHNlY3RvcnMKc2VjdG9yLXNpemU6IDUxMgoKL2Rldi9zZGExIDogc3RhcnQ9ICAgICAgICAgIDYzLCBzaXplPSAgICAxMDQ4NTY5NywgdHlwZT04ZQo=" | base64 -d | sfdisk /dev/sda

`label: dos
device: /dev/sda
unit: sectors
sector-size: 512

/dev/sda1 : start=          63, size=    10485697, type=8e
`
*/
