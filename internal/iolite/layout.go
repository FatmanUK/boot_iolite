package iolite

import (
	"fmt"
	"gorm.io/gorm"
	"strings"
)

type Layout struct {
	ID         uint                 `gorm:"primaryKey"`
	Name       string               `gorm:"unique;not null"`
	TemplatesM []*Template          `gorm:"many2many:layout_templates;"`
	Templates  map[string]*Template `gorm:"-"` // convenience view; synced via hooks
}

func (l *Layout) String() string {
	var rv []string
	for k, v := range l.Templates {
		rv = append(rv, fmt.Sprintf(`"%s":"%s"`, k, v.String(k)))
	}
	return fmt.Sprintf(`{"layout":[{%s}]}`, strings.Join(rv, `},{`))
}

// AfterSave persists/updates each Template (reusing existing rows by ID)
// and rewrites this layout's join rows to match the current map.
func (l *Layout) AfterSave(tx *gorm.DB) error {
	for key, tmpl := range l.Templates {
		if err := tx.Save(tmpl).Error; err != nil {
			return fmt.Errorf("save template %q: %w", key, err)
		}
		join := LayoutTemplate{LayoutID: l.ID, TemplateID: tmpl.ID, MapKey: key}
		if err := tx.Where("layout_id = ? AND map_key = ?", l.ID, key).
			Assign(join).
			FirstOrCreate(&LayoutTemplate{}).Error; err != nil {
			return fmt.Errorf("link template %q: %w", key, err)
		}
	}
	return nil
}

// AfterFind rebuilds the map using the join table's MapKey for each linked Template.
func (l *Layout) AfterFind(tx *gorm.DB) error {
	var joins []LayoutTemplate
	if err := tx.Where("layout_id = ?", l.ID).Find(&joins).Error; err != nil {
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
