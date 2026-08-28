// Package pagarme implementa o adapter do FuuDelivery para o gateway Pagar.me v4.
//
// O Pagar.me é o gateway principal do FuuDelivery, suportando:
//   - PIX (instantâneo)
//   - Cartão de crédito (com 3DS e pré-autorização)
//   - Cartão de débito (com 3DS)
//   - Split de pagamento nativo (percentual + fixo)
//   - Sub-contas (recipients) com bank account
//   - Webhook com HMAC-SHA256
//
// Documentação: https://docs.pagar.me/v4/
package pagarme

// ═══════════════════════════════════════════════════════════════
// REQUEST TYPES — Criação de Transação
// ═══════════════════════════════════════════════════════════════

// CreateTransactionRequest é o payload enviado ao POST /1/orders
// para criar uma transação no Pagar.me.
type CreateTransactionRequest struct {
	// Amount é o valor em centavos (ex: R$ 50,00 = 5000).
	Amount int64 `json:"amount"`

	// PaymentMethod determina o método: "pix", "credit_card" ou "debit_card".
	PaymentMethod string `json:"payment_method"`

	// Capture controla se a transação é capturada imediatamente.
	// false = apenas autoriza (pré-autorização). true = autoriza + captura.
	// Default: true para PIX e débito, false para crédito (quando configurado).
	Capture *bool `json:"capture,omitempty"`

	// ExternalReference é o ID do pedido no FuuDelivery (metadata de rastreio).
	ExternalReference string `json:"external_reference,omitempty"`

	// Items contém os itens da transação (obrigatório no Pagar.me).
	Items []OrderItem `json:"items"`

	// Customer dados do cliente (obrigatório para cartão).
	Customer *CustomerRequest `json:"customer,omitempty"`

	// Shipping dados de entrega (opcional).
	Shipping *ShippingRequest `json:"shipping,omitempty"`

	// SplitRules define como o valor é dividido entre recebedores.
	SplitRules []SplitRuleRequest `json:"split_rules,omitempty"`

	// Metadata pares chave-valor salvos no gateway.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// OrderItem representa um item na transação (obrigatório no Pagar.me).
type OrderItem struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	UnitPrice   int64  `json:"unit_price"` // Em centavos
	Quantity    int    `json:"quantity"`
	Tangible    bool   `json:"tangible"` // true = produto físico
	Description string `json:"description,omitempty"`
}

// CustomerRequest dados do cliente para o Pagar.me.
type CustomerRequest struct {
	Name         string            `json:"name"`
	Email        string            `json:"email"`
	Type         string            `json:"type"` // "individual" ou "corporation"
	Documents    []DocumentRequest `json:"documents"`
	PhoneNumbers []PhoneNumber     `json:"phone_numbers,omitempty"`
	Addresses    []AddressRequest  `json:"addresses,omitempty"`
}

// DocumentRequest documento do cliente (CPF/CNPJ).
type DocumentRequest struct {
	Type   string `json:"type"`   // "cpf" ou "cnpj"
	Number string `json:"number"` // Somente dígitos
}

// PhoneNumber telefone do cliente.
type PhoneNumber struct {
	AreaCode string `json:"area_code"` // DDD (ex: "11")
	Number   string `json:"number"`    // Número (ex: "999999999")
}

// AddressRequest endereço do cliente.
type AddressRequest struct {
	Street       string `json:"street"`
	Number       string `json:"number"`
	Complement   string `json:"complement,omitempty"`
	Neighborhood string `json:"neighborhood"`
	City         string `json:"city"`
	State        string `json:"state"`
	ZipCode      string `json:"zip_code"`
	Country      string `json:"country"` // "BR"
}

// ShippingRequest dados de entrega.
type ShippingRequest struct {
	Name    string          `json:"name"`
	Fee     int64           `json:"fee"` // Taxa de entrega em centavos
	Address *AddressRequest `json:"address,omitempty"`
}

