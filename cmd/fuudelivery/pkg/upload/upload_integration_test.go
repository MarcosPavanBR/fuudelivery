//go:build integration

package upload

import (
	"os"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// requirePostgres devolve a URI do Postgres de teste. Em CI a ausência é
// erro de configuração do workflow e FALHA (senão o job ficaria verde sem ter
// rodado nada); fora do CI, pula.
func requirePostgres(t *testing.T) string {
	t.Helper()
	uri := os.Getenv("POSTGRES_TEST_URI")
	if uri == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("POSTGRES_TEST_URI ausente em CI — o job precisa provê-la, senão este teste não roda")
		}
		t.Skip("POSTGRES_TEST_URI não definida (rode com um Postgres local)")
	}
	return uri
}

func TestEntityBelongsTo_Postgres(t *testing.T) {
	uri := requirePostgres(t)
	db, err := gorm.Open(postgres.Open(uri), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	for _, tb := range []string{"products", "categories", "additionals"} {
		db.Exec("DROP TABLE IF EXISTS " + tb + " CASCADE")
		if err := db.Exec("CREATE TABLE " + tb + " (id BIGINT PRIMARY KEY, establishment_id BIGINT)").Error; err != nil {
			t.Fatal(err)
		}
		db.Exec("INSERT INTO " + tb + " VALUES (10, 42)")
	}
	for _, tb := range []string{"products", "categories", "additionals"} {
		if !entityBelongsTo(db, tb, "10", 42) {
			t.Errorf("%s 10 é da loja 42", tb)
		}
		if entityBelongsTo(db, tb, "10", 41) {
			t.Errorf("%s 10 não é da loja 41", tb)
		}
	}
	if entityBelongsTo(db, "users", "10", 42) {
		t.Error("entidade fora da lista não pode passar")
	}
}
