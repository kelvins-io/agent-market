package geo

import "testing"

func TestDistanceKmBeijingTiananmen(t *testing.T) {
	// Roughly Tiananmen to Olympic Park — should be under 15 km
	d := DistanceKm(39.9087, 116.3975, 40.0020, 116.4800)
	if d < 5 || d > 20 {
		t.Fatalf("unexpected distance %.2f km", d)
	}
}

func TestDistanceKmZero(t *testing.T) {
	if d := DistanceKm(39.9, 116.4, 39.9, 116.4); d != 0 {
		t.Fatalf("want 0, got %v", d)
	}
}
