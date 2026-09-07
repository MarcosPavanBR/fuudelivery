// Mocka o cliente HTTP: só testamos lógica pura aqui (sem chamada de rede),
// e importar o módulo real dispara a checagem de EXPO_PUBLIC_API_URL, que
// só existe em build/dev real.
jest.mock("@/services/api", () => ({ __esModule: true, default: { post: jest.fn(), get: jest.fn() } }));

import {
  isDeliveryValid,
  buildOrderPayload,
  parseOrderResponse,
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
