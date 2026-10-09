package handlers

import (
	"testing"

	"leiheV2/db"
)

func TestInitialStorageShelvesMatchShelfPlan(t *testing.T) {
	expected := []string{
		"A1L", "A1R", "A2L", "A2R", "A3L", "A3R", "A4L", "A4R",
		"B0", "B2L", "B2R", "B3L", "B3R", "B4L", "B4R",
		"C1L", "C1R", "C2L", "C2R", "C3L", "C3R", "CU",
	}
	initialShelves := db.InitialStorageShelves()
	if len(initialShelves) != len(expected) {
		t.Fatalf("expected %d initial shelves, got %d", len(expected), len(initialShelves))
	}
	for index, shelf := range expected {
		if initialShelves[index] != shelf {
			t.Errorf("initial shelf %d = %q, want %q", index, initialShelves[index], shelf)
		}
	}
}
