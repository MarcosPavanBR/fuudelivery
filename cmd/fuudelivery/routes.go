package main

// Registro das rotas HTTP por domínio (auth, pedidos, entrega, zonas,
// dispatch, pagamentos, patrocínio, assinaturas, chat).

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/gofiber/fiber/v2"

	// Models (database initialization)

	// Handlers
	authHandlers "github.com/carloshomar/fuudelivery/auth_api/app/handlers"
	chatHandlers "github.com/carloshomar/fuudelivery/chat_api/app/handlers"
	deliveryHandlers "github.com/carloshomar/fuudelivery/delivery_api/app/handlers"
	ordersHandlers "github.com/carloshomar/fuudelivery/orders_api/app/handlers"
	paymentHandlers "github.com/carloshomar/fuudelivery/payment_api/app/handlers"

	// Middleware
	"github.com/carloshomar/fuudelivery/auth_api/app/middlewares"

	// Dispatch engine

	// Batch expiry

	// Queue + Health + Upload + Metrics + Search
	"github.com/carloshomar/fuudelivery/pkg/gateway"
	"github.com/carloshomar/fuudelivery/pkg/gateway/abacatepay"
	"github.com/carloshomar/fuudelivery/pkg/gateway/asaas"
	"github.com/carloshomar/fuudelivery/pkg/gateway/mercadopago"
	"github.com/carloshomar/fuudelivery/pkg/gateway/pagarme"
)

func setupAuthRoutes(app *fiber.App) {
	app.Get("/csrf-token", authHandlers.GetCSRFToken)
	app.Post("/users/register", rateLimitMiddleware(5), authHandlers.CreateUser)
	app.Post("/users/login", rateLimitMiddleware(10), authHandlers.Login)
	app.Post("/auth/refresh", rateLimitMiddleware(30), authHandlers.RefreshToken)
	app.Post("/auth/logout", rateLimitMiddleware(10), authHandlers.Logout)
	app.Post("/auth/session", rateLimitMiddleware(10), authHandlers.SessionLogin)
	app.Get("/auth/session", protectedRoute, authHandlers.SessionMe)
	app.Post("/auth/session/refresh", rateLimitMiddleware(30), authHandlers.SessionRefresh)
	app.Post("/auth/session/logout", rateLimitMiddleware(10), authHandlers.SessionLogout)
	// Ticket de curta duração (60s) para WebSockets: o JWT fica SÓ no header
	// Authorization desta chamada e o WS conecta com ?ticket= — nada de JWT
	// na query string (vazava em logs de proxy). Ver resolveWSTicket.
	app.Post("/auth/ws-ticket", protectedRoute, rateLimitMiddleware(20), func(c *fiber.Ctx) error {
		auth := c.Get("Authorization")
		if len(auth) > 7 && auth[:7] == "Bearer " {
			auth = auth[7:]
		}
		ticket, tErr := IssueWSTicket(auth)
		if tErr != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
		}
		return c.JSON(fiber.Map{"ticket": ticket, "expires_in": 60})
	})

	// Reset de senha assistido: o suporte gera o código no WebAdmin e informa
	// por telefone/WhatsApp (não há serviço de email; clientes só têm phone).
	// O usuário usa o código na página pública /resetar-senha do WebRestaurant.
	//
	// Rate limit em DUAS camadas:
	//   1. rateLimitMiddleware(N) — por IP (protege contra flooding de uma fonte)
	//   2. rateLimitByIdentifier — por conta (protege contra brute-force
	//      distribuído: múltiplos IPs tentando a mesma conta)
	app.Post("/admin/password-reset/code", adminRequired, rateLimitMiddleware(10), rateLimitByIdentifierMiddleware(3), authHandlers.GenerateAdminResetCode)
	app.Post("/auth/reset-password", rateLimitMiddleware(5), rateLimitByIdentifierMiddleware(10), authHandlers.ResetPassword)
	app.Post("/users", adminRequired, authHandlers.CreateUserAdmin)
	app.Post("/admin/bootstrap", rateLimitMiddleware(3), authHandlers.BootstrapAdmin)
	app.Get("/users", adminRequired, authHandlers.ListAllUsers)
	app.Get("/users/:id", protectedRoute, authHandlers.GetUser)
	app.Put("/users/:id", protectedRoute, authHandlers.UpdateUser)
	app.Delete("/users/:id", protectedRoute, authHandlers.DeleteUser)
	app.Put("/users/:id/password", protectedRoute, authHandlers.ChangePassword)

	app.Get("/establishments", authHandlers.ListEstablishments)
	app.Get("/establishments/:id", authHandlers.GetEstablishments)
	app.Post("/establishments", adminRequired, authHandlers.CreateEstablishment)
	app.Put("/establishments/status/handler/:id", protectedRoute, authHandlers.HandlerEstablishmentStatus)
	app.Put("/establishments/:id", protectedRoute, authHandlers.UpdateEstablishment)
	app.Delete("/establishments/:id", adminRequired, authHandlers.DeleteEstablishment)
	app.Get("/establishments/:id/users", protectedRoute, authHandlers.GetUserByEstablishment)

	app.Get("/establishments/:id/hours", authHandlers.GetBusinessHours)
	app.Post("/establishments/hours", protectedRoute, authHandlers.UpsertBusinessHours)
	app.Post("/establishments/hours/bulk", protectedRoute, authHandlers.BulkUpdateBusinessHours)
	app.Get("/establishments/:id/is-open", authHandlers.CheckEstablishmentOpen)
	app.Put("/establishments/:id/wallet", protectedRoute, authHandlers.UpdateEstablishmentWallet)

	app.Post("/delivery-man/login", rateLimitMiddleware(10), authHandlers.LoginDeliveryMan)
	app.Post("/delivery-man/register", rateLimitMiddleware(5), authHandlers.CreateDeliveryMan)
	app.Get("/delivery-man", adminRequired, authHandlers.ListAllDeliveryMen)
	// Cadastro de entregador pelo admin (WebAdmin > Entregadores > Novo). A
	// tela chamava POST /delivery-man, que não existia; o público segue em
	// /delivery-man/register.
	app.Post("/delivery-man", adminRequired, authHandlers.CreateDeliveryMan)
	app.Put("/delivery-man/:id", adminRequired, authHandlers.UpdateDeliveryMan)
	app.Delete("/delivery-man/:id", adminRequired, authHandlers.DeleteDeliveryMan)
	app.Put("/delivery-man/:id/wallet", protectedRoute, authHandlers.UpdateDeliveryManWallet)

	// === Rotas de Cliente (AppComida) ===
	app.Post("/clients/register", rateLimitMiddleware(5), authHandlers.RegisterClient)
	app.Post("/clients/login", rateLimitMiddleware(10), authHandlers.LoginClient)

	// Cadastro público de restaurante (WebRestaurant)
	app.Post("/establishments/register", rateLimitMiddleware(3), authHandlers.RegisterEstablishment)
}

