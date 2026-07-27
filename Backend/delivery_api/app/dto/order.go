package dto

type OrderDTO struct {
	OrderId       string           `json:"orderid" bson:"orderid"`
	Status        string           `json:"status" bson:"status"`
	Establishment EstablishmentDTO `json:"establishment" bson:"establishment"`
	User          UserDTO          `json:"user" bson:"user"`
	Products      []ProductDTO     `json:"products" bson:"products"`
	DeliveryMan   DeliveryManDTO   `json:"deliveryman,omitempty" bson:"deliveryman,omitempty"`
	Total         float64          `json:"total,omitempty" bson:"total,omitempty"`
	Payment       PaymentDTO       `json:"payment,omitempty" bson:"payment,omitempty"`
	CreatedAt     string           `json:"created_at,omitempty" bson:"created_at,omitempty"`
	LastModified  string           `json:"lastModified,omitempty" bson:"lastModified,omitempty"`

	// Campo para zone resolution
	ZoneID *uint `json:"zone_id,omitempty" bson:"zone_id,omitempty"`
}

type EstablishmentDTO struct {
	Id      int64   `json:"id" bson:"id"`
	Name    string  `json:"name" bson:"name"`
	Lat     float64 `json:"lat,omitempty" bson:"lat,omitempty"`
	Long    float64 `json:"long,omitempty" bson:"long,omitempty"`
	Address string  `json:"address,omitempty" bson:"address,omitempty"`
	Phone   string  `json:"phone,omitempty" bson:"phone,omitempty"`
	Image   string  `json:"image,omitempty" bson:"image,omitempty"`
}

type UserDTO struct {
	ID    int64  `json:"id" bson:"id"`
	Name  string `json:"name" bson:"name"`
	Phone string `json:"phone" bson:"phone"`
}

type ProductDTO struct {
	Id       int64   `json:"id" bson:"id"`
	Name     string  `json:"name" bson:"name"`
	Quantity int     `json:"quantity" bson:"quantity"`
	Price    float64 `json:"price" bson:"price"`
	Image    string  `json:"image,omitempty" bson:"image,omitempty"`
	Note     string  `json:"note,omitempty" bson:"note,omitempty"`
}

type DeliveryManDTO struct {
	Id     int64  `json:"id" bson:"id"`
	Name   string `json:"name" bson:"name"`
	Status string `json:"status" bson:"status"`
}

type PaymentDTO struct {
	Method string  `json:"method" bson:"method"`
	Change float64 `json:"change,omitempty" bson:"change,omitempty"`
}

// --- Dispatch specific DTOs ---

type DispatchRequest struct {
	OrderID string  `json:"order_id"`
	Lat     float64 `json:"lat"`
	Lng     float64 `json:"lng"`
	Force   bool    `json:"force"` // true = ignora score, pega o mais proximo
}

type DispatchResponse struct {
	OrderID     string  `json:"order_id"`
	Matched     bool    `json:"matched"`
	CourierID   int64   `json:"courier_id,omitempty"`
	CourierName string  `json:"courier_name,omitempty"`
	DistanceKm  float64 `json:"distance_km,omitempty"`
	Fallback    bool    `json:"fallback,omitempty"`
	ZoneName    string  `json:"zone_name,omitempty"`
	DLQSize     int     `json:"dlq_size,omitempty"`
	Error       string  `json:"error,omitempty"`
}

type LocationUpdateRequest struct {
	DeliverymanID int64   `json:"deliveryman_id"`
	Name          string  `json:"name,omitempty"`
	Lat           float64 `json:"lat"`
	Lng           float64 `json:"lng"`
	Status        string  `json:"status"` // available, busy
}

type NearByRequest struct {
	Lat      float64 `json:"lat"`
	Lng      float64 `json:"lng"`
	RadiusKm float64 `json:"radius_km"`
}

type MatchStatusResponse struct {
	ZoneCount       int `json:"zone_count"`
	ActiveCouriers  int `json:"active_couriers"`
	UnmatchedOrders int `json:"unmatched_orders"`
	TotalDispatched int `json:"total_dispatched"`
}
