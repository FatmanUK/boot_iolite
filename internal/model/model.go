// Package model holds the GORM data model: a Profile (one machine)
// points at a Layout, which maps device names to partition
// Templates, each of which lists Partitions.
package model

import (
	"fmt"

	"gorm.io/gorm"
)

// Profile describes one machine that iolite may install.
type Profile struct {
	ID               uint    `gorm:"primaryKey"`
	LayoutID         uint    `gorm:"not null;index"`
	Layout           *Layout `gorm:"foreignKey:LayoutID"`
	HardwareAddress  string  `gorm:"unique;not null"`
	IPAddress        string  `gorm:"unique;not null"`
	FQDN             string  `gorm:"unique;not null"`
	Distro           string
	Repo             string
	Packages         string
	AuthorizedKeys   string
	RootPasswordHash string
	IsBuildEnabled   bool
}

// Layout maps device names (sda, vda, nvme0n1) to Templates.
type Layout struct {
	ID         uint        `gorm:"primaryKey"`
	Name       string      `gorm:"unique;not null"`
	TemplatesM []*Template `gorm:"many2many:layout_templates;"`
	// Templates is keyed by device name; synced by the hooks.
	Templates map[string]*Template `gorm:"-"`
}

// LayoutTemplate is the join table; MapKey is the device name.
// No OnDelete:CASCADE: a shared Template survives Layout deletion.
type LayoutTemplate struct {
	ID         uint   `gorm:"primaryKey"`
	LayoutID   uint   `gorm:"not null;index"`
	TemplateID uint   `gorm:"not null;index"`
	MapKey     string `gorm:"not null;index"`
	Template   *Template
}

// Template is a partition table: Label is "dos" or "gpt".
type Template struct {
	ID         uint         `gorm:"primaryKey"`
	Name       string       `gorm:"unique;not null"`
	Label      string       `gorm:"not null;default:dos"`
	SectorSize uint16       `gorm:"not null"`
	Partitions []*Partition `gorm:"constraint:OnDelete:CASCADE"`
}

// Partition is one sfdisk line. Start and Size are sectors; Size 0
// means "to the end of the disk". Type is whatever sfdisk accepts
// (hex for dos, alias or GUID for gpt). Filesystem is ext4, xfs,
// vfat, swap or empty for none.
type Partition struct {
	ID           uint   `gorm:"primaryKey"`
	TemplateID   uint   `gorm:"not null;index"`
	Start        uint64 `gorm:"not null"`
	Size         uint64 `gorm:"not null"`
	Type         string `gorm:"not null"`
	Name         string
	Filesystem   string
	MountPoint   string
	MountOptions string
	IsBootable   bool `gorm:"not null"`
}

// AfterSave persists each Template and upserts a join row per key.
func (l *Layout) AfterSave(tx *gorm.DB) error {
	for key, tmpl := range l.Templates {
		if err := tx.Save(tmpl).Error; err != nil {
			return fmt.Errorf("save template %q: %w", key, err)
		}
		join := LayoutTemplate{
			LayoutID:   l.ID,
			TemplateID: tmpl.ID,
			MapKey:     key,
		}
		err := tx.Where("layout_id = ? AND map_key = ?", l.ID, key).
			Assign(join).
			FirstOrCreate(&LayoutTemplate{}).Error
		if err != nil {
			return fmt.Errorf("link template %q: %w", key, err)
		}
	}
	return nil
}

// AfterFind rebuilds Templates from the join table's MapKey.
func (l *Layout) AfterFind(tx *gorm.DB) error {
	var joins []LayoutTemplate
	err := tx.Where("layout_id = ?", l.ID).Find(&joins).Error
	if err != nil {
		return err
	}
	byID := make(map[uint]*Template, len(l.TemplatesM))
	for _, t := range l.TemplatesM {
		byID[t.ID] = t
	}
	l.Templates = make(map[string]*Template, len(joins))
	for _, j := range joins {
		if t, ok := byID[j.TemplateID]; ok {
			l.Templates[j.MapKey] = t
		}
	}
	return nil
}
