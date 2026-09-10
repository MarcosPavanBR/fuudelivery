package dto

import "time"

type Additional struct {
	ID          int     `json:"ID"`
	Name        string  `json:"Name"`
	Price       float64 `json:"Price"`
	Image       string  `json:"Image"`
	Description string  `json:"Description"`
}

type Item struct {
	ID              int          `json:"ID"`
	Name            string       `json:"Name"`
	Description     string       `json:"Description"`
	Price           float64      `json:"Price"`
	Image           string       `json:"Image"`
	EstablishmentID int          `json:"EstablishmentID"`
	Categories      interface{}  `json:"Categories"`
	Additional      []Additional `json:"Additional"`
}

type CartItem struct {
	Item        Item   `json:"item"`
	Additionals []int  `json:"additionals"`
	Quantity    int    `json:"quantity"`
	ID          string `json:"id"`
}

type Coords struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type Location struct {
	Cep         string `json:"cep"`
	Logradouro  string `json:"logradouro"`
	Complemento string `json:"complemento"`
	Bairro      string `json:"bairro"`
	Localidade  string `json:"localidade"`
	UF          string `json:"uf"`
	IBGE        string `json:"ibge"`
	GIA         string `json:"gia"`
	DDD         string `json:"ddd"`
	Siafi       string `json:"siafi"`
	Numero      string `json:"numero"`
	Coords      Coords `json:"coords"`
}

type User struct {
	Phone string `json:"phone"`
	Nome  string `json:"nome"`
}

type PaymentMethod struct {
	Type string `json:"type"`
	Icon string `json:"icon"`
}

type Establishment struct {
	HorarioFuncionamento string
	Id                   int64
	Image                string
	Latitude             float64 `json:"lat"`
	Longitude            float64 `json:"long"`
	MaxDistanceDelivery  float64 `json:"max_distance_delivery"`
	Name                 string  `json:"name"`
	OwnerId              int64   `json:"owner_id"`
	PrimaryCollor        string  `json:"primary_color"`
	SecondaryCollor      string  `json:"secondary_color"`
	LocationString       string  `json:"location_string"`
}

type DeliveryMan struct {
	Email  string `json:"email"`
	Id     int64  `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type RequestPayload struct {
	Cart []CartItem `json:"cart"`
	// Distance é IGNORADO no cálculo do frete desde que o preço passou a sair
	// da região do endereço (faixa de CEP). Continua no DTO só para app antigo
	// não quebrar no parse.
	//
	// Era o furo: `(distance × perKm) + fixa` com a distância vindo daqui, ou
	// seja, o cliente escolhia o próprio frete mandando "distance": 0. Quando
	// o servidor precisa de distância (fallback por km, e o raio de entrega),
	// ele a calcula com haversine a partir de Location.Coords e das
	// coordenadas do estabelecimento no banco.
	Distance      float64       `json:"distance"`
	Location      Location      `json:"location"`
	Status        string        `json:"status"`
	PaymentMethod PaymentMethod `json:"paymentMethod"`
	DeliveryValue float64       `json:"deliveryValue"`
	// OrderTotal é SEMPRE recalculado no servidor (computeOrderTotal) a partir
	// dos preços do banco; o valor do carrinho enviado pelo cliente é ignorado.
	// Gravado no payload JSONB e usado pela cobrança PIX/card como fonte única.
	OrderTotal float64 `json:"order_total"`

	// CouponCode é o ÚNICO campo de cupom que o cliente preenche. O servidor o
	// reescreve com o código efetivamente consumido (ou com "" se o cupom não
	// descontou nada), para que o payload nunca guarde um cupom que não foi
	// aplicado.
	CouponCode string `json:"coupon_code,omitempty"`
	// DiscountAmount e DiscountFundedBy são escritos SÓ pelo servidor
	// (CreateOrder -> applyCouponToOrder). Valor mandado pelo cliente é
	// sobrescrito. É daqui que payment_api lê para subtrair o desconto do lado
	// certo do split — sem FundedBy o desconto sairia diluído entre plataforma
	// e restaurante, e não de quem ofereceu a promoção.
	DiscountAmount   float64 `json:"discount_amount,omitempty"`
	DiscountFundedBy string  `json:"discount_funded_by,omitempty"`

	User            User          `json:"user"`
	EstablishmentId int64         `json:"establishmentId"`
	Establishment   Establishment `json:"establishment"`
	OrderId         string        `json:"order_id" `
	DeliveryMan     DeliveryMan   `json:"deliveryman"`
	ScheduledAt     *time.Time    `json:"scheduled_at,omitempty"`
	IsScheduled     bool          `json:"is_scheduled"`
}

type UpdateOrderStatusRequest struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}
