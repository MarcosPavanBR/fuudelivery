//go:build integration

package handlers

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/carloshomar/fuudelivery/orders_api/app/dto"
	"github.com/carloshomar/fuudelivery/orders_api/app/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Produção conecta com default_query_exec_mode=simple_protocol (pgbouncer do
// Supabase; ver models.ConnectPostgresDatabase). Nesse modo o pgx mandava o
// payload []byte como bytea e o jsonb recusava: NENHUM pedido era gravado.
// Os outros testes abrem o banco sem esse modo e não viam o problema.
func TestOrderDocument_GravaNoModoDeProducao(t *testing.T) {
	uri := os.Getenv("POSTGRES_TEST_URI")
	if uri == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("POSTGRES_TEST_URI ausente em CI")
		}
		t.Skip("POSTGRES_TEST_URI não definida")
	}
	sep := "?"
	if strings.Contains(uri, "?") {
		sep = "&"
	}
	db, err := gorm.Open(postgres.Open(uri+sep+"default_query_exec_mode=simple_protocol"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	db.Exec("DROP TABLE IF EXISTS order_documents")
	if err := db.AutoMigrate(&models.OrderDocument{}); err != nil {
		t.Fatal(err)
	}
	prev := models.DB
	models.DB = db
	t.Cleanup(func() { models.DB = prev })

	p := dto.RequestPayload{Status: "AWAIT_APPROVE", EstablishmentId: 1}
	p.User.Phone = "+5511900000001"
	p.Establishment.HorarioFuncionamento = "Seg a Sáb, 08:00–22:00" // acento e travessão
	doc, err := payloadToDoc("pedido-simple-proto", &p)
	if err != nil {
		t.Fatal(err)
	}
	if err := saveOrderPrimary(doc); err != nil {
		t.Fatalf("gravar pedido no modo de produção: %v", err)
	}

	// Atualização (patch) também grava o payload.
	if err := patchOrderDoc(doc, func(d *models.OrderDocument, p *dto.RequestPayload) error {
		p.Status = "APPROVED"
		return nil
	}); err != nil {
		t.Fatalf("atualizar pedido no modo de produção: %v", err)
	}
	got, err := findOrderByLegacyID("pedido-simple-proto")
	if err != nil {
		t.Fatal(err)
	}
	var back dto.RequestPayload
	if err := json.Unmarshal(got.Payload, &back); err != nil {
		t.Fatal(err)
	}
	if got.Status != "APPROVED" || back.Establishment.HorarioFuncionamento != "Seg a Sáb, 08:00–22:00" {
		t.Fatalf("lido errado: status=%s horario=%q", got.Status, back.Establishment.HorarioFuncionamento)
	}
}
