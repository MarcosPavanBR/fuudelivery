package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

type OSRMResponse struct {
	Code   string `json:"code"`
	Routes []struct {
		Distance float64 `json:"distance"` // meters
		Duration float64 `json:"duration"` // seconds
	} `json:"routes"`
}

// getOSRMDistance calls the public OSRM demo server for real driving distance.
// Returns distance in km and duration in minutes.
// Falls back to 0,0 on error so caller can use Haversine.
func getOSRMDistance(lat1, lon1, lat2, lon2 float64) (distanceKm float64, durationMin float64, ok bool) {
	url := fmt.Sprintf(
		"https://router.project-osrm.org/route/v1/driving/%f,%f;%f,%f?overview=false",
		lon1, lat1, lon2, lat2,
	)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		log.Printf("[OSRM] Request failed: %v, falling back to Haversine", err)
		return 0, 0, false
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[OSRM] Read body failed: %v", err)
		return 0, 0, false
	}

	var osrmResp OSRMResponse
	if err := json.Unmarshal(body, &osrmResp); err != nil {
		log.Printf("[OSRM] Unmarshal failed: %v", err)
		return 0, 0, false
	}

	if osrmResp.Code != "Ok" || len(osrmResp.Routes) == 0 {
		log.Printf("[OSRM] No route found: code=%s", osrmResp.Code)
		return 0, 0, false
	}

	route := osrmResp.Routes[0]
	distanceKm = route.Distance / 1000.0
	durationMin = route.Duration / 60.0

	return distanceKm, durationMin, true
}
