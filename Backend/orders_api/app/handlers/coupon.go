package handlers

import (
	"math"
	"strings"
	"time"

	"github.com/carloshomar/fuudelivery/auth_api/app/middlewares"
	"github.com/carloshomar/fuudelivery/orders_api/app/dto"
	"github.com/carloshomar/fuudelivery/orders_api/app/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func CreateCoupon(c *fiber.Ctx) error {
	var request dto.CreateCouponRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Erro ao fazer parsing do corpo da requisição"})
	}

	// Cupom é dinheiro: quem cria decide quanto desconto sai do bolso de
	// alguém. A rota é `protectedRoute`, que só valida o JWT — QUALQUER
	// usuário logado passava por ela. Sem esta checagem, um cliente comum
	// criava um PERCENTAGE de 100 (a validação abaixo só recusa acima de 100)
	// para o establishment_id que quisesse e usava no próprio pedido.
	//
	// canActOnEstablishment: admin passa sempre; estabelecimento só no
	// próprio. É o mesmo helper que os handlers de pedido já usam.
	if !canActOnEstablishment(c, int64(request.EstablishmentID)) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Apenas o administrador ou o próprio estabelecimento pode criar cupom",
		})
	}

	request.Code = strings.ToUpper(strings.TrimSpace(request.Code))
	if request.Code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Código do cupom é obrigatório"})
	}

	if request.DiscountType != "PERCENTAGE" && request.DiscountType != "FIXED" && request.DiscountType != "FREE_DELIVERY" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Tipo de desconto inválido. Use PERCENTAGE, FIXED ou FREE_DELIVERY"})
	}

	if request.DiscountType == "PERCENTAGE" && (request.DiscountValue <= 0 || request.DiscountValue > 100) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Percentual de desconto deve estar entre 1 e 100"})
	}

	if request.DiscountType == "FIXED" && request.DiscountValue <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Valor de desconto deve ser maior que zero"})
	}

	// Limites negativos contornam os tetos: MaxUses < 0 pula a checagem
	// (`MaxUses > 0`) e vira "ilimitado" — o mesmo para a cota por usuário.
	// Número negativo aqui é typo ou tentativa; recusar.
	if request.MaxUses < 0 || request.MaxUsesPerUser < 0 || request.MinOrderValue < 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Limites negativos não são permitidos"})
	}

	// Cupom global (establishment_id = 0 — vale em TODOS os restaurantes) é
	// decisão da plataforma: só o admin cria. Um dono mandando 0 criava uma
	// promoção válida em lojas que nunca ouviram falar nele.
	isAdmin := false
	if role, rErr := middlewares.GetUserRoleFromToken(c); rErr == nil && role == "admin" {
		isAdmin = true
	}
	if !isAdmin && request.EstablishmentID == 0 {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Cupom válido em todos os restaurantes só pode ser criado pelo administrador. Informe o seu establishment_id",
		})
	}

	// Quem banca o desconto — é o que diz ao split de qual lado subtrair.
	//
	// O PADRÃO depende de quem cria, e não é detalhe: quem oferece a promoção
	// sem dizer nada é quem a está oferecendo. Admin cria promoção da
	// plataforma; estabelecimento cria a dele. Um padrão fixo obrigaria o
	// restaurante a preencher um campo só para não mandar a conta para outro.
	//
	// A REGRA é assimétrica de propósito: o admin escolhe os dois lados
	// (ele pode negociar que a loja banque), mas o estabelecimento só banca a
	// si mesmo — senão ele criaria a promoção dele marcada como "platform" e
	// empurraria o custo para a plataforma.
	// (isAdmin já foi resolvido acima, junto com a checagem de cupom global.)

	fundedBy := strings.ToLower(strings.TrimSpace(request.FundedBy))
	if fundedBy == "" {
		if isAdmin {
			fundedBy = models.CouponFundedByPlatform
		} else {
			fundedBy = models.CouponFundedByEstablishment
		}
	}
	if fundedBy != models.CouponFundedByPlatform && fundedBy != models.CouponFundedByEstablishment {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "funded_by inválido. Use platform ou establishment",
		})
	}
	if !isAdmin && fundedBy != models.CouponFundedByEstablishment {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Cupom criado pelo estabelecimento é bancado pelo próprio estabelecimento",
		})
	}

	var existingCoupon models.Coupon
	if err := models.DB.Where("code = ?", request.Code).First(&existingCoupon).Error; err == nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "Código de cupom já existe"})
	}

	startDate, err := time.Parse(time.RFC3339, request.StartDate)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Data de início inválida. Use o formato RFC3339"})
	}

	expiryDate, err := time.Parse(time.RFC3339, request.ExpiryDate)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Data de expiração inválida. Use o formato RFC3339"})
	}

	coupon := models.Coupon{
		Code:            request.Code,
		Description:     request.Description,
		DiscountType:    request.DiscountType,
		DiscountValue:   request.DiscountValue,
		MinOrderValue:   request.MinOrderValue,
		MaxUses:         request.MaxUses,
		MaxUsesPerUser:  request.MaxUsesPerUser,
		StartDate:       startDate,
		ExpiryDate:      expiryDate,
		EstablishmentID: request.EstablishmentID,
		FundedBy:        fundedBy,
	}

	if err := models.DB.Create(&coupon).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Erro ao criar cupom"})
	}

	return c.Status(fiber.StatusCreated).JSON(coupon)
}

