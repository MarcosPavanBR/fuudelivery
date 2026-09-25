import { describe, it, expect } from "vitest";
import { paymentType, selectedAdditionals, orderTotal } from "../orderCard";

const cartItem = {
  quantity: 2,
  additionals: [2],
  item: {
    name: "Pizza",
    price: 40,
    additional: [
      { ID: 1, name: "Borda", price: 8 },
      { ID: 2, name: "Bacon", price: 5 },
    ],
  },
};

describe("card do pedido", () => {
  it("lê a forma de pagamento gravada em paymentMethod", () => {
    expect(paymentType({ paymentMethod: { type: "pix" } })).toBe("pix");
    expect(paymentType({ paymentmethod: { type: "money" } })).toBe("money");
    expect(paymentType({})).toBe("");
  });

  it("mostra só os adicionais escolhidos", () => {
    expect(selectedAdditionals(cartItem).map((a) => a.name)).toEqual(["Bacon"]);
    expect(selectedAdditionals({ item: { additional: [{ ID: 1 }] } })).toEqual([]);
  });

  it("usa o total do servidor (com frete e cupom)", () => {
    expect(orderTotal({ order_total: 87.5, cart: [cartItem] })).toBe(87.5);
  });

  it("pedido antigo sem order_total soma os itens com os adicionais escolhidos", () => {
    expect(orderTotal({ cart: [cartItem] })).toBe(90);
  });
});
