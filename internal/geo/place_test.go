package geo

import "testing"

func TestReversePlaceBeijing(t *testing.T) {
	p := ReversePlace(39.9042, 116.4074)
	if p.Name != "北京" {
		t.Fatalf("want 北京, got %q", p.Name)
	}
}

func TestReversePlaceNearShanghai(t *testing.T) {
	p := ReversePlace(31.23, 121.47)
	if p.City != "上海" {
		t.Fatalf("want city 上海, got %q", p.City)
	}
}