func ValidateCoupon(c *fiber.Ctx) error {
	var request dto.ValidateCouponRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Erro ao fazer parsing do corpo da requisição"})
	}

	request.Code = strings.ToUpper(strings.TrimSpace(request.Code))

	// Quando há token, o telefone dele vence o do corpo: a rota existe para o
	// app consultar, mas o dono do cupom e a cota por usuário só podem ser
	// consultados por quem é — o corpo forjável não decide identidade.
	if tokenPhone, pErr := middlewares.GetUserPhoneFromToken(c); pErr == nil && tokenPhone != "" {
		request.UserPhone = tokenPhone
	}

	var coupon models.Coupon
	if err := models.DB.Where("code = ?", request.Code).First(&coupon).Error; err != nil {
		return c.JSON(dto.ValidateCouponResponse{
			Valid:   false,
			Message: "Cupom não encontrado",
		})
	}

	if !coupon.IsActive {
		return c.JSON(dto.ValidateCouponResponse{
			Valid:   false,
			Message: "Cupom está inativo",
		})
	}

	now := time.Now()
	if now.Before(coupon.StartDate) {
		return c.JSON(dto.ValidateCouponResponse{
			Valid:   false,
			Message: "Cupom ainda não está válido",
		})
	}

	if now.After(coupon.ExpiryDate) {
		return c.JSON(dto.ValidateCouponResponse{
			Valid:   false,
			Message: "Cupom expirado",
		})
	}

	if coupon.MaxUses > 0 && coupon.UsedCount >= coupon.MaxUses {
		return c.JSON(dto.ValidateCouponResponse{
			Valid:   false,
			Message: "Cupom atingiu o limite máximo de usos",
		})
	}

	if request.OrderValue < coupon.MinOrderValue {
		return c.JSON(dto.ValidateCouponResponse{
			Valid:   false,
			Message: "Valor mínimo do pedido não atingido",
		})
	}

	if request.EstablishmentID != 0 && coupon.EstablishmentID != 0 && coupon.EstablishmentID != request.EstablishmentID {
		return c.JSON(dto.ValidateCouponResponse{
			Valid:   false,
			Message: "Cupom não é válido para este estabelecimento",
		})
	}

	if coupon.OwnerPhone != "" && request.UserPhone != "" && coupon.OwnerPhone != request.UserPhone {
		return c.JSON(dto.ValidateCouponResponse{
			Valid:   false,
			Message: "Este cupom é pessoal e intransferível",
		})
	}

	if coupon.MaxUsesPerUser > 0 && request.UserPhone != "" {
		var userUsageCount int64
		models.DB.Model(&models.CouponUsage{}).Where("coupon_id = ? AND user_phone = ?", coupon.ID, request.UserPhone).Count(&userUsageCount)
		if int(userUsageCount) >= coupon.MaxUsesPerUser {
			return c.JSON(dto.ValidateCouponResponse{
				Valid:   false,
				Message: "Você já atingiu o limite de usos deste cupom",
			})
		}
	}

	var discountAmount float64
	switch coupon.DiscountType {
	case "PERCENTAGE":
		discountAmount = math.Round(request.OrderValue*(coupon.DiscountValue/100)*100) / 100
	case "FIXED":
		discountAmount = coupon.DiscountValue
	case "FREE_DELIVERY":
		discountAmount = 0
	}

	finalValue := request.OrderValue - discountAmount
	if finalValue < 0 {
		finalValue = 0
	}

	return c.JSON(dto.ValidateCouponResponse{
		Valid:          true,
		DiscountType:   coupon.DiscountType,
		DiscountValue:  coupon.DiscountValue,
		DiscountAmount: discountAmount,
		FinalValue:     finalValue,
	})
}

