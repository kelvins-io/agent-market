package geo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type ipSBResponse struct {
	City        string  `json:"city"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	Country     string  `json:"country"`
	CountryCode string  `json:"country_code"`
	IP          string  `json:"ip"`
}

type ipAPIResponse struct {
	Status  string  `json:"status"`
	Message string  `json:"message"`
	City    string  `json:"city"`
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
}

// ApproximateByPublicIP looks up this host's egress IP location.
// Used when the browser Geolocation API is unavailable or times out
// (common when network location providers are unreachable).
func ApproximateByPublicIP(ctx context.Context) (PlaceResult, error) {
	if _, has := ctx.Deadline(); !has {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 6*time.Second)
		defer cancel()
	}

	if place, err := lookupIPSB(ctx); err == nil {
		return place, nil
	}
	return lookupIPAPI(ctx)
}

func lookupIPSB(ctx context.Context) (PlaceResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.ip.sb/geoip", nil)
	if err != nil {
		return PlaceResult{}, err
	}
	req.Header.Set("User-Agent", "agent-market/1.0")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return PlaceResult{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return PlaceResult{}, fmt.Errorf("ip.sb http %d", res.StatusCode)
	}
	var body ipSBResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return PlaceResult{}, err
	}
	if body.Latitude == 0 && body.Longitude == 0 {
		return PlaceResult{}, fmt.Errorf("ip.sb empty coordinates")
	}
	place := ReversePlace(body.Latitude, body.Longitude)
	place.Source = "ip"
	return place, nil
}

func lookupIPAPI(ctx context.Context) (PlaceResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://ip-api.com/json/?fields=status,message,city,lat,lon", nil)
	if err != nil {
		return PlaceResult{}, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return PlaceResult{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return PlaceResult{}, fmt.Errorf("ip-api http %d", res.StatusCode)
	}
	var body ipAPIResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return PlaceResult{}, err
	}
	if body.Status != "success" || (body.Lat == 0 && body.Lon == 0) {
		msg := body.Message
		if msg == "" {
			msg = "ip-api lookup failed"
		}
		return PlaceResult{}, fmt.Errorf("%s", msg)
	}
	place := ReversePlace(body.Lat, body.Lon)
	place.Source = "ip"
	return place, nil
}
