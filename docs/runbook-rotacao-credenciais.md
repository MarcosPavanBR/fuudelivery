# Runbook — Rotação de Credenciais Vazadas

> **Por quê:** credenciais de produção (Render API key, senha Atlas, chaves
> AbacatePay prod, backup com PII) ficaram versionadas no histórico do git até
> o commit `057db54`. Foram removidas da árvore, mas **continuam no histórico**
> — a única mitigação efetiva é ROTACIONAR tudo listado abaixo.
>
> Ordem pensada para nunca derrubar o serviço: cada passo troca a credencial e
> atualiza imediatamente o ambiente que a consome.

---

## 1. AbacatePay (pagamentos) ⚠️ prioridade máxima

1. Painel AbacatePay → API Keys → **revogar** `abc_prod_uCfX…` (a chave vazada).
2. Gerar nova chave de produção.
3. Webhooks → regenerar o **webhook secret** (o antigo `whsec_fuudelivery_prod_2024` está público).
4. Render → `fuudelivery-api` → Environment:
   - `ABACATE_PAY_API_KEY` = nova chave
   - `ABACATE_PAY_WEBHOOK_SECRET` = novo secret
5. Confirmar que a URL do webhook no painel aponta para
   `https://fuudelivery-api-8y6l.onrender.com/payments/webhook`.
6. Teste: gerar um PIX real de R$0,01 pelo app e conferir o webhook (logs do Render).

## 2. MongoDB Atlas (dual-write legado)

1. Atlas → Database Access → user `pavanbrtl050_db_user` → **Edit Password** (gerar forte).
2. Render Environment → `MONGO_URI` com a nova senha.
3. Observação: em ~22/09 o Atlas é aposentado (ver ARQUITETURA-BANCO-UNICO.md);
   mesmo assim rotacione agora — a senha está exposta.

## 3. Redis Cloud (fila financeira)

1. Redis Cloud → instância → **password reset**.
2. Render Environment → `REDIS_URL` com a nova senha.
3. Conferir política de eviction: para fila de pagamentos usar `noeviction`
   (ou aceitar explicitamente o risco documentado em render.yaml).

## 4. Supabase (banco primário)

1. Dashboard → Settings → API → **Rotate** `service_role` key.
2. Render Environment → `SUPABASE_SERVICE_ROLE_KEY`.
3. A connection string do banco (`DB_CONNECTION_STRING`) usa a senha do role
   `postgres`/`app_backend` — se ela também esteve em algum arquivo local,
   troque em Database → Settings → Reset database password, e atualize o Render.

## 5. Render API Key

1. Account Settings → API Keys → **revoke** `rnd_uWc5Uf…` (vazada).
2. Criar nova chave.
3. GitHub repo → Settings → Secrets and variables → Actions → atualizar `RENDER_API_KEY`.
4. (Opcional) passar a chave temporariamente para o agente aplicar as env vars dos passos 1–4 via API.

## 6. JWT_SECRET

1. `openssl rand -hex 32` (ou gerador equivalente).
2. Render Environment → `JWT_SECRET`.
3. Efeito: todas as sessões existentes são invalidadas; com refresh token já
   implantado nos clientes, os usuários são reautenticados transparentemente
   na maioria dos casos (login novo quando o refresh também for antigo).

## 7. Validação final

- [ ] `monitor.yml` verde / `/health` 200
- [ ] Login no app + painel admin funcionando
- [ ] PIX de teste confirmado via webhook
- [ ] Upload de imagem funcionando (Supabase Storage)

## 8. Purgar o histórico (APENAS depois de fechar todos os PRs abertos)

```bash
pip install git-filter-repo
git filter-repo --path .fuudelivery-config --path backups --invert-paths --force
git push origin master --force
# colaboradores/agentes devem re-clonar depois disso
```

## 9. Higiene local

- Apagar `.fuudelivery-config/CREDENTIALS.md` do disco após migrar os valores
  para um gerenciador de senhas.
- Nunca colar segredos em docs/.md/.html novamente (motivo deste runbook).
