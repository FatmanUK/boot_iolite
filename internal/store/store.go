// Package store opens the profiles database and answers the
// queries the servers and the admin UI need.
package store

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"iolite/internal/model"
)

// ErrNotFound reports a MAC with no profile row.
var ErrNotFound = errors.New("profile not found")

// Store wraps the GORM handle.
type Store struct {
	db *gorm.DB
}

// Open connects to the SQLite file at path (created if absent).
func Open(path string) (*Store, error) {
	cfg := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	}
	db, err := gorm.Open(sqlite.Open(path), cfg)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	return &Store{db: db}, nil
}

// DB exposes the handle for the admin UI's CRUD.
func (s *Store) DB() *gorm.DB {
	return s.db
}

// Migrate creates or updates the schema.
func (s *Store) Migrate() error {
	err := s.db.SetupJoinTable(&model.Layout{}, "TemplatesM",
		&model.LayoutTemplate{})
	if err != nil {
		return fmt.Errorf("join table: %w", err)
	}
	err = s.db.AutoMigrate(
		&model.Profile{},
		&model.Layout{},
		&model.LayoutTemplate{},
		&model.Template{},
		&model.Partition{},
	)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}

// SeedIfEmpty runs the SQL file when there are no profiles yet and
// reports whether it did.
func (s *Store) SeedIfEmpty(path string) (bool, error) {
	var n int64
	err := s.db.Model(&model.Profile{}).Count(&n).Error
	if err != nil {
		return false, err
	}
	if n > 0 {
		return false, nil
	}
	src, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		for _, stmt := range statements(string(src)) {
			if err := tx.Exec(stmt).Error; err != nil {
				return fmt.Errorf("seed: %w", err)
			}
		}
		return nil
	})
	return err == nil, err
}

// statements drops comment lines and splits on semicolons.
func statements(src string) []string {
	var keep []string
	for _, l := range strings.Split(src, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(l), "--") {
			keep = append(keep, l)
		}
	}
	var out []string
	for _, st := range strings.Split(strings.Join(keep, "\n"), ";") {
		if st = strings.TrimSpace(st); st != "" {
			out = append(out, st)
		}
	}
	return out
}

func (s *Store) preload() *gorm.DB {
	order := func(db *gorm.DB) *gorm.DB {
		return db.Order("start")
	}
	return s.db.Preload("Layout.TemplatesM.Partitions", order)
}

// ProfileByMAC loads a profile with its whole layout.
func (s *Store) ProfileByMAC(mac string) (*model.Profile, error) {
	var p model.Profile
	err := s.preload().
		Where("hardware_address = ?", strings.ToLower(mac)).
		First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// EnabledProfiles lists profiles that DHCP should answer.
func (s *Store) EnabledProfiles() ([]model.Profile, error) {
	var ps []model.Profile
	err := s.preload().
		Where("is_build_enabled = ?", true).
		Order("id").
		Find(&ps).Error
	return ps, err
}

// MarkDone disables a profile after a successful install.
func (s *Store) MarkDone(mac string) error {
	res := s.db.Model(&model.Profile{}).
		Where("hardware_address = ?", strings.ToLower(mac)).
		Update("is_build_enabled", false)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
