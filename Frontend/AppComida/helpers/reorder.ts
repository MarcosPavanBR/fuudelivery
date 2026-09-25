// "Repetir pedido": remonta o carrinho de um pedido antigo com o cardápio
// ATUAL da loja. Testado em __tests__/reorder.test.js.
//
// O pedido guarda uma cópia dos produtos da época: repetir com ela mostrava
// preço velho (o servidor cobra o atual) e mandava item que nem existe mais
// ou está esgotado (o servidor recusava o pedido inteiro).

const pick = (o: any, ...keys: string[]) => {
  for (const k of keys) if (o && o[k] !== undefined && o[k] !== null) return o[k];
  return undefined;
};

export const productId = (p: any) => pick(p, "ID", "id", "Id");
export const productName = (p: any) => pick(p, "Name", "name") ?? "Item";
const productPrice = (p: any) => Number(pick(p, "Price", "price") ?? 0);

// Id da loja do pedido: o payload grava establishmentId e o objeto
// establishment (DTO em Go, chave "Id").
export function orderEstablishmentId(order: any): number | null {
  const id = Number(order?.establishmentId ?? order?.establishment?.Id ?? order?.establishment?.id ?? 0);
  return id > 0 ? id : null;
}

export function rebuildCart(oldCart: any[], currentProducts: any[]) {
  const byId = new Map((currentProducts || []).map((p) => [productId(p), p]));
  const cart: any[] = [];
  const removed: string[] = [];
  let priceChanged = false;

  for (const line of oldCart || []) {
    const old = line?.item || {};
    const now = byId.get(productId(old));
    if (!now || now.Available === false) {
      removed.push(productName(old));
      continue;
    }
    const offered = new Set((pick(now, "Additional", "additional") || []).map((a: any) => pick(a, "ID", "id")));
    const additionals = (line.additionals || []).filter((id: any) => offered.has(id));
    if (additionals.length !== (line.additionals || []).length) priceChanged = true;
    if (productPrice(now) !== productPrice(old)) priceChanged = true;
    cart.push({
      item: now,
      quantity: line.quantity || 1,
      additionals,
      ...(line.note ? { note: line.note } : {}),
    });
  }
  return { cart, removed, priceChanged };
}

// Total que o cliente pagou: o do servidor (com frete e cupom). A soma dos
// itens + frete fica só para pedido antigo sem order_total.
export function paidTotal(order: any): number {
  if (typeof order?.order_total === "number" && order.order_total > 0) return order.order_total;
  const items = (order?.cart || []).reduce(
    (s: number, it: any) => s + productPrice(it.item) * (it.quantity || 1),
    0
  );
  return items + Number(order?.deliveryValue || 0);
}
