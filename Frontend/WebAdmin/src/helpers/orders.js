// Pedido como o servidor devolve (orders_api docToResponseMap) → o que as
// telas do admin mostram. As telas liam id/total/createdAt/items e status em
// minúsculas, que não existem: tudo aparecia "R$ 0,00", sem data e com o
// status cru, e a lista não tinha key (aviso do React).

export const ORDER_STATUS = {
  AWAIT_APPROVE: { bg: "#FEF3C7", text: "#B45309", label: "Aguardando loja" },
  REQUEST_APPROVE: { bg: "#FEF3C7", text: "#B45309", label: "Aguardando loja" },
  SCHEDULED: { bg: "#EDE9FE", text: "#6D28D9", label: "Agendado" },
  APPROVED: { bg: "#DBEAFE", text: "#1D4ED8", label: "Aceito" },
  PREPARING: { bg: "#FFEDD5", text: "#C2410C", label: "Preparando" },
  DONE: { bg: "#DBEAFE", text: "#1D4ED8", label: "Pronto" },
  IN_ROUTE_DELIVERY: { bg: "#E0F2FE", text: "#0369A1", label: "Em rota" },
  FINISHED: { bg: "#ECFDF5", text: "#047857", label: "Entregue" },
  DENIED: { bg: "#FEE2E2", text: "#B91C1C", label: "Recusado" },
  CANCELLED: { bg: "#FEE2E2", text: "#B91C1C", label: "Cancelado" },
};

// Opções do filtro (sem os sinônimos).
export const STATUS_FILTER = ["AWAIT_APPROVE", "SCHEDULED", "APPROVED", "PREPARING", "DONE", "IN_ROUTE_DELIVERY", "FINISHED", "DENIED", "CANCELLED"];

export function statusInfo(status) {
  const s = String(status || "").toUpperCase();
  return ORDER_STATUS[s] || { bg: "#F3F4F6", text: "#4B5563", label: status || "—" };
}

export const PAYMENT_LABEL = { money: "Dinheiro", pix: "PIX", card: "Cartão", credit_card: "Cartão de crédito", debit_card: "Cartão de débito", wallet: "Carteira" };

export function normalizeOrder(o) {
  const status = String(o?.status || "").toUpperCase();
  const items = (Array.isArray(o?.cart) ? o.cart : []).map((c) => {
    const qty = Number(c?.quantity) || 1;
    const adds = Array.isArray(c?.item?.Additional) ? c.item.Additional : [];
    const unit = (Number(c?.item?.Price) || 0) + adds.reduce((s, a) => s + (Number(a?.Price ?? a?.price) || 0), 0);
    return {
      name: c?.item?.Name || "Item",
      quantity: qty,
      subtotal: unit * qty,
      note: c?.note || "",
      additionals: adds.map((a) => a?.Name || a?.name).filter(Boolean),
    };
  });
  const loc = o?.location || {};
  const address = [
    [loc.logradouro, loc.numero].filter(Boolean).join(", "),
    loc.complemento,
    loc.bairro,
    [loc.localidade, loc.uf].filter(Boolean).join(" - "),
  ].filter(Boolean).join(" · ");
  return {
    id: o?.order_id || o?._id || o?.id || "",
    status,
    customer: o?.user?.nome || "Cliente",
    phone: o?.user?.phone || "",
    establishment: o?.establishment?.name || "—",
    establishmentId: o?.establishmentId ?? o?.establishment?.Id,
    total: Number(o?.order_total ?? o?.total) || 0,
    deliveryValue: Number(o?.deliveryValue) || 0,
    deliveryman: o?.deliveryman?.name || "",
    payment: PAYMENT_LABEL[o?.paymentMethod?.type] || o?.paymentMethod?.type || "—",
    createdAt: o?.created_at || o?.createdAt || null,
    scheduledAt: o?.is_scheduled ? o?.scheduled_at : null,
    pickupCode: o?.pickup_code || "",
    address,
    items,
  };
}

export const money = (v) => (Number(v) || 0).toLocaleString("pt-BR", { style: "currency", currency: "BRL" });

export function isToday(date, now = new Date()) {
  if (!date) return false;
  const d = new Date(date);
  return !isNaN(d) && d.toDateString() === now.toDateString();
}