// SplitRuleRequest define uma regra de split para o Pagar.me.
type SplitRuleRequest struct {
	RecipientID         string  `json:"recipient_id"`          // ID do recipient no Pagar.me
	Percentage          float64 `json:"percentage,omitempty"`  // Percentual (0-100)
	Amount              int64   `json:"amount,omitempty"`      // Valor fixo em centavos
	Liable              bool    `json:"liable"`                // Responsável pelo MDR
	ChargeProcessingFee bool    `json:"charge_processing_fee"` // Responsável por taxas
}

// ═══════════════════════════════════════════════════════════════
// RESPONSE TYPES — Criação de Transação
// ═══════════════════════════════════════════════════════════════

// CreateTransactionResponse é a resposta do POST /1/orders.
type CreateTransactionResponse struct {
	ID                int64             `json:"id"`
	Status            string            `json:"status"` // "pending", "paid", "refused", "authorized"
	PaymentMethod     string            `json:"payment_method"`
	Amount            int64             `json:"amount"`
	ExternalReference string            `json:"external_reference"`
	CreatedAt         string            `json:"created_at"`
	UpdatedAt         string            `json:"updated_at"`
	Metadata          map[string]string `json:"metadata,omitempty"`

	// PIX
	QRCode     string `json:"qr_code,omitempty"`     // QR Code base64
	QRCodeURL  string `json:"qr_code_url,omitempty"` // URL do QR Code
	PixPayload string `json:"pix_payload,omitempty"` // Código copia-e-cola

	// Cartão
	CardBrand    string `json:"card_brand,omitempty"`     // "visa", "mastercard", etc.
	CardLastFour string `json:"card_last_four,omitempty"` // Últimos 4 dígitos
	Installments int    `json:"installments,omitempty"`

	// 3DS
	Authenticate    bool   `json:"authenticate"`               // Se 3DS é necessário
	AuthenticateURL string `json:"authenticate_url,omitempty"` // URL de autenticação

	// Split
	SplitRules []SplitRuleResponse `json:"split_rules,omitempty"`
}

// SplitRuleResponse resultado de uma regra de split.
type SplitRuleResponse struct {
	ID          int64   `json:"id"`
	RecipientID string  `json:"recipient_id"`
	Percentage  float64 `json:"percentage"`
	Amount      int64   `json:"amount"`
	Liable      bool    `json:"liable"`
	Status      string  `json:"status"` // "pending", "paid", "refused"
}

// ═══════════════════════════════════════════════════════════════
// REQUEST TYPES — Captura
// ═══════════════════════════════════════════════════════════════

// CaptureRequest é o payload para capturar uma pré-autorização.
type CaptureRequest struct {
	// Amount é o valor a capturar em centavos.
	// Se omitido ou 0, captura o valor total autorizado.
	Amount int64 `json:"amount,omitempty"`

	// SplitRules podem ser atualizados na captura (opcional).
	SplitRules []SplitRuleRequest `json:"split_rules,omitempty"`
}

// ═══════════════════════════════════════════════════════════════
// REQUEST TYPES — Estorno
// ═══════════════════════════════════════════════════════════════