func setupOrdersRoutes(app *fiber.App) {
	app.Get("/ping", ordersHandlers.Ping)
	app.Get("/products/all/:establishmentId", ordersHandlers.GetByEstablishmentIdWithRelations)
	app.Get("/products/:establishmentId", ordersHandlers.GetByEstablishmentId)
	app.Post("/products/create", protectedRoute, ordersHandlers.CreateProduct)
	app.Delete("/products/delete/:id", protectedRoute, ordersHandlers.DeleteProduct)
	app.Post("/products/multi-create", protectedRoute, ordersHandlers.CreateMultProducts)
	app.Put("/products/update/:id", protectedRoute, ordersHandlers.UpdateProduct)
	// Pausar/reativar item esgotado (Cardápio da loja).
	app.Put("/products/:id/availability", protectedRoute, ordersHandlers.SetProductAvailability)
	app.Post("/categories/create", protectedRoute, ordersHandlers.CreateCategories)
	app.Get("/categories/:establishmentId", ordersHandlers.GetCategories)
	app.Post("/categories/product", protectedRoute, ordersHandlers.CreateProductCategorie)
	app.Delete("/categories/:id", protectedRoute, ordersHandlers.DeleteCategory)
	app.Put("/categories/:id", protectedRoute, ordersHandlers.UpdateCategory)
	app.Get("/categories/product/:establishmentId", ordersHandlers.GetCategoriesWithProducts)
	app.Post("/additional", protectedRoute, ordersHandlers.CreateAdditional)
	app.Get("/additional/:id", ordersHandlers.ListAdditional)
	app.Put("/additional/:id", protectedRoute, ordersHandlers.UpdateAdditional)
	app.Delete("/additional/:id", protectedRoute, ordersHandlers.DeleteAdditional)
	app.Post("/additional/product", protectedRoute, ordersHandlers.CreateProductToAdditional)
	app.Post("/delivery", protectedRoute, ordersHandlers.InsertDelivery)
	app.Post("/delivery/calculate-delivery-value", protectedRoute, ordersHandlers.CalculateDeliveryValue)
	app.Post("/delivery/calculate-route", protectedRoute, ordersHandlers.CalculateRoute)

	// Regiões de frete (faixa de CEP → preço). adminRequired em todas: quem
	// define o preço do frete é o dono da plataforma. Uma região é dinheiro em
	// todo pedido que casar com ela.
	app.Get("/delivery/regions", adminRequired, ordersHandlers.ListDeliveryRegions)
	app.Post("/delivery/regions", adminRequired, ordersHandlers.CreateDeliveryRegion)
	app.Put("/delivery/regions/:id", adminRequired, ordersHandlers.UpdateDeliveryRegion)
	app.Delete("/delivery/regions/:id", adminRequired, ordersHandlers.DeleteDeliveryRegion)
	app.Get("/delivery/value/:establishmentId", ordersHandlers.GetDeliveryByEstablishmentID)
	// Rate limit 30/min na criação de pedidos: é a rota que grava, notifica
	// e dispara dispatch — abuso direto impacta o banco e a fila.
	app.Post("/orders", protectedRoute, rateLimitMiddleware(30), func(c *fiber.Ctx) error {
		return ordersHandlers.CreateOrder(c, sendToEstablishment)
	})
	app.Put("/orders/status", protectedRoute, func(c *fiber.Ctx) error {
		return ordersHandlers.UpdateOrderStatus(c, sendToEstablishment)
	})
	app.Get("/orders/all", adminRequired, ordersHandlers.ListAllOrders)
	app.Get("/orders/repeat/:orderId", protectedRoute, ordersHandlers.RepeatOrder)
	app.Get("/orders/list-phone/:phone", protectedRoute, ordersHandlers.ListOrdersByPhone)
	app.Get("/orders/:establishmentId", protectedRoute, ordersHandlers.ListOrdersByEstablishmentID)
	app.Get("/orders/:establishmentId/:phoneNumber", protectedRoute, ordersHandlers.ListOrdersByEstablishmentIDAndPhone)
	app.Post("/coupons", protectedRoute, rateLimitMiddleware(20), ordersHandlers.CreateCoupon)
	app.Post("/coupons/validate", protectedRoute, rateLimitMiddleware(30), ordersHandlers.ValidateCoupon)
	app.Post("/coupons/apply", protectedRoute, rateLimitMiddleware(30), ordersHandlers.ApplyCoupon)
	app.Get("/coupons", protectedRoute, ordersHandlers.ListCoupons)
	app.Get("/coupons/:id", protectedRoute, ordersHandlers.GetCoupon)
	app.Delete("/coupons/:id", protectedRoute, ordersHandlers.DeleteCoupon)
	app.Post("/coupons/referral", protectedRoute, ordersHandlers.GenerateReferralCoupon)
	app.Post("/coupons/calculate", protectedRoute, ordersHandlers.CalculateDiscount)
	app.Get("/qrcode/:establishmentId", ordersHandlers.GenerateTableQRCode)
	app.Post("/orders/schedule", protectedRoute, ordersHandlers.ScheduleOrder)
	app.Post("/notifications/register", protectedRoute, rateLimitMiddleware(20), ordersHandlers.RegisterPushToken)
	app.Post("/loyalty/earn", protectedRoute, rateLimitMiddleware(20), ordersHandlers.EarnPoints)
	app.Post("/loyalty/redeem", protectedRoute, rateLimitMiddleware(20), ordersHandlers.RedeemPoints)
	app.Get("/loyalty/balance/:phone", protectedRoute, ordersHandlers.GetLoyaltyBalance)
	app.Get("/loyalty/history/:phone", protectedRoute, ordersHandlers.GetLoyaltyHistory)
	app.Get("/loyalty/calculate", protectedRoute, ordersHandlers.CalculateLoyaltyDiscount)
	app.Post("/reviews", protectedRoute, ordersHandlers.CreateReview)
	app.Get("/reviews/establishment/:id", protectedRoute, ordersHandlers.GetEstablishmentReviews)
	app.Get("/reviews/product/:id", protectedRoute, ordersHandlers.GetProductReviews)
	app.Put("/reviews/respond/:id", protectedRoute, ordersHandlers.RespondToReview)
	app.Get("/reviews/user/:phone", protectedRoute, ordersHandlers.GetUserReviews)
	app.Get("/reviews/rating/:establishmentId", protectedRoute, ordersHandlers.GetEstablishmentRating)
	app.Post("/orders/pickup-code/generate", protectedRoute, ordersHandlers.GeneratePickupCode)
	// Código de 6 dígitos: sem teto, o entregador do pedido testava o milhão
	// de combinações.
	app.Post("/orders/pickup-code/validate", protectedRoute, rateLimitMiddleware(10), ordersHandlers.ValidatePickupCode)
	app.Get("/orders/pickup-code/:id", protectedRoute, ordersHandlers.GetPickupCode)

	// === Rotas de Batch (batching de pedidos) ===
	batches := app.Group("/batches", adminRequired)
	batches.Post("/", ordersHandlers.CreateBatch)
	batches.Get("/:id", ordersHandlers.GetBatch)
	batches.Post("/:id/assign", ordersHandlers.AssignBatch)
	batches.Post("/:id/complete", ordersHandlers.CompleteBatch)
	batches.Post("/:id/add-order", ordersHandlers.AddOrderToBatch)
	batches.Get("/zone/:zoneId", ordersHandlers.ListBatchesByZone)
	batches.Post("/:id/force-expire", ordersHandlers.ForceExpireBatch)
}

