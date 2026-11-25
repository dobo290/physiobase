package store

import (
	"context"
	"testing"
)

// TestSeedDemoOnce: eine frisch gestartete Demo hat sofort Patienten,
// ein zweiter Start legt nichts doppelt an.
func TestSeedDemoOnce(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	seeded, err := s.SeedDemoOnce(ctx)
	if err != nil || !seeded {
		t.Fatalf("erstes Anlegen: %v %v", seeded, err)
	}
	ps, err := s.Patients(ctx, "", false)
	if err != nil || len(ps) != 3 {
		t.Fatalf("Patienten: %v (%d), erwartet 3", err, len(ps))
	}
	again, err := s.SeedDemoOnce(ctx)
	if err != nil || again {
		t.Fatalf("zweiter Aufruf: %v %v", again, err)
	}
	if ps, _ := s.Patients(ctx, "", false); len(ps) != 3 {
		t.Errorf("Patienten nach zweitem Aufruf: %d", len(ps))
	}
	// Assessments sind Teil der Demo-Daten.
	hits, err := s.SearchTests(ctx, "Berg Balance", 5)
	if err != nil || len(hits) == 0 {
		t.Fatalf("Assessments fehlen: %v", err)
	}
}
