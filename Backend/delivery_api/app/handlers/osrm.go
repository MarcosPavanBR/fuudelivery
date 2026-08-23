package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

// osrmBaseURL retorna a base do servidor OSRM configurada.
// Configurar via OSRM_BASE_URL em produção — o default é o servidor DEMO
// público (router.project-osrm.org), que proíbe uso em produção e tem rate
// limit agressivo. Para volume real, rode sua instância (é open source) ou
// use um provedor gerenciado e aponte OSRM_BASE_URL para ela.
// Ex: OSRM_BASE_URL=https://osrm.meudominio.com
func osrmBaseURL() string {
	if base := os.Getenv("OSRM_BASE_URL"); base != "" {
		return base
	}
	return "https://router.project-osrm.org" // demo — apenas dev
}

// clientOSRM reutilizado entre chamadas (criar http.Client por request
// desperdiça conexões no pool).
var clientOSRM = &http.Client{Timeout: 5 * time.Second}

type OSRMResponse struct {
	Code   string `json:"code"`
	Routes []struct {
		Distance float64 `json:"distance"` // meters
		Duration float64 `json:"duration"` // seconds
	} `json:"routes"`
}

// getOSRMDistance consulta o servidor OSRM (ver osrmBaseURL) para obter a
// distância real de direção. Retorna distância em km e duração em minutos.
// Em caso de erro retorna 0,0 (ok=false) para o chamador usar Haversine.
func getOSRMDistance(lat1, lon1, lat2, lon2 float64) (distanceKm float64, durationMin float64, ok bool) {
	url := fmt.Sprintf(
		"%s/route/v1/driving/%f,%f;%f,%f?overview=false",
		osrmBaseURL(), lon1, lat1, lon2, lat2,
	)

	resp, err := clientOSRM.Get(url)
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