func setupDeliveryRoutes(app *fiber.App) {
	app.Get("/solicitation-orders", protectedRoute, deliveryHandlers.GetApprovedSolicitations)
	app.Put("/solicitation-orders/hand-shake", protectedRoute, deliveryHandlers.HandShakeDeliveryman)
	app.Get("/deliveryman/has-active/:id", protectedRoute, deliveryHandlers.GetOrdersByDeliverymanID)
	app.Post("/deliveryman/status", protectedRoute, func(c *fiber.Ctx) error {
		return deliveryHandlers.UpdateOrderStatusByDeliverymanID(c, sendToEstablishment)
	})
	app.Get("/deliveryman/extrato/:id", protectedRoute, deliveryHandlers.GetExtrato)
}

func setupZoneRoutes(app *fiber.App) {
	// Sem :id no path de propósito — resolve sempre pelo establishment_id
	// do próprio token, nunca por um ID que o cliente possa manipular.
	app.Get("/establishments/me/zone", protectedRoute, authHandlers.GetMyZoneFee)
	app.Get("/zones", adminRequired, authHandlers.ListZones)
	app.Get("/zones/all", adminRequired, authHandlers.ListAllZones)
	app.Get("/zones/:id", adminRequired, authHandlers.GetZone)
	app.Post("/zones", adminRequired, authHandlers.CreateZone)
	app.Put("/zones/:id", adminRequired, authHandlers.UpdateZone)
	app.Delete("/zones/:id", adminRequired, authHandlers.DeleteZone)
	app.Post("/zones/:id/calibrate", adminRequired, authHandlers.CalibrateZone)
}