func ApplyCoupon(c *fiber.Ctx) error {
	var request dto.ApplyCouponRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Erro ao fazer parsing do corpo da requisição"})
	}

	// O telefone é identidade: vem do TOKEN, não do corpo. Lendo do corpo,
	// qualquer logado registrava uso em nome de terceiro — queimava a cota
	// MaxUsesPerUser do outro e poluía a auditoria de quem gastou o quê.
	tokenPhone, pErr := middlewares.GetUserPhoneFromToken(c)
	if pErr != nil || tokenPhone == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Usuário não identificado no token",
		})
	}
	request.UserPhone = tokenPhone
	request.Code = strings.ToUpper(strings.TrimSpace(request.Code))

	tx := models.DB.Begin()
	defer tx.Rollback()

	var coupon models.Coupon
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "NOWAIT"}).
		Where("code = ?", request.Code).
		First(&coupon).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Cupom não encontrado"})
	}

	now := time.Now()
	if !coupon.IsActive {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cupom está inativo"})
	}
	if now.Before(coupon.StartDate) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cupom ainda não está válido"})
	}
	if now.After(coupon.ExpiryDate) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cupom expirado"})
	}
	if coupon.MaxUses > 0 && coupon.UsedCount >= coupon.MaxUses {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "Cupom atingiu o limite máximo de usos"})
	}

	if request.EstablishmentID != 0 && coupon.EstablishmentID != 0 && coupon.EstablishmentID != request.EstablishmentID {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cupom não é válido para este estabelecimento"})
	}

	if coupon.OwnerPhone != "" && request.UserPhone != "" && coupon.OwnerPhone != request.UserPhone {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Este cupom é pessoal e intransferível"})
	}

	if coupon.MaxUsesPerUser > 0 && request.UserPhone != "" {
		var userUsageCount int64
		if err := tx.Model(&models.CouponUsage{}).Where("coupon_id = ? AND user_phone = ?", coupon.ID, request.UserPhone).Count(&userUsageCount).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Erro ao verificar uso do cupom"})
		}
		if int(userUsageCount) >= coupon.MaxUsesPerUser {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "Você já atingiu o limite de usos deste cupom"})
		}
	}

	if request.OrderValue < coupon.MinOrderValue {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Valor mínimo do pedido não atingido"})
	}

	// Compute discount from coupon data inside the transaction (no TOCTOU).
	var discountAmount float64
	switch coupon.DiscountType {
	case "PERCENTAGE":
		discountAmount = math.Round(request.OrderValue*(coupon.DiscountValue/100)*100) / 100
	case "FIXED":
		discountAmount = coupon.DiscountValue
	case "FREE_DELIVERY":
		discountAmount = 0
	}

	usage := models.CouponUsage{
		CouponID:       coupon.ID,
		UserPhone:      request.UserPhone,
		OrderID:        request.OrderID,
		DiscountAmount: discountAmount,
		UsedAt:         time.Now(),
	}

	if err := tx.Create(&usage).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Erro ao registrar uso do cupom"})
	}

	if err := tx.Model(&coupon).UpdateColumn("used_count", gorm.Expr("used_count + ?", 1)).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Erro ao atualizar contagem do cupom"})
	}

	if err := tx.Commit().Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Erro ao confirmar uso do cupom"})
	}

	return c.JSON(fiber.Map{
		"success":         true,
		"message":         "Cupom aplicado com sucesso",
		"discount_type":   coupon.DiscountType,
		"discount_value":  coupon.DiscountValue,
		"discount_amount": discountAmount,
	})
}

