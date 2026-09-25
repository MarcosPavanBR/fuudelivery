// Leitura do pedido para o card do Kanban (Task.js). Funções puras, testadas
// em __tests__/orderCard.test.js.

// O payload grava "paymentMethod" (camelCase); o card lia "paymentmethod" e
// mostrava sempre "—".
export function paymentType(data) {
  return data?.paymentMethod?.type ?? data?.paymentmethod?.type ?? "";
}

// Adicionais ESCOLHIDOS no item (ids em cartItem.additionals). O card
// listava item.additional — todos os adicionais que o produto oferece — e a
// cozinha via adicional que ninguém pediu.
export function selectedAdditionals(cartItem) {
  const ids = cartItem?.additionals || [];
  const available = cartItem?.item?.additional || [];
  return ids
    .map((id) => available.find((a) => a.ID === id || a.id === id))
    .filter(Boolean);
}

function itemTotal(cartItem) {
  const extras = selectedAdditionals(cartItem).reduce((s, a) => s + (a.price || 0), 0);
  return (cartItem?.quantity || 0) * ((cartItem?.item?.price || 0) + extras);
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