func setupDispatchRoutes(app *fiber.App) {
	dispatch := app.Group("/dispatch", protectedRoute)

	// Localizacao do entregador
	// 120/min: chega a cada poucos segundos por courier em movimento, mas
	// precisa de teto contra abuso.
	dispatch.Post("/location", rateLimitMiddleware(120), dispatchHandler.UpdateLocation)
	dispatch.Post("/status", rateLimitMiddleware(30), dispatchHandler.SetCourierStatus)

	// Matching — só admin. /nearby expõe nome e GPS ao vivo dos entregadores
	// (dado pessoal) e /trigger com force incrementa a carga do entregador a
	// cada chamada; com só protectedRoute, qualquer cliente logado fazia os
	// dois. Nenhum app chama estas rotas: o dispatch real roda no processo.
	dispatch.Post("/trigger", adminRequired, dispatchHandler.TriggerDispatch)
	dispatch.Get("/nearby", adminRequired, dispatchHandler.NearbyCouriers)

	// Dead-letter queue e metricas
	dispatch.Get("/dlq", adminRequired, dispatchHandler.GetDLQ)
	dispatch.Get("/status", adminRequired, dispatchHandler.GetDispatchStatus)
}

// paymentRouterMiddleware injeta o router de pagamento no contexto Fiber.
func paymentRouterMiddleware(router *gateway.Router) fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Locals("payment_router", router)
		return c.Next()
	}
}

