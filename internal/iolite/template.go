package iolite

import (
	"encoding/base64"
	"fmt"
	"strings"
)

type Template struct {
	ID         uint            `gorm:"primaryKey"`
	Name       string          `gorm:"unique;not null"`
	SectorSize uint16          `gorm:"not null"`
	Partitions []*MBRPartition `gorm:"constraint:OnDelete:CASCADE"`
}

func (t Template) String(n string) string {
	mbrHeaderFmt := `label: dos
device: /dev/%s
unit: sectors
sector-size: %d

`
	header := fmt.Sprintf(mbrHeaderFmt, n, t.SectorSize)
	lines := []string{}
	for i := 0; i < len(t.Partitions); i++ {
		s := t.Partitions[i].String(n, uint8(i+1))
		lines = append(lines, s)
	}
	pt := fmt.Sprintf(`%s%s`, header, strings.Join(lines, "\n"))
	return base64.StdEncoding.EncodeToString([]byte(pt))
}

// many-many joiner table
// note: no OnDelete:CASCADE — deleting a layout must not delete
// a shared template
// Note this needs to be here. Though it's not explicitly referenced,
// GORM is using it to do some real work.
type LayoutTemplate struct {
	ID         uint   `gorm:"primaryKey"`
	LayoutID   uint   `gorm:"not null;index"`
	TemplateID uint   `gorm:"not null;index"`
	MapKey     string `gorm:"not null;index"`
	Template   *Template
}
