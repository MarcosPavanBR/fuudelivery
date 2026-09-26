//go:build integration

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	authModels "github.com/carloshomar/fuudelivery/auth_api/app/models"
	"github.com/carloshomar/fuudelivery/payment_api/app/models"
	"github.com/carloshomar/fuudelivery/pkg/gateway"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

// Destaque patrocinado por dia: dinheiro (débito/estorno uma vez só), vagas
// por dia e disputa pela última vaga — contra Postgres de verdade, porque a
// trava da praça é pg_advisory_xact_lock.

const sponsorSecret = "sponsor-test-secret"

func setupSponsor(t *testing.T) *fiber.App {
	t.Helper()
	cleanup := setupCheckoutE2EEnv(t)
	t.Cleanup(cleanup)
	applyLedgerIdempotencyIndexes(t)
	for _, tbl := range []string{"sponsor_bookings", "establishments"} {
		require.NoError(t, models.DB.Exec("DROP TABLE IF EXISTS "+tbl+" CASCADE").Error)
	}
	require.NoError(t, models.DB.AutoMigrate(&authModels.Establishment{}, &authModels.SponsorBooking{}))
	t.Setenv("JWT_SECRET", sponsorSecret)
	t.Setenv("SPONSOR_DAILY_PRICE", "15")
	t.Setenv("SPONSOR_SLOTS_PER_DAY", "3")

	app := fiber.New()
	app.Get("/sponsorship/offer", GetSponsorOffer)
	app.Post("/sponsorship/bookings", CreateSponsorBooking)
	app.Post("/sponsorship/bookings/:id/cancel", CancelSponsorBooking)
	app.Post("/sponsorship/bookings/:id/confirm", ConfirmSponsorPayment)
	return app
}

func seedStore(t *testing.T, id uint, balance float64) {
	t.Helper()
	require.NoError(t, models.DB.Create(&authModels.Establishment{ID: id, Name: fmt.Sprintf("Loja %d", id)}).Error)
	if balance > 0 {
		seedWallet(t, int64(id), "establishment", balance)
	}
}

func storeToken(t *testing.T, estID uint) string {
	t.Helper()
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id": float64(estID + 100), "role": "user", "account_type": "user",
		"establishment_id": float64(estID), "exp": time.Now().Add(time.Hour).Unix(),
	}).SignedString([]byte(sponsorSecret))
	require.NoError(t, err)
	return s
}

func adminToken(t *testing.T) string {
	t.Helper()
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id": float64(1), "role": "admin", "account_type": "user", "exp": time.Now().Add(time.Hour).Unix(),
	}).SignedString([]byte(sponsorSecret))
	require.NoError(t, err)
	return s
}