// buildPaymentGateways monta a cadeia de fallback só com gateways realmente
// utilizáveis, na ordem de preferência: pagarme -> asaas -> abacatepay ->
// mercadopago.
//
// Duas regras:
//
//  1. Construtor que retorna erro é PULADO. abacatepay e mercadopago devolvem
//     (nil, err) sem credencial; registrar esse nil causaria panic no
//     CreateTransaction (ver comentário no chamador sobre typed nil).
//  2. pagarme e asaas nunca falham na construção — sobem mesmo com a chave
//     vazia. Sem a env de credencial eles só rendem um 401 por tentativa,
//     gastando um round-trip da cadeia de fallback à toa, então também
//     ficam de fora.
func buildPaymentGateways() []gateway.Gateway {
	var gws []gateway.Gateway

	add := func(name, credEnv string, build func() (gateway.Gateway, error)) {
		if credEnv != "" && os.Getenv(credEnv) == "" {
			log.Printf("[GATEWAY] %s fora da cadeia: %s não configurada", name, credEnv)
			return
		}
		gw, err := build()
		if err != nil {
			log.Printf("[GATEWAY] %s fora da cadeia: %v", name, err)
			return
		}
		gws = append(gws, gw)
		log.Printf("[GATEWAY] %s registrado na cadeia de fallback", name)
	}

	add("pagarme", "PAGARME_API_KEY", func() (gateway.Gateway, error) { return pagarme.NewGateway() })
	add("asaas", "ASAAS_API_KEY", func() (gateway.Gateway, error) { return asaas.NewGateway() })
	add("abacatepay", "ABACATE_PAY_API_KEY", func() (gateway.Gateway, error) { return abacatepay.NewGateway() })
	add("mercadopago", "MERCADOPAGO_ACCESS_TOKEN", func() (gateway.Gateway, error) { return mercadopago.NewGateway() })

	if len(gws) == 0 {
		log.Printf("[GATEWAY] ATENÇÃO: nenhum gateway de pagamento configurado — cobranças vão falhar")
	}
	return gws
}

