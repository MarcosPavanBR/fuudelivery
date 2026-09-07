// index.tsx importa services/api (exige EXPO_PUBLIC_API_URL) e hooks de
// navegação — mockados aqui porque só testamos a máquina de estado do
// pedido (lógica pura), sem renderizar a tela.
jest.mock("@/services/api", () => ({
  __esModule: true,
  default: { get: jest.fn(), patch: jest.fn() },
}));
jest.mock("expo-router/react-navigation", () => ({ useFocusEffect: jest.fn() }));
jest.mock("@/contexts/ApiContext", () => ({ useApi: () => ({ getUserData: () => null }) }));

import {
  resolveOrderStatus,
  getAvailableActions,
  ACTION_TO_STATUS,
  applyStatusUpdate,
} from "../index";

// Cobre o fluxo de aceite/recusa de pedido (e2e-testing: segunda maior
// prioridade depois do checkout) — o que o restaurante pode fazer em cada
// status, pra qual status cada ação leva, e que atualizar um pedido não
// mexe nos outros da lista.

describe("resolveOrderStatus", () => {
  it("resolve a cor/label de cada status conhecido", () => {
    expect(resolveOrderStatus("pending").label).toBe("Pendente");
    expect(resolveOrderStatus("preparing").label).toBe("Preparando");
    expect(resolveOrderStatus("ready").label).toBe("Pronto");
    expect(resolveOrderStatus("cancelled").label).toBe("Cancelado");
  });

  it("cai num badge neutro pra status desconhecido, sem quebrar", () => {
    const badge = resolveOrderStatus("status_novo_do_backend");
    expect(badge.label).toBe("status_novo_do_backend");
    expect(badge.bg).toBe("#F3F4F6");
  });
});

describe("getAvailableActions", () => {
  it("pedido pendente pode ser aceito ou rejeitado", () => {
    expect(getAvailableActions("pending")).toEqual(["accept", "reject"]);
  });

  it("pedido em preparo só pode ser marcado como pronto", () => {
    expect(getAvailableActions("preparing")).toEqual(["ready"]);
  });

  it("pedido já aceito não pode ser aceito de novo (regressão: preparo duplicado)", () => {
    expect(getAvailableActions("preparing")).not.toContain("accept");
  });

  it.each(["ready", "delivering", "delivered", "cancelled", "approved"])(
    "pedido em '%s' não tem ação disponível pro restaurante",
    (status) => {
      expect(getAvailableActions(status)).toEqual([]);
    }
  );
});

describe("ACTION_TO_STATUS", () => {
  it("aceitar leva a 'preparing', rejeitar a 'cancelled', pronto a 'ready'", () => {
    expect(ACTION_TO_STATUS.accept).toBe("preparing");
    expect(ACTION_TO_STATUS.reject).toBe("cancelled");
    expect(ACTION_TO_STATUS.ready).toBe("ready");
  });
});

describe("applyStatusUpdate", () => {
  const orders = [
    { id: 1, status: "pending" },
    { id: 2, status: "pending" },
    { id: 3, status: "preparing" },
  ];

  it("atualiza só o pedido alvo", () => {
    const updated = applyStatusUpdate(orders, 2, "preparing");
    expect(updated.find((o) => o.id === 2).status).toBe("preparing");
  });

  it("não altera os demais pedidos da lista (regressão: atualização em massa)", () => {
    const updated = applyStatusUpdate(orders, 2, "preparing");
    expect(updated.find((o) => o.id === 1).status).toBe("pending");
    expect(updated.find((o) => o.id === 3).status).toBe("preparing");
  });

  it("não muta o array original", () => {
    applyStatusUpdate(orders, 1, "cancelled");
    expect(orders.find((o) => o.id === 1).status).toBe("pending");
  });

  it("é inofensivo se o id não existir na lista (ex.: pedido já removido)", () => {
    const updated = applyStatusUpdate(orders, 999, "cancelled");
    expect(updated).toEqual(orders);
  });
});
