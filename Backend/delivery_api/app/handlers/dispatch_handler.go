package handlers

import (
	"math"
	"strconv"
	"time"

	"github.com/carloshomar/vercardapio/delivery_api/app/dto"
	"github.com/carloshomar/vercardapio/delivery_api/app/services"
	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// DispatchHandler expoe os endpoints do motor de matching.
type DispatchHandler struct {
	CourierStore     *services.CourierStore
	Matching         *services.MatchingEngine
	SolicitationColl *mongo.Collection
}

// NewDispatchHandler cria um novo handler de dispatch.
func NewDispatchHandler(courierStore *services.CourierStore, matching *services.MatchingEngine, db *mongo.Database) *DispatchHandler {
	return &DispatchHandler{
		CourierStore:     courierStore,
		Matching:         matching,
		SolicitationColl: db.Collection("solicitations"),
	}
}

// UpdateLocation recebe a localizacao do entregador e atualiza no store.
// POST /dispatch/location
func (h *DispatchHandler) UpdateLocation(c *fiber.Ctx) error {
	var req dto.LocationUpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.DeliverymanID == 0 || (req.Lat == 0 && req.Lng == 0) {
		return c.Status(400).JSON(fiber.Map{"error": "deliveryman_id, lat, lng required"})
	}

	if req.Status == "" {
		req.Status = "available"
	}

	h.CourierStore.UpdateLocation(req.DeliverymanID, req.Name, req.Lat, req.Lng, req.Status)

	return c.JSON(fiber.Map{
		"message":        "Location updated",
		"deliveryman_id": req.DeliverymanID,
		"status":         req.Status,
	})
}

// SetCourierStatus atualiza o status de um entregador.
// POST /dispatch/status
func (h *DispatchHandler) SetCourierStatus(c *fiber.Ctx) error {
	var req struct {
		DeliverymanID int64  `json:"deliveryman_id"`
		Status        string `json:"status"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.DeliverymanID == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "deliveryman_id required"})
	}

	validStatuses := map[string]bool{"available": true, "busy": true, "offline": true}
	if !validStatuses[req.Status] {
		return c.Status(400).JSON(fiber.Map{
			"error":   "Invalid status",
			"allowed": []string{"available", "busy", "offline"},
		})
	}

	h.CourierStore.SetCourierStatus(req.DeliverymanID, req.Status)

	return c.JSON(fiber.Map{
		"message":        "Status updated",
		"deliveryman_id": req.DeliverymanID,
		"status":         req.Status,
	})
}

// TriggerDispatch tenta encontrar um entregador para um pedido.
// POST /dispatch/trigger
func (h *DispatchHandler) TriggerDispatch(c *fiber.Ctx) error {
	var req dto.DispatchRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.OrderID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "order_id required"})
	}

	// Busca o pedido no MongoDB para obter coordenadas do estabelecimento
	var solicitation dto.OrderDTO
	err := h.SolicitationColl.FindOne(c.Context(), bson.M{"orderid": req.OrderID}).Decode(&solicitation)
	if err != nil {
		if req.Lat == 0 && req.Lng == 0 {
			return c.Status(404).JSON(fiber.Map{"error": "Order not found and no coordinates provided"})
		}
		solicitation = dto.OrderDTO{
			OrderId: req.OrderID,
			Establishment: dto.EstablishmentDTO{
				Lat:  req.Lat,
				Long: req.Lng,
			},
		}
	}

	if req.Force {
		candidates := h.CourierStore.FindNearby(
			solicitation.Establishment.Lat,
			solicitation.Establishment.Long,
			50,
			1,
		)
		if len(candidates) > 0 {
			best := candidates[0]
			dist := haversine(solicitation.Establishment.Lat, solicitation.Establishment.Long, best.Lat, best.Lng)
			h.CourierStore.SetOrdersCount(best.DeliverymanID, best.CurrentOrders+1)

			return c.JSON(dto.DispatchResponse{
				OrderID:     req.OrderID,
				Matched:     true,
				CourierID:   best.DeliverymanID,
				CourierName: best.Name,
				DistanceKm:  dist,
			})
		}
	}

	result := h.Matching.AttemptMatch(&solicitation)
	if result == nil {
		return c.Status(500).JSON(fiber.Map{"error": "Matching engine returned nil"})
	}

	resp := dto.DispatchResponse{
		OrderID:  req.OrderID,
		Matched:  result.Matched,
		Fallback: result.Fallback,
		DLQSize:  h.Matching.DLQ.Len(),
	}

	if result.Matched {
		resp.CourierID = result.CourierID
		resp.CourierName = result.CourierName
		resp.DistanceKm = result.DistanceKm
	}

	return c.JSON(resp)
}

// NearbyCouriers lista entregadores disponiveis proximos a uma localizacao.
// GET /dispatch/nearby?lat=-23.5505&lng=-46.6333&radius=5
func (h *DispatchHandler) NearbyCouriers(c *fiber.Ctx) error {
	lat, err := strconv.ParseFloat(c.Query("lat"), 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "lat required"})
	}
	lng, err := strconv.ParseFloat(c.Query("lng"), 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "lng required"})
	}
	radius := 10.0
	if r := c.Query("radius"); r != "" {
		if parsed, err := strconv.ParseFloat(r, 64); err == nil {
			radius = parsed
		}
	}

	couriers := h.CourierStore.FindNearby(lat, lng, radius, 20)

	type CourierView struct {
		ID            int64   `json:"id"`
		Name          string  `json:"name"`
		Lat           float64 `json:"lat"`
		Lng           float64 `json:"lng"`
		Status        string  `json:"status"`
		CurrentOrders int     `json:"current_orders"`
		DistanceKm    float64 `json:"distance_km"`
	}

	view := make([]CourierView, 0, len(couriers))
	for _, c := range couriers {
		view = append(view, CourierView{
			ID:            c.DeliverymanID,
			Name:          c.Name,
			Lat:           c.Lat,
			Lng:           c.Lng,
			Status:        c.Status,
			CurrentOrders: c.CurrentOrders,
			DistanceKm:    haversine(lat, lng, c.Lat, c.Lng),
		})
	}

	return c.JSON(fiber.Map{
		"couriers": view,
		"total":    len(view),
		"lat":      lat,
		"lng":      lng,
		"radius":   radius,
	})
}

// GetDLQ retorna o estado atual da dead-letter queue.
// GET /dispatch/dlq
func (h *DispatchHandler) GetDLQ(c *fiber.Ctx) error {
	orders := h.Matching.DLQ.List()

	type DLQView struct {
		OrderID    string `json:"order_id"`
		RetryCount int    `json:"retry_count"`
		AgeSeconds int64  `json:"age_seconds"`
	}

	now := time.Now().UnixMilli()
	view := make([]DLQView, 0, len(orders))
	for _, o := range orders {
		view = append(view, DLQView{
			OrderID:    o.OrderID,
			RetryCount: o.RetryCount,
			AgeSeconds: (now - o.CreatedAt) / 1000,
		})
	}

	return c.JSON(fiber.Map{
		"total":  len(view),
		"orders": view,
	})
}

// GetDispatchStatus retorna metricas do motor de dispatch.
// GET /dispatch/status
func (h *DispatchHandler) GetDispatchStatus(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"active_couriers":  len(h.CourierStore.FindNearby(-90, -180, 20000, 0)),
		"unmatched_orders": h.Matching.DLQ.Len(),
		"engine_running":   true,
	})
}

// haversine calcula distancia em km entre dois pontos geograficos.
func haversine(lat1, lng1, lat2, lng2 float64) float64 {
	const R = 6371.0
	dLat := (lat2 - lat1) * math.Pi / 180
	dLng := (lng2 - lng1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLng/2)*math.Sin(dLng/2)
	return R * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}