func setupPaymentRoutes(app *fiber.App, router *gateway.Router) {
	paymentGroup := app.Group("/payments", paymentRouterMiddleware(router))
	walletGroup := app.Group("/wallets", paymentRouterMiddleware(router))
	// Admin — painel Financeiro do WebAdmin
	paymentGroup.Get("/all", adminRequired, paymentHandlers.ListAllPayments)
	paymentGroup.Get("/", adminRequired, paymentHandlers.ListAllPayments)
	paymentGroup.Get("/stats", adminRequired, paymentHandlers.GetPaymentStats)
	// Lista de carteiras do painel Financeiro do admin. O handler existia e o
	// WebAdmin chamava GET /wallets, mas a rota nunca foi registrada (404).
	walletGroup.Get("/", adminRequired, paymentHandlers.ListWallets)
	walletGroup.Get("/balance/:user_id", protectedRoute, paymentHandlers.GetBalance)
	walletGroup.Get("/establishment/balance", protectedRoute, paymentHandlers.GetEstablishmentWallet)
	walletGroup.Get("/establishment/transactions", protectedRoute, paymentHandlers.GetEstablishmentTransactions)
	walletGroup.Post("/topup", protectedRoute, rateLimitMiddleware(20), paymentHandlers.TopUp)
	walletGroup.Post("/deduct", protectedRoute, rateLimitMiddleware(20), paymentHandlers.DeductFromWallet)
	walletGroup.Post("/establishment/withdraw", protectedRoute, rateLimitMiddleware(20), paymentHandlers.EstablishmentWithdraw)
	paymentGroup.Get("/chargebacks", adminRequired, paymentHandlers.ListChargebacks)
	paymentGroup.Post("/:id/approve", adminRequired, rateLimitMiddleware(20), paymentHandlers.ApprovePayment)
	paymentGroup.Post("/:id/reject", adminRequired, rateLimitMiddleware(20), paymentHandlers.RejectPayment)
	// Rate limit 20/min nos endpoints de dinheiro (proteção contra abuso/custo)
	paymentGroup.Post("/pix/generate", protectedRoute, rateLimitMiddleware(20), paymentHandlers.GeneratePIX)
	// Tokenização de cartão REMOVIDA: o endpoint recebia PAN/CVV crus e
	// devolvia um "token" local sem valor no gateway (risco PCI puro). O
	// cartão volta quando houver tokenização server-side do AbacatePay.
	paymentGroup.Post("/card/charge", protectedRoute, rateLimitMiddleware(20), paymentHandlers.ChargeCard)
	paymentGroup.Post("/process", protectedRoute, rateLimitMiddleware(20), paymentHandlers.ProcessPayment)
	// Split rules definem como o dinheiro é dividido — só admin.
	paymentGroup.Post("/split", adminRequired, rateLimitMiddleware(20), paymentHandlers.ProcessSplit)
	paymentGroup.Post("/webhook", rateLimitMiddleware(100), paymentHandlers.HandlePaymentWebhook)
	// Status da cobrança por pedido (polling do app do cliente pós-PIX).
	paymentGroup.Get("/order/:order_id", protectedRoute, rateLimitMiddleware(30), paymentHandlers.GetPaymentByOrder)
	paymentGroup.Get("/reports/establishment/:id", protectedRoute, paymentHandlers.GetEstablishmentReport)
	// OAuth do Mercado Pago (split na origem): o connect exige o JWT do dono; o
	// callback é o redirect do MP e NÃO carrega JWT — a identidade vem do state
	// assinado (ver oauth_mp.go). Por isso o callback fica sem protectedRoute.
	paymentGroup.Get("/gateways/mercadopago/connect", protectedRoute, rateLimitMiddleware(20), paymentHandlers.ConnectMercadoPago)
	paymentGroup.Get("/gateways/mercadopago/callback", rateLimitMiddleware(20), paymentHandlers.MercadoPagoCallback)
	paymentGroup.Get("/gateways/mercadopago/status", protectedRoute, paymentHandlers.StatusMercadoPago)
	// Asaas legado — só admin. Os handlers aceitam do corpo valor, carteira de
	// destino e percentual do split (sem conferir com pedido nem dono) e criam
	// subcontas com qualquer CPF/CNPJ sob a conta da plataforma. Nenhum app
	// chama estas rotas; o pagamento do cliente passa pelo router.
	paymentGroup.Post("/asaas/wallet/create", adminRequired, rateLimitMiddleware(20), paymentHandlers.CreateAsaasWallet)
	paymentGroup.Get("/asaas/wallet/:walletId/status", adminRequired, paymentHandlers.GetAsaasWalletStatus)
	paymentGroup.Post("/asaas/payment/split", adminRequired, rateLimitMiddleware(20), paymentHandlers.CreateAsaasSplitPayment)
}

func setupSponsoredRoutes(app *fiber.App) {
	// Destaque patrocinado POR DIA (payment_api/app/handlers/sponsor.go).
	// Prefixo novo de propósito: o grupo antigo /sponsored (mensal, aposentado)
	// era app.Group(..., adminRequired), que prende TODA rota sob o prefixo —
	// as "públicas" dele também exigiam admin.
	app.Get("/sponsorship/offer", protectedRoute, paymentHandlers.GetSponsorOffer)
	app.Post("/sponsorship/bookings", protectedRoute, rateLimitMiddleware(20), paymentHandlers.CreateSponsorBooking)
	app.Post("/sponsorship/bookings/:id/cancel", protectedRoute, paymentHandlers.CancelSponsorBooking)
	app.Get("/sponsorship/bookings", adminRequired, paymentHandlers.ListSponsorBookings)
	app.Post("/sponsorship/bookings/:id/confirm", adminRequired, paymentHandlers.ConfirmSponsorPayment)
	// Quem aparece em destaque na vitrine sai de GET /establishments
	// (is_sponsored), já na ordem do rodízio.
}

