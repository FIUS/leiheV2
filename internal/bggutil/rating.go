package bggutil

import "strconv"

// ParseRating returns the BGG average on its native 1–10 scale.
func ParseRating(value string) (float64, bool) {
	rating, err := strconv.ParseFloat(value, 64)
	if err != nil || rating < 1 || rating > 10 {
		return 0, false
	}
	return rating, true
}
