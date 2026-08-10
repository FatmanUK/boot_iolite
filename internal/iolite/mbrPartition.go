package iolite

import (
	"fmt"
)

type MBRPartition struct {
	ID         uint   `gorm:"primaryKey"`
	TemplateID uint   `gorm:"not null;index"`
	Start      uint64 `gorm:"not null"`
	Size       uint64 `gorm:"not null"`
	Type       string `gorm:"not null"`
	IsBootable bool   `gorm:"not null"`
}

func (l MBRPartition) String(n string, i uint8) string {
	rv := fmt.Sprintf(`/dev/%s%d : `, n, i)
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
