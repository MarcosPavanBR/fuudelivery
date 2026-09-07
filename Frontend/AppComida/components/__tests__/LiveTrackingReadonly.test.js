// LiveTrackingReadonly importa services/api (via ApiContext) e maplibre —
// mockados aqui porque só testamos a lógica pura do rastreamento (parse de
// mensagem WS, decisão de reconexão, centro do mapa), sem abrir socket nem
// renderizar o mapa de verdade.
jest.mock("@/services/api", () => ({
  __esModule: true,
  default: { get: jest.fn(), post: jest.fn() },
}));
jest.mock("@maplibre/maplibre-react-native", () => ({
  Map: "Map",
  Camera: "Camera",
  ViewAnnotation: "ViewAnnotation",
}));

import {
  parseLocationMessage,
  shouldReconnect,
  resolveMapCenter,
} from "../LiveTrackingReadonly";

// Cobre o rastreamento ao vivo do entregador (terceira prioridade do
// e2e-testing, depois de checkout e aceite/recusa) — o cliente vendo a
// localização atualizar via WebSocket sem travar nem crashar com payload
// inesperado.

describe("parseLocationMessage", () => {
  it("extrai o payload de uma mensagem de localização válida", () => {
    const raw = JSON.stringify({
      type: "location",
      payload: { lat: -23.5, lng: -46.6, order_id: "42", timestamp: 123 },
    });
    expect(parseLocationMessage(raw)).toEqual({
      lat: -23.5,
      lng: -46.6,
      order_id: "42",
      timestamp: 123,
    });
  });

  it("ignora mensagens de outro tipo (ex.: keepalive) sem lançar erro", () => {
    expect(parseLocationMessage(JSON.stringify({ type: "ping" }))).toBeNull();
  });

  it("ignora mensagem de localização sem payload", () => {
    expect(parseLocationMessage(JSON.stringify({ type: "location" }))).toBeNull();
  });

  it("não quebra com payload não-JSON", () => {
    expect(parseLocationMessage("not json")).toBeNull();
    expect(parseLocationMessage("")).toBeNull();
  });
});

describe("shouldReconnect", () => {
  it("reconecta enquanto não atingiu o limite de tentativas", () => {
    expect(shouldReconnect(0, 20)).toBe(true);
    expect(shouldReconnect(19, 20)).toBe(true);
  });

  it("para de reconectar ao atingir o limite (regressão: loop infinito de reconexão)", () => {
    expect(shouldReconnect(20, 20)).toBe(false);
    expect(shouldReconnect(21, 20)).toBe(false);
  });
});

describe("resolveMapCenter", () => {
  it("prioriza a posição ao vivo do entregador quando disponível", () => {
    const center = resolveMapCenter({
      deliveryLocation: { lat: 1, lng: 2, order_id: "1", timestamp: 0 },
      destinationLat: 3,
      destinationLng: 4,
      originLat: 5,
      originLng: 6,
    });
    expect(center).toEqual({ lat: 1, lng: 2 });
  });

  it("cai pro destino quando ainda não há posição do entregador", () => {
    const center = resolveMapCenter({
      deliveryLocation: null,
      destinationLat: 3,
      destinationLng: 4,
      originLat: 5,
      originLng: 6,
    });
    expect(center).toEqual({ lat: 3, lng: 4 });
  });

  it("cai pra origem quando não há posição nem destino", () => {
    const center = resolveMapCenter({
      deliveryLocation: null,
      originLat: 5,
      originLng: 6,
    });
    expect(center).toEqual({ lat: 5, lng: 6 });
  });

  it("nunca fica undefined — cai no fallback fixo sem nenhuma coordenada (regressão: mapa quebrando ao montar)", () => {
    const center = resolveMapCenter({ deliveryLocation: null });
    expect(center.lat).toBe(-23.5505);
    expect(center.lng).toBe(-46.6333);
  });
});
