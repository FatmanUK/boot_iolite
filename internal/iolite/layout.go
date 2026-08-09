package iolite

import (
	"fmt"
	"gorm.io/gorm"
)

type Layout struct {
	ID           uint             `gorm:"primaryKey"`
	Name         string           `gorm:"not null"`
	TemplateRefs []LayoutTemplate `gorm:"constraint:OnDelete:CASCADE"`
	Templates map[string]Template `gorm:"-"` // convenience view; synced via hooks
}

// BeforeSave ensures every Template in the map is persisted (reusing
// existing rows by ID, creating new ones if ID==0), then rebuilds
// TemplateRefs to point at them under the right map key.
func (l *Layout) BeforeSave(tx *gorm.DB) error {
	refs := make([]LayoutTemplate, 0, len(l.Templates))
	for key, tmpl := range l.Templates {
		err := tx.Save(&tmpl).Error
		m := fmt.Errorf("save template %q: %w", key, err)
		if err != nil { // Save = insert if ID==0, else update
			return m
		}
		refs = append(refs, LayoutTemplate{
			TemplateID: tmpl.ID,
			MapKey:     key,
		})
		l.Templates[key] = tmpl // write back the ID
	}
	l.TemplateRefs = refs
	return nil
}

// AfterFind rebuilds the map from the loaded join rows.
func (l *Layout) AfterFind(tx *gorm.DB) error {
	l.Templates = make(map[string]Template, len(l.TemplateRefs))
	for _, ref := range l.TemplateRefs {
		l.Templates[ref.MapKey] = ref.Template
	}
	return nil
}
