-- ============================================================================
-- sql/29_loyalty_points_unique.sql
-- FUUDELIVERY — Uma conta de pontos por telefone
-- ============================================================================
-- PARA QUE SERVE
--
-- O crédito de pontos fazia FOR UPDATE NOWAIT e tratava a falha do lock como
-- "conta não existe": com dois pedidos do mesmo cliente aprovados juntos, o
-- segundo criava OUTRA linha em loyalty_points para o mesmo telefone (até 8
-- contas em 20 créditos simultâneos, em teste). Os pontos ficavam divididos e
-- o saldo lido dependia de qual linha o SELECT devolvia.
--
-- O código já serializa por telefone (advisory lock em lockLoyaltyAccount).
-- Esta migração:
--   1. FUNDE as duplicatas existentes na linha mais antiga (menor id):
--      soma pontos, pedidos e gasto, e recalcula o nível pelas mesmas faixas
--      do código (getTier: >=1500 ouro, >=500 prata, senão bronze);
--   2. cria o índice único uq_loyalty_points_phone como defesa em
--      profundidade.
--
-- IDEMPOTENTE: sem duplicatas o passo 1 não mexe em nada; o índice é
-- IF NOT EXISTS. loyalty_points nasce no AutoMigrate — se ainda não existe,
-- pula (rode run_all.sh de novo após o primeiro boot, como o sql/17).
-- ============================================================================

DO $$
BEGIN
    IF to_regclass('public.loyalty_points') IS NULL THEN
        RAISE NOTICE 'loyalty_points ainda nao existe (AutoMigrate nao rodou) — pulando 29.';
        RETURN;
    END IF;

    -- 1. Funde duplicatas na menor id de cada telefone.
    WITH somas AS (
        SELECT user_phone,
               MIN(id)           AS keep_id,
               SUM(points)       AS points,
               SUM(total_orders) AS total_orders,
               SUM(total_spent)  AS total_spent
        FROM loyalty_points
        GROUP BY user_phone
        HAVING COUNT(*) > 1
    )
    UPDATE loyalty_points lp
       SET points       = s.points,
           total_orders = s.total_orders,
           total_spent  = s.total_spent,
           tier         = CASE WHEN s.points >= 1500 THEN 'ouro'
                               WHEN s.points >= 500  THEN 'prata'
                               ELSE 'bronze' END,
           updated_at   = now()
      FROM somas s
     WHERE lp.id = s.keep_id;

    DELETE FROM loyalty_points lp
     USING (SELECT user_phone, MIN(id) AS keep_id
              FROM loyalty_points GROUP BY user_phone HAVING COUNT(*) > 1) d
     WHERE lp.user_phone = d.user_phone
       AND lp.id <> d.keep_id;

    -- 2. Índice único.
    CREATE UNIQUE INDEX IF NOT EXISTS uq_loyalty_points_phone
        ON loyalty_points (user_phone);
END $$;

INSERT INTO schema_migrations (version, description)
VALUES ('29_loyalty_points_unique', 'Funde contas de pontos duplicadas por telefone e cria uq_loyalty_points_phone')
ON CONFLICT DO NOTHING;