func ValidateCouponInternal(req dto.ValidateCouponRequest) dto.ValidateCouponResponse {
	var coupon models.Coupon
	if err := models.DB.Where("code = ?", req.Code).First(&coupon).Error; err != nil {
		return dto.ValidateCouponResponse{Valid: false, Message: "Cupom não encontrado"}
	}

	if !coupon.IsActive {
		return dto.ValidateCouponResponse{Valid: false, Message: "Cupom está inativo"}
	}

	now := time.Now()
	if now.Before(coupon.StartDate) {
		return dto.ValidateCouponResponse{Valid: false, Message: "Cupom ainda não está válido"}
	}

	if now.After(coupon.ExpiryDate) {
		return dto.ValidateCouponResponse{Valid: false, Message: "Cupom expirado"}
	}

	if coupon.MaxUses > 0 && coupon.UsedCount >= coupon.MaxUses {
		return dto.ValidateCouponResponse{Valid: false, Message: "Cupom atingiu o limite máximo de usos"}
	}

	if req.OrderValue < coupon.MinOrderValue {
		return dto.ValidateCouponResponse{Valid: false, Message: "Valor mínimo do pedido não atingido"}
	}

	if req.EstablishmentID != 0 && coupon.EstablishmentID != 0 && coupon.EstablishmentID != req.EstablishmentID {
		return dto.ValidateCouponResponse{Valid: false, Message: "Cupom não é válido para este estabelecimento"}
	}

	if coupon.OwnerPhone != "" && req.UserPhone != "" && coupon.OwnerPhone != req.UserPhone {
		return dto.ValidateCouponResponse{Valid: false, Message: "Este cupom é pessoal e intransferível"}
	}

	if coupon.MaxUsesPerUser > 0 && req.UserPhone != "" {
		var userUsageCount int64
		models.DB.Model(&models.CouponUsage{}).Where("coupon_id = ? AND user_phone = ?", coupon.ID, req.UserPhone).Count(&userUsageCount)
		if int(userUsageCount) >= coupon.MaxUsesPerUser {
			return dto.ValidateCouponResponse{Valid: false, Message: "Você já atingiu o limite de usos deste cupom"}
		}
	}

	var discountAmount float64
	switch coupon.DiscountType {
	case "PERCENTAGE":
		discountAmount = math.Round(req.OrderValue*(coupon.DiscountValue/100)*100) / 100
	case "FIXED":
		discountAmount = coupon.DiscountValue
	case "FREE_DELIVERY":
		discountAmount = 0
	}

	finalValue := req.OrderValue - discountAmount
	if finalValue < 0 {
		finalValue = 0
	}

	return dto.ValidateCouponResponse{
		Valid:          true,
		DiscountType:   coupon.DiscountType,
		DiscountValue:  coupon.DiscountValue,
		DiscountAmount: discountAmount,
		FinalValue:     finalValue,
	}
}

func ListCoupons(c *fiber.Ctx) error {
	role, roleErr := middlewares.GetUserRoleFromToken(c)
	if roleErr != nil || (role != "admin" && role != "establishment") {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Admin or establishment access required"})
	}

	// O escopo vem do TOKEN, não do query: um estabelecimento passava
	// establishment_id alheio e listava as promoções do concorrente (com
	// código, vigência e limites — material para competição direta). Admin
	// pode filtrar por qualquer um; o resto vê o próprio escopo.
	var coupons []models.Coupon
	query := models.DB
	if role == "admin" {
		if establishmentID := c.Query("establishment_id"); establishmentID != "" {
			query = query.Where("establishment_id = ? OR establishment_id = 0", establishmentID)
		}
	} else {
		tokenEstID, eErr := middlewares.GetEstablishmentIDFromToken(c)
		if eErr != nil {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Estabelecimento não identificado no token"})
		}
		// Os globais (establishment_id = 0) entram porque valem para ele.
		query = query.Where("establishment_id = ? OR establishment_id = 0", tokenEstID)
	}
	query.Order("created_at DESC").Find(&coupons)

	return c.JSON(coupons)
}

