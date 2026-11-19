package geo

import (
	"fmt"
	"math"
)

type city struct {
	Name string
	Lat  float64
	Lng  float64
}

// Major cities used for offline reverse-geocoding (nearest match).
var cities = []city{
	{"北京", 39.9042, 116.4074},
	{"天津", 39.0842, 117.2009},
	{"上海", 31.2304, 121.4737},
	{"广州", 23.1291, 113.2644},
	{"深圳", 22.5431, 114.0579},
	{"杭州", 30.2741, 120.1551},
	{"南京", 32.0603, 118.7969},
	{"成都", 30.5728, 104.0668},
	{"重庆", 29.5630, 106.5516},
	{"武汉", 30.5928, 114.3055},
	{"西安", 34.3416, 108.9398},
	{"苏州", 31.2989, 120.5853},
	{"郑州", 34.7466, 113.6254},
	{"长沙", 28.2282, 112.9388},
	{"青岛", 36.0671, 120.3826},
	{"大连", 38.9140, 121.6147},
	{"厦门", 24.4798, 118.0894},
	{"香港", 22.3193, 114.1694},
	{"台北", 25.0330, 121.5654},
}

// PlaceResult is a human-readable location label.
type PlaceResult struct {
	Name       string  `json:"name"`
	City       string  `json:"city"`
	DistanceKm float64 `json:"distance_km"`
	Lat        float64 `json:"lat"`
	Lng        float64 `json:"lng"`
	Source     string  `json:"source"`
}

// ReversePlace returns the nearest known city name for lat/lng.
// Within ~80km of a city center it returns that city; otherwise "附近·城市名".
func ReversePlace(lat, lng float64) PlaceResult {
	best := cities[0]
	bestD := math.MaxFloat64
	for _, c := range cities {
		d := DistanceKm(lat, lng, c.Lat, c.Lng)
		if d < bestD {
			bestD = d
			best = c
		}
	}
	name := best.Name
	if bestD > 80 {
		name = fmt.Sprintf("靠近%s", best.Name)
	}
	return PlaceResult{
		Name:       name,
		City:       best.Name,
		DistanceKm: round2(bestD),
		Lat:        lat,
		Lng:        lng,
		Source:     "local",
	}
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
