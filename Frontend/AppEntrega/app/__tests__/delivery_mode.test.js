// delivery_mode.tsx importa services/api, maplibre e contexto de auth —
// mockados aqui porque só testamos a máquina de estado da entrega, sem
// renderizar mapa nem swipe button de verdade.
jest.mock("@/services/api", () => ({
  __esModule: true,
  default: { post: jest.fn() },
}));
jest.mock("@maplibre/maplibre-react-native", () => ({
  Map: "Map",
  Camera: "Camera",
  ViewAnnotation: "ViewAnnotation",
}));
jest.mock("@/contexts/AuthContext", () => ({ useAuthApi: () => ({}) }));
jest.mock("@/componentes/SwipButton", () => "SwipeButtonDelivery");

import { nextDeliveryStatus, needsPickupCode, isSwipeDisabled } from "../delivery_mode";

// Cobre a progressão de status do entregador — o equivalente, do lado de
// quem entrega, do aceite/recusa de pedido do restaurante.

describe("nextDeliveryStatus", () => {
  it("segue a progressão: coleta -> aguardando -> a caminho -> finalizado", () => {
    expect(nextDeliveryStatus("IN_ROUTE_COLECT")).toBe("AWAIT_COLECT");
    expect(nextDeliveryStatus("AWAIT_COLECT")).toBe("IN_ROUTE_DELIVERY");
    expect(nextDeliveryStatus("IN_ROUTE_DELIVERY")).toBe("FINISHED");
  });

  it("status desconhecido reinicia no primeiro passo (mesmo comportamento do switch original)", () => {
    expect(nextDeliveryStatus("ALGO_QUE_NAO_EXISTE")).toBe("IN_ROUTE_COLECT");
    expect(nextDeliveryStatus("FINISHED")).toBe("IN_ROUTE_COLECT");
  });
});

describe("needsPickupCode", () => {
  it("exige código só nas etapas de confirmação com estabelecimento/cliente", () => {
    expect(needsPickupCode("AWAIT_COLECT")).toBe(true);
    expect(needsPickupCode("IN_ROUTE_DELIVERY")).toBe(true);
  });

  it("não exige código nas demais etapas", () => {
    expect(needsPickupCode("IN_ROUTE_COLECT")).toBe(false);
    expect(needsPickupCode("FINISHED")).toBe(false);
  });
});

describe("isSwipeDisabled", () => {
  it("trava enquanto aguarda coleta e o pedido ainda não está pronto", () => {
    expect(isSwipeDisabled({ status: "AWAIT_COLECT" }, { status: "PREPARING" })).toBe(true);
  });

  it("libera aguardando coleta assim que o pedido está pronto (DONE)", () => {
    expect(isSwipeDisabled({ status: "AWAIT_COLECT" }, { status: "DONE" })).toBe(false);
  });

  it("trava sempre que o pedido ainda espera aprovação, em qualquer status do entregador", () => {
    expect(isSwipeDisabled({ status: "IN_ROUTE_DELIVERY" }, { status: "AWAIT_APPROVE" })).toBe(true);
  });

  it("libera nas demais combinações", () => {
    expect(isSwipeDisabled({ status: "IN_ROUTE_COLECT" }, { status: "PREPARING" })).toBe(false);
    expect(isSwipeDisabled({ status: "IN_ROUTE_DELIVERY" }, { status: "DONE" })).toBe(false);
  });
});