func call(t *testing.T, app *fiber.App, method, path, token, body string) (int, map[string]interface{}) {
	t.Helper()
	req := httptest.NewRequest(method, path, bytesReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	var out map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func dayFromToday(n int) string {
	d, _ := authModels.SponsorDayRange(authModels.SponsorToday(time.Now()), n+1)
	return d[n]
}

func book(t *testing.T, app *fiber.App, estID uint, start string, days int, pay string) (int, map[string]interface{}) {
	return call(t, app, http.MethodPost, "/sponsorship/bookings", storeToken(t, estID),
		fmt.Sprintf(`{"start_day":%q,"days":%d,"pay_with":%q}`, start, days, pay))
}

func bookingID(out map[string]interface{}) uint {
	b, _ := out["booking"].(map[string]interface{})
	id, _ := b["id"].(float64)
	return uint(id)
}

// Paga pela carteira: debita uma vez, ativa na hora; o cancelamento antes de
// começar estorna uma vez, mesmo chamado duas vezes.
func TestSponsor_CarteiraDebitaEEstornaUmaVez(t *testing.T) {
	app := setupSponsor(t)
	seedStore(t, 7, 100)

	code, out := book(t, app, 7, dayFromToday(2), 3, "wallet")
	require.Equal(t, 201, code, "%v", out)
	require.InDelta(t, 55.0, currentBalance(t, 7, "establishment"), 0.001, "3 dias × R$ 15 saem da carteira")
	id := bookingID(out)
	require.Equal(t, int64(1), countLedgerOf(t, 7, "establishment", "debit", fmt.Sprintf("sponsor:%d", id)))

	for i := 0; i < 2; i++ {
		code, out = call(t, app, http.MethodPost, fmt.Sprintf("/sponsorship/bookings/%d/cancel", id), storeToken(t, 7), "")
		require.Equal(t, 200, code, "%v", out)
	}
	require.InDelta(t, 100.0, currentBalance(t, 7, "establishment"), 0.001, "estorno integral, uma vez só")
	require.Equal(t, int64(1), countLedgerOf(t, 7, "establishment", "credit", fmt.Sprintf("sponsor-refund:%d", id)))
}

// Sem saldo: nada é criado e nada é debitado.
func TestSponsor_SemSaldoNaoCriaNada(t *testing.T) {
	app := setupSponsor(t)
	seedStore(t, 7, 20)

	code, out := book(t, app, 7, dayFromToday(1), 3, "wallet") // R$ 45 > R$ 20
	require.Equal(t, http.StatusPaymentRequired, code, "%v", out)
	require.InDelta(t, 20.0, currentBalance(t, 7, "establishment"), 0.001)
	var n int64
	models.DB.Model(&authModels.SponsorBooking{}).Count(&n)
	require.Zero(t, n, "reserva não pode sobrar sem pagamento")
}

// Vagas: 3 por dia. A 4ª loja recebe 409 com a próxima data livre; PIX
// pendente segura a vaga até expirar; outra loja não cancela reserva alheia.
func TestSponsor_VagasPorDiaEProximaData(t *testing.T) {
	app := setupSponsor(t)
	for id := uint(1); id <= 4; id++ {
		seedStore(t, id, 100)
	}
	start := dayFromToday(1)
	for id := uint(1); id <= 2; id++ {
		code, out := book(t, app, id, start, 2, "wallet")
		require.Equal(t, 201, code, "%v", out)
	}
	code, out := book(t, app, 3, start, 2, "pix") // pendente também ocupa
	require.Equal(t, 201, code, "%v", out)
	pixID := bookingID(out)

	code, out = book(t, app, 4, start, 2, "wallet")
	require.Equal(t, http.StatusConflict, code, "%v", out)
	require.Equal(t, dayFromToday(3), out["next_available"], "próxima data com 2 dias livres")
	require.InDelta(t, 100.0, currentBalance(t, 4, "establishment"), 0.001, "recusada não cobra")

	// Mesma loja, dias sobrepostos: recusa.
	code, _ = book(t, app, 1, dayFromToday(2), 1, "wallet")
	require.Equal(t, http.StatusConflict, code)

	// Loja 4 tentando cancelar a reserva da loja 3.
	code, _ = call(t, app, http.MethodPost, fmt.Sprintf("/sponsorship/bookings/%d/cancel", pixID), storeToken(t, 4), "")
	require.Equal(t, http.StatusForbidden, code)

	// O PIX expira sem pagamento: a vaga volta, e a loja 4 consegue.
	require.NoError(t, models.DB.Model(&authModels.SponsorBooking{}).Where("id = ?", pixID).
		Update("expires_at", time.Now().Add(-time.Minute)).Error)
	code, out = book(t, app, 4, start, 2, "wallet")
	require.Equal(t, 201, code, "%v", out)

	// Confirmar o PIX expirado agora falha: as vagas foram ocupadas.
	code, _ = call(t, app, http.MethodPost, fmt.Sprintf("/sponsorship/bookings/%d/confirm", pixID), adminToken(t), "")
	require.Equal(t, http.StatusConflict, code)
}

// Confirmação do PIX pelo admin ativa a reserva; confirmar de novo é no-op.
func TestSponsor_AdminConfirmaPix(t *testing.T) {
	app := setupSponsor(t)
	seedStore(t, 9, 0)
	code, out := book(t, app, 9, dayFromToday(0), 1, "pix")
	require.Equal(t, 201, code, "%v", out)
	id := bookingID(out)

	now, err := authModels.SponsorsNow(models.DB, time.Now())
	require.NoError(t, err)
	require.Empty(t, now, "pendente não aparece na vitrine")

	for i := 0; i < 2; i++ {
		code, out = call(t, app, http.MethodPost, fmt.Sprintf("/sponsorship/bookings/%d/confirm", id), adminToken(t), "")
		require.Equal(t, 200, code, "%v", out)
	}
	now, err = authModels.SponsorsNow(models.DB, time.Now())
	require.NoError(t, err)
	require.Equal(t, []uint{9}, now)
}

// Disputa pela vaga: 6 lojas ao mesmo tempo, 3 vagas — exatamente 3 levam,
// e só as 3 são cobradas.
func TestSponsor_DisputaConcorrenteRespeitaVagas(t *testing.T) {
	app := setupSponsor(t)
	for id := uint(1); id <= 6; id++ {
		seedStore(t, id, 100)
	}
	start := dayFromToday(5)
	var wg sync.WaitGroup
	codes := make([]int, 7)
	for id := uint(1); id <= 6; id++ {
		wg.Add(1)
		go func(id uint) {
			defer wg.Done()
			codes[id], _ = book(t, app, id, start, 1, "wallet")
		}(id)
	}
	wg.Wait()
	ok, charged := 0, 0
	for id := 1; id <= 6; id++ {
		if codes[id] == 201 {
			ok++
		}
		if currentBalance(t, int64(id), "establishment") < 100 {
			charged++
		}
	}
	require.Equal(t, 3, ok, "codes=%v", codes)
	require.Equal(t, 3, charged, "só quem levou a vaga paga")
}

// ── PIX automático ──

func stubSponsorPix(t *testing.T, paid map[string]int64) *int {
	t.Helper()
	calls := 0
	prevCreate, prevCheck := createSponsorPix, checkSponsorPix
	createSponsorPix = func(_ context.Context, req *gateway.TransactionRequest) (*gateway.TransactionResponse, error) {
		calls++
		return &gateway.TransactionResponse{
			GatewayID: "pix_" + req.Metadata["sponsor_booking"], PIXCopyPaste: "00020126BRCODE", PIXQRCodeBase64: "iVBOR",
		}, nil
	}
	checkSponsorPix = func(id string) (bool, int64, error) {
		c, ok := paid[id]
		return ok, c, nil
	}
	t.Cleanup(func() { createSponsorPix, checkSponsorPix = prevCreate, prevCheck })
	return &calls
}

func reload(t *testing.T, id uint) authModels.SponsorBooking {
	t.Helper()
	var b authModels.SponsorBooking
	require.NoError(t, models.DB.First(&b, id).Error)
	return b
}

// Reserva por PIX: sai com copia-e-cola; o pagamento confirmado liga o
// destaque (uma vez), valor divergente não liga, e nada vai para payments
// nem para a carteira da loja.
func TestSponsorPix_PagamentoLigaODestaque(t *testing.T) {
	app := setupSponsor(t)
	stubSponsorPix(t, nil)
	seedStore(t, 7, 0)

	code, out := book(t, app, 7, dayFromToday(1), 2, "pix") // R$ 30
	require.Equal(t, 201, code, "%v", out)
	id := bookingID(out)
	b := reload(t, id)
	require.Equal(t, fmt.Sprintf("pix_%d", id), b.GatewayChargeID)
	require.Equal(t, "00020126BRCODE", out["booking"].(map[string]interface{})["pix_copy_paste"])

	require.True(t, settleSponsorCharge(b.GatewayChargeID, 2999, time.Now()), "cobrança de destaque é tratada aqui")
	require.Equal(t, authModels.SponsorBookingPending, reload(t, id).Status, "valor divergente não liga")

	for i := 0; i < 2; i++ {
		require.True(t, settleSponsorCharge(b.GatewayChargeID, 3000, time.Now()))
	}
	require.Equal(t, authModels.SponsorBookingActive, reload(t, id).Status)

	var payments int64
	models.DB.Model(&models.Payment{}).Count(&payments)
	require.Zero(t, payments, "PIX do destaque não pode virar pagamento de pedido")
	var wallets int64
	models.DB.Model(&models.WalletTxn{}).Count(&wallets)
	require.Zero(t, wallets, "a loja não recebe crédito pelo próprio destaque")

	require.False(t, settleSponsorCharge("cobranca_de_pedido", 3000, time.Now()), "cobrança que não é de destaque segue o fluxo de pedidos")
}

// Webhook perdido: a varredura consulta o gateway e liga.
func TestSponsorPix_VarreduraCobreWebhookPerdido(t *testing.T) {
	app := setupSponsor(t)
	paid := map[string]int64{}
	stubSponsorPix(t, paid)
	seedStore(t, 7, 0)
	_, out := book(t, app, 7, dayFromToday(1), 1, "pix")
	id := bookingID(out)
	paid[fmt.Sprintf("pix_%d", id)] = 1500
	// Dentro da janela de graça não mexe (o webhook pode estar rodando).
	ReconcileSponsorPixOnce(time.Now())
	require.Equal(t, authModels.SponsorBookingPending, reload(t, id).Status)
	ReconcileSponsorPixOnce(time.Now().Add(5 * time.Minute))
	require.Equal(t, authModels.SponsorBookingActive, reload(t, id).Status)
}

// Pagou depois de a vaga expirar e ser vendida: não liga e o valor volta à
// carteira da loja uma vez só. Gateway fora: reserva segue, sem PIX.
func TestSponsorPix_PagoSemVagaEGatewayFora(t *testing.T) {
	app := setupSponsor(t)
	stubSponsorPix(t, nil)
	for id := uint(1); id <= 4; id++ {
		seedStore(t, id, 100)
	}
	start := dayFromToday(2)
	_, out := book(t, app, 4, start, 1, "pix")
	pixID := bookingID(out)
	require.NoError(t, models.DB.Model(&authModels.SponsorBooking{}).Where("id = ?", pixID).
		Update("expires_at", time.Now().Add(-time.Minute)).Error)
	for id := uint(1); id <= 3; id++ {
		code, o := book(t, app, id, start, 1, "wallet")
		require.Equal(t, 201, code, "%v", o)
	}
	b := reload(t, pixID)
	before := currentBalance(t, 4, "establishment")
	for i := 0; i < 2; i++ { // webhook repetido
		require.True(t, settleSponsorCharge(b.GatewayChargeID, 1500, time.Now()))
	}
	b = reload(t, pixID)
	require.Equal(t, authModels.SponsorBookingCancelled, b.Status, "sem vaga: não liga")
	require.NotNil(t, b.PaidAt)
	require.InDelta(t, before+15, currentBalance(t, 4, "establishment"), 0.001, "o valor volta à carteira, uma vez")

	createSponsorPix = func(context.Context, *gateway.TransactionRequest) (*gateway.TransactionResponse, error) {
		return nil, fmt.Errorf("gateway fora")
	}
	code, out := book(t, app, 1, dayFromToday(3), 1, "pix")
	require.Equal(t, 201, code, "%v", out)
	require.Contains(t, out["message"], "fora do ar")
	require.Empty(t, reload(t, bookingID(out)).GatewayChargeID)
}

// Destaque pago por PIX e cancelado pela loja antes de começar: o valor
// volta à carteira (antes era estorno manual); PIX pago de reserva que a
// loja já cancelou também volta, uma vez.
func TestSponsorPix_CancelamentoDevolveNaCarteira(t *testing.T) {
	app := setupSponsor(t)
	stubSponsorPix(t, nil)
	seedStore(t, 7, 0)

	_, out := book(t, app, 7, dayFromToday(3), 2, "pix") // R$ 30
	id := bookingID(out)
	require.True(t, settleSponsorCharge(reload(t, id).GatewayChargeID, 3000, time.Now()))
	require.Equal(t, authModels.SponsorBookingActive, reload(t, id).Status)
	code, o := call(t, app, http.MethodPost, fmt.Sprintf("/sponsorship/bookings/%d/cancel", id), storeToken(t, 7), "")
	require.Equal(t, 200, code, "%v", o)
	require.InDelta(t, 30.0, currentBalance(t, 7, "establishment"), 0.001)

	// Cancela ainda pendente e depois paga o PIX.
	_, out = book(t, app, 7, dayFromToday(5), 1, "pix") // R$ 15
	id2 := bookingID(out)
	code, _ = call(t, app, http.MethodPost, fmt.Sprintf("/sponsorship/bookings/%d/cancel", id2), storeToken(t, 7), "")
	require.Equal(t, 200, code)
	for i := 0; i < 2; i++ {
		require.True(t, settleSponsorCharge(reload(t, id2).GatewayChargeID, 1500, time.Now()))
	}
	require.Equal(t, authModels.SponsorBookingCancelled, reload(t, id2).Status)
	require.InDelta(t, 45.0, currentBalance(t, 7, "establishment"), 0.001, "30 + 15, cada um uma vez")

	// Webhook perdido de PIX pago após cancelar: a varredura devolve.
	_, out = book(t, app, 7, dayFromToday(7), 1, "pix")
	id3 := bookingID(out)
	call(t, app, http.MethodPost, fmt.Sprintf("/sponsorship/bookings/%d/cancel", id3), storeToken(t, 7), "")
	checkSponsorPix = func(string) (bool, int64, error) { return true, 1500, nil }
	ReconcileSponsorPixOnce(time.Now().Add(5 * time.Minute))
	ReconcileSponsorPixOnce(time.Now().Add(6 * time.Minute))
	require.InDelta(t, 60.0, currentBalance(t, 7, "establishment"), 0.001, "varredura devolve uma vez")
}
