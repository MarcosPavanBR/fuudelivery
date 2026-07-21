package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"time"
)

type osrmResponse struct {
	Code   string `json:"code"`
	Routes []struct {
		Distance float64 `json:"distance"`
		Duration float64 `json:"duration"`
	} `json:"routes"`
}

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

	var osrmResp osrmResponse
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

func calculateDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadius = 6371
	lat1Rad := lat1 * math.Pi / 180
	lon1Rad := lon1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	lon2Rad := lon2 * math.Pi / 180

	dlat := lat2Rad - lat1Rad
	dlon := lon2Rad - lon1Rad

	a := math.Sin(dlat/2)*math.Sin(dlat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(dlon/2)*math.Sin(dlon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadius * c
}
