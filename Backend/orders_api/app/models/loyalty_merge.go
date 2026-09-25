package models

import (
	"fmt"

	"gorm.io/gorm"
)

// MergeDuplicateLoyaltyAccounts funde as contas de pontos duplicadas por
// telefone na mais antiga (menor id) e garante o índice único
// uq_loyalty_points_phone. Roda na subida (ConnectPostgresDatabase), depois do
// AutoMigrate — é o mesmo trabalho de sql/29, sem depender de alguém lembrar
// de rodar o run_all.sh em produção.
//
// As duplicatas vinham do FOR UPDATE NOWAIT antigo: com dois pedidos do mesmo
// cliente aprovados juntos, o segundo criava outra conta. O código novo
// (lockLoyaltyAccount) não cria mais; isto limpa o que já existe.
//
// Idempotente e barato quando não há o que fundir (uma contagem). Quando há,
// a fusão roda numa transação com a tabela travada para escrita: durante um
// deploy a versão antiga ainda atende e poderia creditar pontos numa linha
// entre o UPDATE e o DELETE — o lock impede essa perda. Devolve quantos
// telefones foram fundidos.
func MergeDuplicateLoyaltyAccounts(db *gorm.DB) (int64, error) {
	var phones int64
	if err := db.Raw(`SELECT COUNT(*) FROM (
		SELECT user_phone FROM loyalty_points GROUP BY user_phone HAVING COUNT(*) > 1
	) d`).Scan(&phones).Error; err != nil {
		return 0, fmt.Errorf("contar duplicatas de loyalty_points: %w", err)
	}

	if phones > 0 {
		err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec(`LOCK TABLE loyalty_points IN SHARE ROW EXCLUSIVE MODE`).Error; err != nil {
				return err
			}
			// Faixas de nível iguais às de getTier (handlers/loyalty.go):
			// >= 1500 ouro, >= 500 prata, senão bronze.
			if err := tx.Exec(`
				WITH somas AS (
					SELECT user_phone, MIN(id) AS keep_id, SUM(points) AS points,
					       SUM(total_orders) AS total_orders, SUM(total_spent) AS total_spent
					FROM loyalty_points GROUP BY user_phone HAVING COUNT(*) > 1
				)
				UPDATE loyalty_points lp
				   SET points = s.points, total_orders = s.total_orders, total_spent = s.total_spent,
				       tier = CASE WHEN s.points >= 1500 THEN 'ouro'
				                   WHEN s.points >= 500  THEN 'prata'
				                   ELSE 'bronze' END,
				       updated_at = now()
				  FROM somas s
				 WHERE lp.id = s.keep_id`).Error; err != nil {
				return err
			}
			return tx.Exec(`
				DELETE FROM loyalty_points lp
				 USING (SELECT user_phone, MIN(id) AS keep_id FROM loyalty_points
				        GROUP BY user_phone HAVING COUNT(*) > 1) d
				 WHERE lp.user_phone = d.user_phone AND lp.id <> d.keep_id`).Error
		})
		if err != nil {
			return 0, fmt.Errorf("fundir contas de pontos duplicadas: %w", err)
		}
	}

	if err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS uq_loyalty_points_phone
		ON loyalty_points (user_phone)`).Error; err != nil {
		return phones, fmt.Errorf("criar uq_loyalty_points_phone: %w", err)
	}
	return phones, nil
}
