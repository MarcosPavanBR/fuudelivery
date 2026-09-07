// home_delivery.tsx importa services/api, maplibre e contexto de auth —
// mockados aqui porque só testamos a lógica pura de leitura do pedido
// ativo e montagem do payload de GPS, sem GPS nem mapa de verdade.
jest.mock("@/services/api", () => ({
  __esModule: true,
  default: { get: jest.fn(), post: jest.fn() },
}));
jest.mock("@maplibre/maplibre-react-native", () => ({
  Map: "Map",
  Camera: "Camera",
  ViewAnnotation: "ViewAnnotation",
}));
jest.mock("@/contexts/AuthContext", () => ({ useAuthApi: () => ({}) }));
jest.mock("expo-router/react-navigation", () => ({ useIsFocused: () => true }));
jest.mock("@/componentes/ModalMinimize", () => "MinimizableModal");
jest.mock("@/componentes/HeaderDelivery", () => "HeaderDelivery");

import { getCurrentOrder, buildGpsPayloads } from "../home_delivery";

// Cobre o lado do entregador do rastreamento ao vivo: qual é "o pedido
// ativo" (has-active já devolveu isso de formas inconsistentes — já
// derrubou a tela) e o corpo exato enviado pro backend a cada GPS.

describe("getCurrentOrder", () => {
  it("pega o primeiro pedido quando has-active devolve um array", () => {
    const inWork = { order: [{ order_id: 1 }, { order_id: 2 }] };
    expect(getCurrentOrder(inWork).order_id).toBe(1);
  });

  it("usa o pedido direto quando has-active devolve objeto único", () => {
    const inWork = { order: { order_id: 7 } };
    expect(getCurrentOrder(inWork).order_id).toBe(7);
  });

  it("não quebra quando has-active devolve array vazio (regressão: TypeError em order[0].location)", () => {
    const inWork = { order: [] };
    expect(getCurrentOrder(inWork)).toBeUndefined();
  });

  it("não quebra quando não há pedido nenhum", () => {
    const inWork = { order: null };
    expect(getCurrentOrder(inWork)).toBeNull();
  });
});

describe("buildGpsPayloads", () => {
  const coords = { latitude: -23.5, longitude: -46.6 };

  it("monta os dois payloads quando há um pedido ativo", () => {
    const { deliveryLocation, dispatchLocation } = buildGpsPayloads(42, coords);
    expect(deliveryLocation).toEqual({ lat: -23.5, lng: -46.6, order_id: "42" });
    expect(dispatchLocation).toEqual({ lat: -23.5, lng: -46.6, status: "busy" });
  });

  it("omite o payload de rastreamento do cliente sem pedido ativo, mas sempre manda o de dispatch", () => {
    const { deliveryLocation, dispatchLocation } = buildGpsPayloads(undefined, coords);
    expect(deliveryLocation).toBeNull();
    expect(dispatchLocation).toEqual({ lat: -23.5, lng: -46.6, status: "busy" });
  });

  it("converte order_id numérico pra string (o backend espera string)", () => {
    const { deliveryLocation } = buildGpsPayloads(999, coords);
    expect(deliveryLocation.order_id).toBe("999");
    expect(typeof deliveryLocation.order_id).toBe("string");
  });
});
