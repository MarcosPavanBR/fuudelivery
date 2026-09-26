// Leitura do pedido para o card do Kanban (Task.js) e regras das ações.
// Funções puras, testadas em __tests__/orderCard.test.js.

// O pedido é gravado pelo servidor com as chaves do DTO em Go ("Name",
// "Price", "Additional", "ID"); pedidos antigos podem vir em minúsculas.
// O card lia só as minúsculas e o nome dos itens aparecia vazio.
const pick = (obj, ...keys) => {
  for (const k of keys) {
    if (obj && obj[k] !== undefined && obj[k] !== null) return obj[k];
  }
  return undefined;
};

// Pedidos gravados antes de o servidor preencher o item pelo cardápio podem
// ter Name "" — aparecia uma linha sem nome em "Mais vendidos".
export const itemName = (cartItem) =>
  String(pick(cartItem?.item, "Name", "name") ?? "").trim() || "Item sem nome";
// Observação do cliente no item ("sem cebola"). Só texto — o React escapa.
export const itemNote = (cartItem) => String(cartItem?.note || "").trim();
const itemPrice = (cartItem) => Number(pick(cartItem?.item, "Price", "price") ?? 0);
const additionalName = (a) => pick(a, "Name", "name") ?? "";

// O payload grava "paymentMethod" (camelCase); o card lia "paymentmethod" e
// mostrava sempre "—".
export function paymentType(data) {
  return data?.paymentMethod?.type ?? data?.paymentmethod?.type ?? "";
}

// Adicionais ESCOLHIDOS no item (ids em cartItem.additionals). O card
// listava todos os que o produto oferece, e a cozinha via adicional que
// ninguém pediu.
export function selectedAdditionals(cartItem) {
  const ids = cartItem?.additionals || [];
  const available = pick(cartItem?.item, "Additional", "additional") || [];
  return ids
    .map((id) => available.find((a) => pick(a, "ID", "id") === id))
    .filter(Boolean)
    .map((a) => ({ ...a, name: additionalName(a), price: Number(pick(a, "Price", "price") ?? 0) }));
}

function itemTotal(cartItem) {
  const extras = selectedAdditionals(cartItem).reduce((s, a) => s + a.price, 0);
  return (cartItem?.quantity || 0) * (itemPrice(cartItem) + extras);
}

// Valor do pedido: o total calculado pelo servidor (itens + frete − cupom),
// que é o que o cliente paga. A soma dos itens fica só para pedido antigo sem
// order_total.
export function orderTotal(data) {
  if (typeof data?.order_total === "number" && data.order_total > 0) {
    return data.order_total;
  }
  return (data?.cart || []).reduce((s, c) => s + itemTotal(c), 0);
}

// Número curto para a cozinha falar com o balcão/entregador ("#A1B2").
export function shortOrderId(id) {
  return id ? `#${String(id).slice(-4).toUpperCase()}` : "";
}

// Coluna em que o pedido aparece. Agendado e "aguardando aprovação" antigos
// não tinham coluna e sumiam do quadro: entram em "Em análise".
export function displayColumn(status) {
  if (status === "SCHEDULED" || status === "REQUEST_APPROVE") return "AWAIT_APPROVE";
  return status;
}

// Espelha validTransitions de Backend/orders_api/app/handlers/orders.go.
const TRANSITIONS = {
  AWAIT_APPROVE: ["APPROVED", "DENIED", "CANCELLED"],
  REQUEST_APPROVE: ["APPROVED", "DENIED", "CANCELLED"],
  SCHEDULED: ["APPROVED", "DENIED", "CANCELLED"],
  APPROVED: ["PREPARING", "CANCELLED"],
  PREPARING: ["DONE", "CANCELLED"],
  DONE: ["IN_ROUTE_DELIVERY", "FINISHED"],
  IN_ROUTE_DELIVERY: ["FINISHED"],
};

export function canMove(from, to) {
  return (TRANSITIONS[from] || []).includes(to);
}

const hasCourier = (data) => !!(data?.deliveryman && Number(data.deliveryman.id) > 0);

// Botões do card por status. "danger" pede confirmação antes de enviar.
// Com entregador atribuído, "saiu" e "entregue" são dele (o app do
// entregador avança o pedido); sem entregador, a loja entrega e marca.
export function actionsFor(data) {
  const status = data?.status;
  switch (status) {
    case "AWAIT_APPROVE":
    case "REQUEST_APPROVE":
    case "SCHEDULED":
      return [
        { label: "Aceitar", to: "APPROVED", kind: "primary" },
        { label: "Recusar", to: "DENIED", kind: "danger", confirm: "Recusar este pedido? O cliente será avisado." },
      ];
    case "APPROVED":
      return [
        { label: "Iniciar preparo", to: "PREPARING", kind: "primary" },
        { label: "Cancelar", to: "CANCELLED", kind: "danger", confirm: "Cancelar este pedido já aceito?" },
      ];
    case "PREPARING":
      return [
        { label: "Pronto", to: "DONE", kind: "primary" },
        { label: "Cancelar", to: "CANCELLED", kind: "danger", confirm: "Cancelar este pedido em preparo?" },
      ];
    case "DONE":
      return hasCourier(data) ? [] : [{ label: "Saiu p/ entrega", to: "IN_ROUTE_DELIVERY", kind: "primary" }];
    case "IN_ROUTE_DELIVERY":
      return hasCourier(data) ? [] : [{ label: "Entregue", to: "FINISHED", kind: "primary" }];
    default:
      return [];
  }
}

// "há 5 min" / "há 1 h 20 min" desde a criação do pedido.
export function elapsedLabel(createdAt, now = Date.now()) {
  const t = Date.parse(createdAt);
  if (Number.isNaN(t)) return "";
  const min = Math.max(0, Math.floor((now - t) / 60000));
  if (min < 1) return "agora";
  if (min < 60) return `há ${min} min`;
  const h = Math.floor(min / 60);
  const m = min % 60;
  return m ? `há ${h} h ${m} min` : `há ${h} h`;
}

// Pedido parado em análise há mais de 5 min fica em destaque: o cliente
// está esperando a loja aceitar.
export function isLate(data, now = Date.now()) {
  if (displayColumn(data?.status) !== "AWAIT_APPROVE" || data?.status === "SCHEDULED") return false;
  const t = Date.parse(data?.created_at);
  return !Number.isNaN(t) && now - t > 5 * 60000;
}

// Pedidos novos em análise que ainda não estavam na lista anterior — são os
// que disparam o alerta. Na primeira carga (previousIds null) não alerta.
export function newPendingIds(previousIds, orders) {
  if (!previousIds) return [];
  return orders
    .filter((o) => displayColumn(o.data?.status) === "AWAIT_APPROVE" && !previousIds.has(o.id))
    .map((o) => o.id);
}

// "+5511988887777" → "(11) 98888-7777" para a loja ler e discar. Formato
// desconhecido volta como veio.
export function formatPhone(raw) {
  const d = String(raw || "").replace(/\D/g, "").replace(/^55(?=\d{10,11}$)/, "");
  if (d.length === 11) return `(${d.slice(0, 2)}) ${d.slice(2, 7)}-${d.slice(7)}`;
  if (d.length === 10) return `(${d.slice(0, 2)}) ${d.slice(2, 6)}-${d.slice(6)}`;
  return String(raw || "");
}