func GetCoupon(c *fiber.Ctx) error {
	id := c.Params("id")

	var coupon models.Coupon
	if err := models.DB.First(&coupon, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Cupom não encontrado"})
	}

	// Listar/ler cupom é ler promoção de alguém: cliente não navega cupons
	// alheios, e o dono só lê o dele (mais os globais da plataforma, que são
	// público do catálogo). O id na URL nunca foi autorização.
	role, roleErr := middlewares.GetUserRoleFromToken(c)
	if roleErr != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Acesso negado"})
	}
	if role != "admin" {
		if role == "establishment" {
			tokenEstID, eErr := middlewares.GetEstablishmentIDFromToken(c)
			if eErr != nil || (coupon.EstablishmentID != 0 && tokenEstID != int64(coupon.EstablishmentID)) {
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Acesso negado"})
			}
		} else {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Acesso negado"})
		}
	}

	return c.JSON(coupon)
}

func DeleteCoupon(c *fiber.Ctx) error {
	id := c.Params("id")

	var coupon models.Coupon
	if err := models.DB.First(&coupon, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Cupom não encontrado"})
	}

	// Sem esta checagem, qualquer usuário logado desativava QUALQUER cupom
	// só sabendo o id — inclusive a promoção de um restaurante concorrente.
	// A autorização é sobre o cupom carregado, não sobre o id da URL: é o
	// dono do cupom que importa.
	if !canActOnEstablishment(c, int64(coupon.EstablishmentID)) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Apenas o administrador ou o próprio estabelecimento pode desativar este cupom",
		})
	}

	coupon.IsActive = false
	if err := models.DB.Save(&coupon).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Erro ao desativar cupom"})
	}

	return c.JSON(fiber.Map{"message": "Cupom desativado com sucesso"})
}

