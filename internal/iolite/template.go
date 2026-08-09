package iolite

type Template struct {
	ID         uint           `gorm:"primaryKey"`
	Name       string         `gorm:"not null"`
	SectorSize uint16         `gorm:"not null"`
	Partitions []MBRPartition `gorm:"constraint:OnDelete:CASCADE"`
}

// many-many joiner table
// note: no OnDelete:CASCADE — deleting a layout must not delete
// a shared template
type LayoutTemplate struct {
	ID         uint     `gorm:"primaryKey"`
	LayoutID   uint     `gorm:"not null;index"`
	TemplateID uint     `gorm:"not null;index"`
	MapKey     string   `gorm:"not null;index"`
	Template   Template
}