func setupSubscriptionRoutes(app *fiber.App) {
	subscriptions := app.Group("/subscriptions")

	// Rotas do usuario (protegidas)
	subscriptions.Get("/me", protectedRoute, authHandlers.GetUserSubscription)
	subscriptions.Post("/", protectedRoute, authHandlers.CreateSubscription)
	subscriptions.Post("/cancel", protectedRoute, authHandlers.CancelSubscription)
	subscriptions.Post("/renew", protectedRoute, authHandlers.RenewSubscription)

	// Rotas de admin
	subscriptions.Get("/", adminRequired, authHandlers.ListSubscriptions)
	subscriptions.Put("/:id", adminRequired, authHandlers.AdminUpdateSubscription)
}

func setupChatRoutes(app *fiber.App) {
	// Mesma regra do WebSocket (wsCanAccessOrder). Antes esta rota lia a
	// tabela "orders" antiga com First(&order, orderID) — o :orderId da URL
	// ia cru para o GORM, que trata string não numérica como SQL literal
	// (injeção de SQL) — e comparava ids de tabelas diferentes.
	app.Get("/chat/messages/:orderId", protectedRoute, func(c *fiber.Ctx) error {
		claims, err := tokenClaims(c)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
		}
		orderID := c.Params("orderId")
		if orderID == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "orderId is required"})
		}
		if !wsCanAccessOrder(claims, orderID) {
			log.Printf("[CHAT IDOR] GetMessages denied: order=%s", orderID)
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "You are not a participant of this order"})
		}
		return chatHandlers.GetMessages(c)
	})
	// IDOR + anti-spoofing: antes qualquer usuário autenticado podia postar
	// como QUALQUER remetente em QUALQUER pedido (o handler confiava 100% no
	// corpo da requisição). Agora: só participantes do pedido e o remetente é
	// sempre quem o token diz que é.
	app.Post("/chat/message", protectedRoute, rateLimitMiddleware(30), func(c *fiber.Ctx) error {
		claims, tErr := tokenClaims(c)
		if tErr != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
		}
		var req struct {
			OrderID string `json:"order_id"`
		}
		if pErr := json.Unmarshal(c.Body(), &req); pErr != nil || req.OrderID == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "order_id is required"})
		}
		if !wsCanAccessOrder(claims, req.OrderID) {
			log.Printf("[CHAT IDOR] SendMessage denied: order=%s", req.OrderID)
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "You are not a participant of this order"})
		}
		tokenUserID, _ := middlewares.GetUserIDFromToken(c)
		var body map[string]interface{}
		if jErr := json.Unmarshal(c.Body(), &body); jErr != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid body"})
		}
		// Remetente vem SEMPRE do token — nunca do corpo. Nome resolvido do
		// banco (na tabela do tipo da conta) para não exibir o nome forjado.
		// O tipo também: antes só era trocado quando o token tinha role, e o
		// entregador (sem role) postava como "restaurant" ou "admin".
		body["sender_id"] = tokenUserID
		body["sender_name"] = senderNameForChat(claims)
		body["sender_type"] = chatUserTypeFromClaims(claims)
		fixedBody, _ := json.Marshal(body)
		c.Request().SetBody(fixedBody)
		return chatHandlers.SendMessage(c)
	})
	app.Put("/chat/read/:orderId/:userId", protectedRoute, func(c *fiber.Ctx) error {
		tokenUserID, err := middlewares.GetUserIDFromToken(c)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
		}
		urlUserIDStr := c.Params("userId")
		var urlUserID int64
		if _, scanErr := fmt.Sscanf(urlUserIDStr, "%d", &urlUserID); scanErr != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid userId"})
		}
		if tokenUserID != urlUserID {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Cannot mark messages as read for another user"})
		}
		// Autorização por recurso: só participantes do pedido podem marcar
		// mensagens como lidas — mesma regra do POST /chat/message (IDOR).
		claims, tErr := tokenClaims(c)
		if tErr != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
		}
		orderID := c.Params("orderId")
		if !wsCanAccessOrder(claims, orderID) {
			log.Printf("[CHAT IDOR] MarkAsRead denied: order=%s user=%d", orderID, tokenUserID)
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "You are not a participant of this order"})
		}
		// "As minhas" são pelo id E pelo tipo: o cliente 5 lendo não pula as
		// mensagens do entregador 5.
		return chatHandlers.MarkAsReadAs(c, chatUserTypeFromClaims(claims))
	})
}
