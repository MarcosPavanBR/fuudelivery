// cart.tsx importa a cadeia inteira de componentes da tela (HeaderMain,
// PaymentComponent, etc.), que por sua vez chega em services/api — mockado
// aqui porque só queremos a função pura calculateSubtotal, sem inicializar
// cliente HTTP nem exigir EXPO_PUBLIC_API_URL.
jest.mock("@/services/api", () => ({ __esModule: true, default: { post: jest.fn(), get: jest.fn() } }));

import { calculateSubtotal } from "../cart";

// Mesmo cálculo usado no resumo do pedido (OrderSummaryWithTotal) e no botão
// de finalizar compra — é o número que vira o valor cobrado no PIX/cartão,
// então qualquer erro aqui é dinheiro cobrado errado do cliente.

describe("calculateSubtotal", () => {
  it("retorna 0 para carrinho vazio", () => {
    expect(calculateSubtotal([])).toBe(0);
  });

  it("soma preço x quantidade sem adicionais", () => {
    const cart = [{ quantity: 2, item: { Price: 25 } }];
    expect(calculateSubtotal(cart)).toBe(50);
  });

  it("soma adicionais válidos ao preço base, multiplicado pela quantidade", () => {
    const cart = [
      {
        quantity: 2,
        additionals: [1, 2],
        item: {
          Price: 20,
          Additional: [
            { ID: 1, Price: 3 },
            { ID: 2, Price: 5 },
            { ID: 3, Price: 99 }, // não pedido — não deve entrar na conta
          ],
        },
      },
    ];
    // (20 + 3 + 5) * 2 = 56
    expect(calculateSubtotal(cart)).toBe(56);
  });

  it("ignora id de adicional que não existe mais no cardápio (sem quebrar)", () => {
    const cart = [
      {
        quantity: 1,
        additionals: ["id-removido"],
        item: { Price: 10, Additional: [{ ID: "outro-id", Price: 4 }] },
      },
    ];
    expect(calculateSubtotal(cart)).toBe(10);
  });

  it("soma múltiplos itens do carrinho", () => {
    const cart = [
      { quantity: 1, item: { Price: 10 } },
      { quantity: 3, item: { Price: 7 } },
    ];
    // 10 + 21 = 31
    expect(calculateSubtotal(cart)).toBe(31);
  });

  it("trata item sem preço ou sem lista de adicionais como zero, sem lançar erro", () => {
    const cart = [{ quantity: 2, item: {} }, { quantity: 1 }];
    expect(calculateSubtotal(cart)).toBe(0);
  });
});
