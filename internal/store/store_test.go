package store

import (
	"errors"
	"path/filepath"
	"testing"
)

func open(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}
	seeded, err := s.SeedIfEmpty("../../seed/profiles.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !seeded {
		t.Fatal("expected seed to run on an empty database")
	}
	return s
}

func TestSeedOnce(t *testing.T) {
	s := open(t)
	seeded, err := s.SeedIfEmpty("../../seed/profiles.sql")
	if err != nil {
		t.Fatal(err)
	}
	if seeded {
		t.Fatal("seed ran twice")
	}
}

func TestProfileByMAC(t *testing.T) {
	s := open(t)
	p, err := s.ProfileByMAC("52:54:00:76:DE:E5")
	if err != nil {
		t.Fatal(err)
	}
	if p.FQDN != "glarg01.dreamtrack.net" {
		t.Fatalf("fqdn %q", p.FQDN)
	}
	tmpl := p.Layout.Templates["vda"]
	if tmpl == nil {
		t.Fatalf("layout keys %v", p.Layout.Templates)
	}
	if tmpl.Label != "dos" || len(tmpl.Partitions) != 2 {
		t.Fatalf("template %+v", tmpl)
	}
	if tmpl.Partitions[0].Start != 2048 {
		t.Fatalf("partitions not ordered by start")
	}
	_, err = s.ProfileByMAC("00:00:00:00:00:00")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestMarkDone(t *testing.T) {
	s := open(t)
	ps, err := s.EnabledProfiles()
	if err != nil || len(ps) != 1 {
		t.Fatalf("enabled %d, err %v", len(ps), err)
	}
	if err := s.MarkDone("52:54:00:76:de:e5"); err != nil {
		t.Fatal(err)
	}
	ps, err = s.EnabledProfiles()
	if err != nil || len(ps) != 0 {
		t.Fatalf("enabled %d after done, err %v", len(ps), err)
	}
	err = s.MarkDone("00:00:00:00:00:00")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}