func GenerateReferralCoupon(c *fiber.Ctx) error {
	var request dto.ReferralCouponRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Erro ao fazer parsing do corpo da requisição"})
	}

	// Você só indica por si mesmo. O telefone do indicador vinha do CORPO da
	// requisição, sem nenhuma checagem: dava para chamar em loop com números
	// arbitrários e cunhar cupons de R$10 à vontade (os códigos são
	// determinísticos — "GANHOU-<telefone>" — então também dava para gerar e
	// usar o cupom de boas-vindas de outra pessoa).
	//
	// Admin segue podendo gerar para qualquer um (campanha manual).
	if role, rErr := middlewares.GetUserRoleFromToken(c); rErr != nil || role != "admin" {
		tokenPhone, pErr := middlewares.GetUserPhoneFromToken(c)
		if pErr != nil || tokenPhone == "" || tokenPhone != request.ReferrerPhone {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Você só pode gerar cupom de indicação para o seu próprio telefone",
			})
		}
	}

	// Normaliza ANTES da checagem de dono: com espaço ou maiúsculas
	// estragadas no corpo, o código cunhado embutia o lixo (INDICOU-"
	// +5511 9999-9999") e o cupom nunca casava com o telefone de verdade.
	request.ReferrerPhone = strings.TrimSpace(request.ReferrerPhone)
	request.NewUserPhone = strings.TrimSpace(request.NewUserPhone)
	if request.ReferrerPhone == "" || request.NewUserPhone == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Telefone do indicador e do novo usuário são obrigatórios",
		})
	}

	now := time.Now()
	referrerExpiry := now.AddDate(0, 3, 0)
	newUserExpiry := now.AddDate(0, 3, 0)

	referrerCode := "INDICOU-" + strings.ToUpper(request.ReferrerPhone)
	newUserCode := "GANHOU-" + strings.ToUpper(request.NewUserPhone)

	referrerCoupon := models.Coupon{
		Code:           referrerCode,
		Description:    "Cupom de indicação - Você indicou um amigo!",
		DiscountType:   "PERCENTAGE",
		DiscountValue:  10,
		MinOrderValue:  0,
		MaxUses:        1,
		MaxUsesPerUser: 1,
		StartDate:      now,
		ExpiryDate:     referrerExpiry,
		IsActive:       true,
		// O cupom é DE quem indicou: só o telefone dele resgata. O código é
		// determinístico, então sem esta marca qualquer um que chutasse
		// "INDICOU-<tel>" de outra pessoa usava o cupom alheio.
		OwnerPhone: request.ReferrerPhone,
		// Quem banca: a indicação é programa da plataforma (o dono do app
		// escolhe recompensar crescimento); o restaurante não pactuou nada.
		FundedBy: models.CouponFundedByPlatform,
	}

	if err := models.DB.Create(&referrerCoupon).Error; err != nil {
		// Código determinístico + vigência de 3 meses: indicar de novo é caso
		// real, e o cliente merece saber disso — não um 500 com segredo de
		// banco vazando no log.
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") ||
			strings.Contains(strings.ToLower(err.Error()), "unique") {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": "Você já tem um cupom de indicação ativo",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Erro ao criar cupom do indicador"})
	}

	newUserCoupon := models.Coupon{
		Code:           newUserCode,
		Description:    "Cupom de indicação - Seja bem-vindo!",
		DiscountType:   "FIXED",
		DiscountValue:  10,
		MinOrderValue:  0,
		MaxUses:        1,
		MaxUsesPerUser: 1,
		StartDate:      now,
		ExpiryDate:     newUserExpiry,
		IsActive:       true,
		// Boas-vindas é pessoal do convidado: quem tem o telefone é quem usa.
		OwnerPhone: request.NewUserPhone,
		FundedBy:   models.CouponFundedByPlatform,
	}

	if err := models.DB.Create(&newUserCoupon).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Erro ao criar cupom do novo usuário"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message":              "Cupons de indicação criados com sucesso",
		"referrer_coupon_code": referrerCoupon.Code,
		"new_user_coupon_code": newUserCoupon.Code,
		"referrer_coupon":      referrerCoupon,
		"new_user_coupon":      newUserCoupon,
	})
}

func CalculateDiscount(c *fiber.Ctx) error {
	var request dto.ValidateCouponRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Erro ao fazer parsing do corpo da requisição"})
	}

	request.Code = strings.ToUpper(strings.TrimSpace(request.Code))

	var coupon models.Coupon
	if err := models.DB.Where("code = ?", request.Code).First(&coupon).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Cupom não encontrado"})
	}

	// Validate coupon before computing discount.
	if !coupon.IsActive {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cupom está inativo"})
	}
	now := time.Now()
	if now.Before(coupon.StartDate) || now.After(coupon.ExpiryDate) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cupom expirado ou ainda não válido"})
	}
	if coupon.MaxUses > 0 && coupon.UsedCount >= coupon.MaxUses {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cupom atingiu o limite máximo de usos"})
	}

	var discountAmount float64
	switch coupon.DiscountType {
	case "PERCENTAGE":
		discountAmount = math.Round(request.OrderValue*(coupon.DiscountValue/100)*100) / 100
	case "FIXED":
		discountAmount = coupon.DiscountValue
	case "FREE_DELIVERY":
		discountAmount = 0
	}

	finalValue := request.OrderValue - discountAmount
	if finalValue < 0 {
		finalValue = 0
	}

	return c.JSON(dto.ValidateCouponResponse{
		Valid:          true,
		DiscountType:   coupon.DiscountType,
		DiscountValue:  coupon.DiscountValue,
		DiscountAmount: discountAmount,
		FinalValue:     finalValue,
	})
}
