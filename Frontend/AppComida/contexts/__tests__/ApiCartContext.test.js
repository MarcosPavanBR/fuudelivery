// Mocka o cliente HTTP: só testamos lógica pura aqui (sem chamada de rede),
// e importar o módulo real dispara a checagem de EXPO_PUBLIC_API_URL, que
// só existe em build/dev real.
jest.mock("@/services/api", () => ({ __esModule: true, default: { post: jest.fn(), get: jest.fn() } }));

import {
  isDeliveryValid,
  buildOrderPayload,
  parseOrderResponse,
  orderErrorMessage,
} from "../ApiCartContext";

// Lógica pura extraída do checkout (submitCart/validDelivery) — cobre o
// fluxo de maior risco financeiro do AppComida: validar se dá pra entregar,
// montar o corpo do pedido com o estabelecimento/taxa certos, e interpretar
// a resposta da API. Testado sem renderizar a tela (ver README do tdd-workflow:
// comece pelo fluxo crítico antes de telas de exibição).

describe("isDeliveryValid", () => {
  const establishment = { max_distance_delivery: 15 };

  it("é inválida sem distância calculada (null)", () => {
    expect(isDeliveryValid(null, establishment)).toBe(false);
  });

  it("é inválida com distância zero (ainda não calculada)", () => {
    expect(isDeliveryValid(0, establishment)).toBe(false);
  });

  it("é válida dentro do raio do estabelecimento", () => {
    expect(isDeliveryValid(10, establishment)).toBe(true);
  });

  it("é válida exatamente no limite do raio", () => {
    expect(isDeliveryValid(15, establishment)).toBe(true);
  });

  it("é inválida acima do raio do estabelecimento", () => {
    expect(isDeliveryValid(15.01, establishment)).toBe(false);
  });
});

describe("buildOrderPayload", () => {
  const establishment = { id: 7, name: "Restaurante Teste", max_distance_delivery: 15 };
  const baseParams = {
    cart: [{ id: "a1", quantity: 2 }],
    distance: 3.5,
    location: { logradouro: "Rua X", bairro: "Centro" },
    coordsLocation: { lat: -23.55, lng: -46.63 },
    paymentMethod: { type: "pix" },
    deliveryValue: 8.9,
    user: { id: 42, name: "Cliente" },
    establishment,
  };

  it("usa o id do estabelecimento atual, não de um anterior", () => {
    const body = buildOrderPayload(baseParams);
    expect(body.establishmentId).toBe(7);
    expect(body.establishment.id).toBe(7);
  });

  it("embute as coordenadas dentro de location.coords sem perder o resto do endereço", () => {
    const body = buildOrderPayload(baseParams);
    expect(body.location).toEqual({
      logradouro: "Rua X",
      bairro: "Centro",
      coords: { lat: -23.55, lng: -46.63 },
    });
  });

  it("repassa taxa de entrega, distância, carrinho e método de pagamento sem alterar", () => {
    const body = buildOrderPayload(baseParams);
    expect(body.deliveryValue).toBe(8.9);
    expect(body.distance).toBe(3.5);
    expect(body.cart).toBe(baseParams.cart);
    expect(body.paymentMethod).toEqual({ type: "pix" });
    expect(body.user).toEqual({ id: 42, name: "Cliente" });
  });

  it("troca de estabelecimento reflete no payload (regressão: taxa do restaurante errado)", () => {
    const outroEstablishment = { id: 99, name: "Outro Restaurante", max_distance_delivery: 5 };
    const body = buildOrderPayload({ ...baseParams, establishment: outroEstablishment, deliveryValue: 3.0 });
    expect(body.establishmentId).toBe(99);
    expect(body.deliveryValue).toBe(3.0);
  });
});

describe("parseOrderResponse", () => {
  it("extrai o orderId como string para uso posterior no PIX", () => {
    expect(parseOrderResponse({ orderId: 123, message: "ok" })).toEqual({
      ok: true,
      orderId: "123",
    });
  });

  it("retorna ok sem orderId quando a API não devolve um", () => {
    expect(parseOrderResponse({ message: "ok" })).toEqual({ ok: true, orderId: undefined });
  });

  it("não quebra com resposta vazia/undefined", () => {
    expect(parseOrderResponse(undefined)).toEqual({ ok: true, orderId: undefined });
    expect(parseOrderResponse(null)).toEqual({ ok: true, orderId: undefined });
  });
});

// ── Cupom no checkout ──
//
// O sistema de cupom estava completo do servidor para dentro (admin cria,
// pedido desconta, split cobra de quem banca) e não tinha porta de entrada
// no app: buildOrderPayload não mandava o código, então o cliente não tinha
// como usar cupom nenhum. Estes testes fixam essa porta.

