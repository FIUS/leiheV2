package handlers

import "testing"

func TestKnownStorageLocationsMatchShelfPlan(t *testing.T) {
	for _, location := range []string{
		"A1L", "A1R", "A2L", "A2R", "A3L", "A3R", "A4L", "A4R",
		"B0", "B2L", "B2R", "B3L", "B3R", "B4L", "B4R",
		"C1L", "C1R", "C2L", "C2R", "C3L", "C3R", "C4L", "C4R",
	} {
		if !isKnownStorageLocation(location) {
			t.Errorf("expected %q to be a known storage location", location)
		}
	}

	for _, location := range []string{"", "B1L", "A5L", "C1", "a1l"} {
		if isKnownStorageLocation(location) {
			t.Errorf("expected %q to be rejected as an unknown storage location", location)
		}
	}
}
