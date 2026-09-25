// index.tsx importa services/api (exige EXPO_PUBLIC_API_URL) e hooks de
// navegação — mockados aqui porque só testamos a máquina de estado do
// pedido (lógica pura), sem renderizar a tela.
jest.mock("@/services/api", () => ({
  __esModule: true,
  default: { get: jest.fn(), put: jest.fn() },
}));
jest.mock("expo-router/react-navigation", () => ({ useFocusEffect: jest.fn() }));
jest.mock("@/contexts/ApiContext", () => ({ useApi: () => ({ getUserData: () => null }) }));

import {
  resolveOrderStatus,
  getAvailableActions,
  ACTION_TO_STATUS,
  applyStatusUpdate,
  activeOrders,
  shortOrderId,
  elapsedLabel,
} from "../index";

// Os status e as transições são os do servidor
// (validTransitions em Backend/orders_api/app/handlers/orders.go). A versão
// anterior usava pending/preparing/ready — que o servidor não tem — e os
// testes passavam com a tela sem funcionar.
const VALID = {
  AWAIT_APPROVE: ["APPROVED", "DENIED", "CANCELLED"],
  REQUEST_APPROVE: ["APPROVED", "DENIED", "CANCELLED"],
  SCHEDULED: ["APPROVED", "DENIED", "CANCELLED"],
  APPROVED: ["PREPARING", "CANCELLED"],
  PREPARING: ["DONE", "CANCELLED"],
  DONE: ["IN_ROUTE_DELIVERY", "FINISHED"],
  IN_ROUTE_DELIVERY: ["FINISHED"],
};

describe("resolveOrderStatus", () => {
  it("rótulo de cada status do servidor", () => {
    expect(resolveOrderStatus("AWAIT_APPROVE").label).toBe("Aguardando");
    expect(resolveOrderStatus("PREPARING").label).toBe("Em preparo");
    expect(resolveOrderStatus("DONE").label).toBe("Pronto");
    expect(resolveOrderStatus("IN_ROUTE_DELIVERY").label).toBe("A caminho");
    expect(resolveOrderStatus("DENIED").label).toBe("Recusado");
  });

  it("badge neutro para status desconhecido, sem quebrar", () => {
    const badge = resolveOrderStatus("status_novo_do_backend");
    expect(badge.label).toBe("status_novo_do_backend");
    expect(badge.bg).toBe("#F3F4F6");
  });
});

describe("getAvailableActions", () => {
  it("aguardando: aceitar ou recusar", () => {
    expect(getAvailableActions("AWAIT_APPROVE")).toEqual(["accept", "reject"]);
    expect(getAvailableActions("SCHEDULED")).toEqual(["accept", "reject"]);
  });

  it("aceito: iniciar preparo; em preparo: pronto (não aceita de novo)", () => {
    expect(getAvailableActions("APPROVED")).toEqual(["start", "cancel"]);
    expect(getAvailableActions("PREPARING")).toEqual(["ready", "cancel"]);
    expect(getAvailableActions("PREPARING")).not.toContain("accept");
  });

  it("pronto/a caminho: a loja só marca se não houver entregador", () => {
    expect(getAvailableActions("DONE")).toEqual(["dispatched"]);
    expect(getAvailableActions("DONE", true)).toEqual([]);
    expect(getAvailableActions("IN_ROUTE_DELIVERY")).toEqual(["delivered"]);
    expect(getAvailableActions("IN_ROUTE_DELIVERY", true)).toEqual([]);
  });

  it.each(["FINISHED", "DENIED", "CANCELLED"])("pedido em '%s' não tem ação", (status) => {
    expect(getAvailableActions(status)).toEqual([]);
  });

  it("toda ação leva a uma transição que o servidor aceita", () => {
    for (const status of Object.keys(VALID)) {
      for (const action of getAvailableActions(status)) {
        expect(VALID[status]).toContain(ACTION_TO_STATUS[action]);
      }
    }
  });
});

describe("activeOrders", () => {
  it("tira os encerrados e põe primeiro quem espera a loja, mais antigos antes", () => {
    const list = activeOrders([
      { _id: "a", status: "PREPARING", created_at: "2026-09-25T12:00:00Z" },
      { _id: "b", status: "FINISHED", created_at: "2026-09-25T11:00:00Z" },
      { _id: "c", status: "AWAIT_APPROVE", created_at: "2026-09-25T12:10:00Z" },
      { _id: "d", status: "AWAIT_APPROVE", created_at: "2026-09-25T12:05:00Z" },
      { _id: "e", status: "DENIED", created_at: "2026-09-25T12:06:00Z" },
    ]);
    expect(list.map((o) => o._id)).toEqual(["d", "c", "a"]);
  });
});

describe("applyStatusUpdate", () => {
  const orders = [
    { _id: "x1", status: "AWAIT_APPROVE" },
    { _id: "x2", status: "AWAIT_APPROVE" },
    { _id: "x3", status: "PREPARING" },
  ];

  it("atualiza só o pedido alvo e não muta o original", () => {
    const updated = applyStatusUpdate(orders, "x2", "APPROVED");
    expect(updated.find((o) => o._id === "x2").status).toBe("APPROVED");
    expect(updated.find((o) => o._id === "x1").status).toBe("AWAIT_APPROVE");
    expect(orders.find((o) => o._id === "x2").status).toBe("AWAIT_APPROVE");
  });

  it("é inofensivo se o id não existir", () => {
    expect(applyStatusUpdate(orders, "zz", "CANCELLED")).toEqual(orders);
  });
});

describe("apresentação", () => {
  it("número curto e tempo decorrido", () => {
    expect(shortOrderId("65f1c2a9b8e4d3c2a1b0f9e8")).toBe("#F9E8");
    const t0 = Date.parse("2026-09-25T12:00:00Z");
    expect(elapsedLabel("2026-09-25T12:00:00Z", t0 + 12 * 60000)).toBe("há 12 min");
    expect(elapsedLabel(undefined, t0)).toBe("");
  });
});