describe("buildOrderPayload — cupom", () => {
  const base = {
    cart: [{ item: { id: 1, Price: 10 }, quantity: 1, additionals: [] }],
    distance: 3,
    location: { rua: "X" },
    coordsLocation: { lat: 1, lng: 2 },
    paymentMethod: { type: "pix" },
    deliveryValue: 11,
    user: { phone: "+5511999900001" },
    establishment: { id: 7 },
  };

  it("manda o código quando o cliente digita um", () => {
    const body = buildOrderPayload({ ...base, couponCode: "PROMO10" });
    expect(body.coupon_code).toBe("PROMO10");
  });

  // Normaliza para o cliente não perder desconto por ter digitado minúsculo
  // ou com espaço sobrando.
  it("normaliza o código (maiúsculo, sem espaços)", () => {
    const body = buildOrderPayload({ ...base, couponCode: "  promo10 " });
    expect(body.coupon_code).toBe("PROMO10");
  });

  // Pedido sem cupom tem de sair EXATAMENTE como saía antes: um campo vazio
  // no corpo faria o servidor tratar como cupom inválido e recusar o pedido.
  it("omite o campo quando não há cupom", () => {
    expect(buildOrderPayload(base).coupon_code).toBeUndefined();
    expect(buildOrderPayload({ ...base, couponCode: "" }).coupon_code).toBeUndefined();
    expect(buildOrderPayload({ ...base, couponCode: "   " }).coupon_code).toBeUndefined();
  });

  // Só o código viaja: valor do desconto e quem banca são decididos no
  // servidor a partir do cupom no banco.
  it("não manda valor de desconto nenhum", () => {
    const body = buildOrderPayload({ ...base, couponCode: "PROMO10" });
    expect(body).not.toHaveProperty("discount_amount");
    expect(body).not.toHaveProperty("discount_funded_by");
  });
});

describe("parseOrderResponse — total do servidor", () => {
  // É o número que a cobrança PIX tem de usar. payment_api confere o amount
  // contra o order_total gravado no pedido: recalculando no app, toda
  // cobrança de pedido com cupom seria recusada.
  it("lê o total e o desconto que o servidor devolveu", () => {
    const res = parseOrderResponse({
      orderId: "abc123",
      order_total: 65,
      discount_amount: 6,
      coupon_code: "DEZ",
    });
    expect(res.orderId).toBe("abc123");
    expect(res.orderTotal).toBe(65);
    expect(res.discount).toBe(6);
    expect(res.couponCode).toBe("DEZ");
  });

  // Resposta de servidor antigo não traz os campos — o chamador precisa
  // conseguir distinguir "não veio" de "veio zero" para cair no cálculo
  // local em vez de cobrar R$ 0,00.
  it("deixa o total indefinido quando o servidor não manda", () => {
    const res = parseOrderResponse({ orderId: "abc123" });
    expect(res.orderId).toBe("abc123");
    expect(res.orderTotal).toBeUndefined();
    expect(res.discount).toBeUndefined();
  });

  // Frete ZERO é legítimo — retirada no balcão, frete grátis de assinatura.
  // Tratar como ausente faria a cobrança declarar a cotação local em vez do
  // zero que o servidor decidiu.
  it("aceita frete zero, que é um valor válido", () => {
    expect(parseOrderResponse({ delivery_value: 0 }).deliveryValue).toBe(0);
    expect(parseOrderResponse({ delivery_value: 11 }).deliveryValue).toBe(11);
  });

  it("deixa o frete indefinido quando o servidor não manda", () => {
    expect(parseOrderResponse({ orderId: "x" }).deliveryValue).toBeUndefined();
    expect(parseOrderResponse({ delivery_value: -1 }).deliveryValue).toBeUndefined();
    expect(parseOrderResponse({ delivery_value: "abc" }).deliveryValue).toBeUndefined();
  });

  it("ignora total zero ou inválido em vez de cobrar zero", () => {
    expect(parseOrderResponse({ order_total: 0 }).orderTotal).toBeUndefined();
    expect(parseOrderResponse({ order_total: "abc" }).orderTotal).toBeUndefined();
    expect(parseOrderResponse({ order_total: -5 }).orderTotal).toBeUndefined();
  });
});

describe("orderErrorMessage", () => {
  // O servidor recusa o pedido inteiro com "cupom: cupom expirado" e afins.
  // Engolir isso num erro genérico deixaria o cliente sem saber que o
  // problema é o código que ele digitou.
  it("mostra a mensagem do servidor quando existe", () => {
    const err = { response: { data: { error: "cupom: cupom expirado" } } };
    expect(orderErrorMessage(err, "genérico")).toBe("cupom: cupom expirado");
  });

  it("cai no texto padrão quando não há mensagem do servidor", () => {
    expect(orderErrorMessage({}, "genérico")).toBe("genérico");
    expect(orderErrorMessage(null, "genérico")).toBe("genérico");
    expect(orderErrorMessage({ response: { data: {} } }, "genérico")).toBe("genérico");
    expect(
      orderErrorMessage({ response: { data: { error: "   " } } }, "genérico")
    ).toBe("genérico");
  });

  // Erro de rede (sem response) não pode virar "[object Object]" na tela.
  it("cai no texto padrão em erro de rede", () => {
    expect(orderErrorMessage(new Error("Network Error"), "genérico")).toBe("genérico");
  });
});
