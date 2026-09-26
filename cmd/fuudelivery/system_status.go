package main

import "strconv"

// Status do sistema para o painel admin (GET /admin/system-status): o que
// está ligado e o que falta configurar no servidor. Só diz "configurado ou
// não" — nunca devolve valor de variável. Substitui a aba "Integrações" do
// painel, que mostrava "Conectado" fixo no código.

type statusItem struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
	Fix    string `json:"fix,omitempty"`
}

// Webhook de cada gateway (a assinatura é conferida com este segredo).
var gatewayWebhookEnv = map[string]string{
	"pagarme":     "PAGARME_WEBHOOK_SECRET",
	"asaas":       "ASAAS_WEBHOOK_TOKEN",
	"abacatepay":  "ABACATE_PAY_WEBHOOK_SECRET",
	"mercadopago": "MERCADOPAGO_WEBHOOK_SECRET",
}

func buildSystemStatus(getenv func(string) string, gateways []string, storageOK, dbOK bool) []statusItem {
	items := []statusItem{{
		Key: "database", Label: "Banco de dados", OK: dbOK,
		Detail: map[bool]string{true: "Conectado", false: "Sem conexão"}[dbOK],
	}}

	st := statusItem{Key: "storage", Label: "Upload de imagens (fotos, logos, avatar)", OK: storageOK}
	if storageOK {
		bucket := getenv("SUPABASE_STORAGE_BUCKET")
		if bucket == "" {
			bucket = "fuudelivery" // mesmo padrão de pkg/storage
		}
		st.Detail = "Supabase Storage, bucket \"" + bucket + "\" (precisa existir e ser público para as fotos aparecerem)"
	} else {
		st.Detail = "Desligado: todo envio de foto falha"
		st.Fix = "Defina SUPABASE_URL e SUPABASE_SERVICE_ROLE_KEY no servidor (e SUPABASE_STORAGE_BUCKET, se o bucket não se chamar \"fuudelivery\"), crie o bucket como público no Supabase e reinicie."
	}
	items = append(items, st)

	pay := statusItem{Key: "payments", Label: "Pagamento online", OK: len(gateways) > 0}
	if pay.OK {
		pay.Detail = "Gateways ativos: " + joinNames(gateways)
	} else {
		pay.Detail = "Nenhum gateway: só dinheiro/maquininha na entrega"
		pay.Fix = "Configure a chave de pelo menos um gateway (PAGARME_API_KEY, ASAAS_API_KEY, ABACATE_PAY_API_KEY ou MERCADOPAGO_ACCESS_TOKEN)."
	}
	items = append(items, pay)

	for _, g := range gateways {
		env, ok := gatewayWebhookEnv[g]
		if !ok {
			continue
		}
		set := getenv(env) != ""
		it := statusItem{Key: "webhook_" + g, Label: "Confirmação automática de pagamento (" + g + ")", OK: set}
		if set {
			it.Detail = "Webhook com assinatura conferida"
		} else {
			it.Detail = "Sem segredo do webhook: pagamentos não se confirmam sozinhos"
			it.Fix = "Defina " + env + " com o segredo cadastrado no painel do " + g + "."
		}
		items = append(items, it)
	}

	price := getenv("SPONSOR_DAILY_PRICE")
	slots := getenv("SPONSOR_SLOTS_PER_DAY")
	sp := statusItem{Key: "sponsor", Label: "Destaque patrocinado", OK: true}
	switch {
	case price == "" && slots == "":
		sp.Detail = "Usando preço e vagas padrão"
	default:
		sp.Detail = "Preço/dia: " + orDefault(price) + " · vagas/dia: " + orDefault(slots)
		if _, err := strconv.ParseFloat(orZero(price), 64); err != nil {
			sp.OK = false
			sp.Fix = "SPONSOR_DAILY_PRICE precisa ser um número (ex.: 15.00)."
		}
		if _, err := strconv.Atoi(orZero(slots)); err != nil {
			sp.OK = false
			sp.Fix = "SPONSOR_SLOTS_PER_DAY precisa ser um número inteiro (ex.: 3)."
		}
	}
	items = append(items, sp)
	return items
}

func joinNames(ns []string) string {
	out := ""
	for i, n := range ns {
		if i > 0 {
			out += ", "
		}
		out += n
	}
	return out
}

func orDefault(v string) string {
	if v == "" {
		return "padrão"
	}
	return v
}

func orZero(v string) string {
	if v == "" {
		return "0"
	}
	return v
}
