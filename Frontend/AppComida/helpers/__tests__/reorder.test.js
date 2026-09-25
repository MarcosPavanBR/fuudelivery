import { orderEstablishmentId, paidTotal, rebuildCart } from "../reorder";

describe("repetir pedido", () => {
  const menuAtual = [
    { ID: 1, Name: "Pizza", Price: 55, Additional: [{ ID: 10, Name: "Borda", Price: 9 }] },
    { ID: 2, Name: "Refri", Price: 12, Available: false },
  ];

  it("usa o produto atual, tira esgotado/removido e adicional que saiu", () => {
    const antigo = [
      { item: { ID: 1, Name: "Pizza", Price: 49.9 }, quantity: 2, additionals: [10, 11], note: "sem cebola" },
      { item: { ID: 2, Name: "Refri", Price: 12 }, quantity: 1, additionals: [] },
      { item: { ID: 3, Name: "Suco", Price: 8 }, quantity: 1, additionals: [] },
    ];
    const { cart, removed, priceChanged } = rebuildCart(antigo, menuAtual);
    expect(cart).toEqual([{ item: menuAtual[0], quantity: 2, additionals: [10], note: "sem cebola" }]);
    expect(removed).toEqual(["Refri", "Suco"]);
    expect(priceChanged).toBe(true);
  });

  it("sem mudança nenhuma não avisa preço", () => {
    const { priceChanged, removed } = rebuildCart(
      [{ item: { ID: 1, Price: 55 }, quantity: 1, additionals: [10] }],
      menuAtual
    );
    expect(priceChanged).toBe(false);
    expect(removed).toEqual([]);
  });

  it("id da loja e total pago", () => {
    expect(orderEstablishmentId({ establishmentId: 7 })).toBe(7);
    expect(orderEstablishmentId({ establishment: { Id: 9 } })).toBe(9);
    expect(orderEstablishmentId({})).toBeNull();
    expect(paidTotal({ order_total: 61.5, cart: [], deliveryValue: 7 })).toBe(61.5);
    expect(paidTotal({ cart: [{ item: { Price: 10 }, quantity: 2 }], deliveryValue: 5 })).toBe(25);
  });
});