// RefundRequest é o payload para estornar uma transação.
type RefundRequest struct {
	// Amount é o valor a estornar em centavos.
	// 0 ou omitido = estorno total.
	Amount int64 `json:"amount,omitempty"`

	// Metadata adicionais para o estorno.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// RefundResponse é a resposta de um estorno.
type RefundResponse struct {
	ID        int64  `json:"id"`
	Status    string `json:"status"` // "pending", "refunded"
	Amount    int64  `json:"amount"`
	CreatedAt string `json:"created_at"`
}

// ═══════════════════════════════════════════════════════════════
// REQUEST/RESPONSE TYPES — Recipients
// ═══════════════════════════════════════════════════════════════

// CreateRecipientRequest é o payload para criar um recipient.
type CreateRecipientRequest struct {
	// RegisterInformation dados de registro (KYC).
	RegisterInformation *RegisterInformationRequest `json:"register_information,omitempty"`

	// BankAccount dados bancários.
	BankAccount *BankAccountRequest `json:"bank_account,omitempty"`

	// AutomaticAnticipationEnabled habilita antecipação automática.
	AutomaticAnticipationEnabled bool `json:"automatic_anticipation_enabled"`

	// AnticipatableVolumePercentage percentual de antecipação.
	AnticipatableVolumePercentage int `json:"anticipatable_volume_percentage"`

	// TransferEnabled habilita transferências automáticas.
	TransferEnabled bool `json:"transfer_enabled"`

	// TransferInterval intervalo de transferência: "daily", "weekly", "monthly".
	TransferInterval string `json:"transfer_interval"`

	// TransferDay dia da transferência (1-28 para monthly, 1-7 para weekly).
	TransferDay int `json:"transfer_day,omitempty"`

	// PostbackURL URL para receber notificações de status.
	PostbackURL string `json:"postback_url,omitempty"`
}

// RegisterInformationRequest dados de registro do recebedor.
type RegisterInformationRequest struct {
	Type             string                   `json:"type"`            // "individual" ou "corporation"
	DocumentNumber   string                   `json:"document_number"` // CPF/CNPJ
	CompanyName      string                   `json:"company_name,omitempty"`
	Email            string                   `json:"email"`
	SiteURL          string                   `json:"site_url,omitempty"`
	AnnualRevenue    string                   `json:"annual_revenue,omitempty"`
	Address          *AddressRequest          `json:"address,omitempty"`
	PhoneNumbers     []PhoneNumber            `json:"phone_numbers,omitempty"`
	ManagingPartners []ManagingPartnerRequest `json:"managing_partners,omitempty"`
}

// ManagingPartnerRequest dados de sócios/parceiros.
type ManagingPartnerRequest struct {
	Name                            string          `json:"name"`
	DocumentNumber                  string          `json:"document_number"`
	MotherName                      string          `json:"mother_name,omitempty"`
	Birthdate                       string          `json:"birthdate,omitempty"`
	Email                           string          `json:"email"`
	MonthlyIncome                   string          `json:"monthly_income,omitempty"`
	ProfessionalOccupation          string          `json:"professional_occupation,omitempty"`
	SelfDeclaredLegalRepresentative bool            `json:"self_declared_legal_representative"`
	Address                         *AddressRequest `json:"address,omitempty"`
	PhoneNumbers                    []PhoneNumber   `json:"phone_numbers,omitempty"`
}

// BankAccountRequest dados bancários do recebedor.
type BankAccountRequest struct {
	BankCode       string `json:"bank_code"`            // Código BACEN (ex: "341")
	Agencia        string `json:"agencia"`              // Agência
	AgenciaDV      string `json:"agencia_dv,omitempty"` // DV da agência
	Conta          string `json:"conta"`                // Conta
	ContaDV        string `json:"conta_dv,omitempty"`   // DV da conta
	Type           string `json:"type"`                 // "conta_corrente", "conta_poupanca"
	DocumentNumber string `json:"document_number"`      // CPF/CNPJ do titular
	LegalName      string `json:"legal_name"`           // Nome do titular
}

// CreateRecipientResponse é a resposta da criação de um recipient.
type CreateRecipientResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// ═══════════════════════════════════════════════════════════════
// WEBHOOK TYPES
// ═══════════════════════════════════════════════════════════════

// WebhookPayload é o payload bruto do webhook do Pagar.me.
type WebhookPayload struct {
	ID                int64             `json:"id"`
	Object            string            `json:"object"` // "transaction"
	Status            string            `json:"status"` // "paid", "refused", "refunded", etc.
	PaymentMethod     string            `json:"payment_method"`
	Amount            int64             `json:"amount"`
	ExternalReference string            `json:"external_reference"`
	CreatedAt         string            `json:"created_at"`
	UpdatedAt         string            `json:"updated_at"`
	Metadata          map[string]string `json:"metadata,omitempty"`

	// PIX
	PixPayload string `json:"pix_payload,omitempty"`

	// Cartão
	CardBrand    string `json:"card_brand,omitempty"`
	CardLastFour string `json:"card_last_four,omitempty"`

	// Split
	SplitRules []SplitRuleResponse `json:"split_rules,omitempty"`
}

// BalanceResponse é a resposta de consulta de saldo.
type BalanceResponse struct {
	Available    int64 `json:"available"`     // Saldo disponível em centavos
	WaitingFunds int64 `json:"waiting_funds"` // Saldo pendente em centavos
}
