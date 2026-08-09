package iolite

type MBRPartition struct {
	ID         uint   `gorm:"primaryKey"`
	TemplateID uint   `gorm:"not null;index"`
	Start      uint64 `gorm:"not null"`
	Size       uint64 `gorm:"not null"`
	Type       string `gorm:"not null"`
	IsBootable bool   `gorm:"not null"`
}
