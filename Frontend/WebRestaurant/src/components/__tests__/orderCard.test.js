import { describe, it, expect } from "vitest";
import {
  actionsFor,
  canMove,
  displayColumn,
  elapsedLabel,
  isLate,
  itemName,
  itemNote,
  newPendingIds,
  orderTotal,
  paymentType,
  selectedAdditionals,
  shortOrderId,
} from "../orderCard";

// Como o servidor grava (chaves do DTO em Go).
const cartItem = {
  quantity: 2,
  additionals: [2],
  item: {
    Name: "Pizza",
    Price: 40,
    Additional: [
      { ID: 1, Name: "Borda", Price: 8 },
      { ID: 2, Name: "Bacon", Price: 5 },
    ],
  },
};

describe("leitura do pedido", () => {
  it("nome do item nas chaves do servidor e nas antigas", () => {
    expect(itemName(cartItem)).toBe("Pizza");
    expect(itemName({ item: { name: "Suco" } })).toBe("Suco");
  });

  it("forma de pagamento gravada em paymentMethod", () => {
    expect(paymentType({ paymentMethod: { type: "pix" } })).toBe("pix");
    expect(paymentType({ paymentmethod: { type: "money" } })).toBe("money");
    expect(paymentType({})).toBe("");
  });

  it("só os adicionais escolhidos", () => {
    expect(selectedAdditionals(cartItem).map((a) => a.name)).toEqual(["Bacon"]);
    expect(selectedAdditionals({ item: { additional: [{ ID: 1 }] } })).toEqual([]);
  });

  it("total do servidor; sem ele, soma dos itens com os adicionais escolhidos", () => {
    expect(orderTotal({ order_total: 87.5, cart: [cartItem] })).toBe(87.5);
    expect(orderTotal({ cart: [cartItem] })).toBe(90);
  });

  it("número curto do pedido", () => {
    expect(shortOrderId("65f1c2a9b8e4d3c2a1b0f9e8")).toBe("#F9E8");
    expect(shortOrderId("")).toBe("");
  });
});

describe("colunas e transições", () => {
  it("agendado e aguardando aprovação aparecem em Em análise", () => {
    expect(displayColumn("SCHEDULED")).toBe("AWAIT_APPROVE");
    expect(displayColumn("REQUEST_APPROVE")).toBe("AWAIT_APPROVE");
    expect(displayColumn("PREPARING")).toBe("PREPARING");
  });

  it("espelha as transições do servidor", () => {
    expect(canMove("AWAIT_APPROVE", "APPROVED")).toBe(true);
    expect(canMove("SCHEDULED", "APPROVED")).toBe(true);
    expect(canMove("APPROVED", "DONE")).toBe(false);
    expect(canMove("DONE", "PREPARING")).toBe(false);
    expect(canMove("FINISHED", "DONE")).toBe(false);
  });

  it("botões por status, sempre transições válidas", () => {
    const labels = (d) => actionsFor(d).map((a) => a.label);
    expect(labels({ status: "AWAIT_APPROVE" })).toEqual(["Aceitar", "Recusar"]);
    expect(labels({ status: "APPROVED" })).toEqual(["Iniciar preparo", "Cancelar"]);
    expect(labels({ status: "PREPARING" })).toEqual(["Pronto", "Cancelar"]);
    expect(labels({ status: "DONE" })).toEqual(["Saiu p/ entrega"]);
    expect(labels({ status: "DONE", deliveryman: { id: 7 } })).toEqual([]);
    expect(labels({ status: "IN_ROUTE_DELIVERY" })).toEqual(["Entregue"]);
    expect(labels({ status: "FINISHED" })).toEqual([]);
    for (const status of ["AWAIT_APPROVE", "SCHEDULED", "APPROVED", "PREPARING", "DONE", "IN_ROUTE_DELIVERY"]) {
      for (const a of actionsFor({ status })) {
        expect(canMove(status, a.to)).toBe(true);
      }
    }
  });

  it("recusar e cancelar pedem confirmação", () => {
    expect(actionsFor({ status: "AWAIT_APPROVE" })[1].confirm).toBeTruthy();
    expect(actionsFor({ status: "PREPARING" })[1].confirm).toBeTruthy();
    expect(actionsFor({ status: "AWAIT_APPROVE" })[0].confirm).toBeFalsy();
  });
});

describe("tempo e alerta", () => {
  const t0 = Date.parse("2026-09-25T12:00:00Z");

  it("tempo decorrido", () => {
    expect(elapsedLabel("2026-09-25T12:00:00Z", t0 + 20 * 1000)).toBe("agora");
    expect(elapsedLabel("2026-09-25T12:00:00Z", t0 + 12 * 60000)).toBe("há 12 min");
    expect(elapsedLabel("2026-09-25T12:00:00Z", t0 + 80 * 60000)).toBe("há 1 h 20 min");
    expect(elapsedLabel("x", t0)).toBe("");
  });

  it("em análise há mais de 5 min fica em destaque (agendado não)", () => {
    const d = { status: "AWAIT_APPROVE", created_at: "2026-09-25T12:00:00Z" };
    expect(isLate(d, t0 + 4 * 60000)).toBe(false);
    expect(isLate(d, t0 + 6 * 60000)).toBe(true);
    expect(isLate({ ...d, status: "SCHEDULED" }, t0 + 60 * 60000)).toBe(false);
    expect(isLate({ ...d, status: "PREPARING" }, t0 + 60 * 60000)).toBe(false);
  });

  it("alerta só para pedido novo em análise, nunca na primeira carga", () => {
    const orders = [
      { id: "a", data: { status: "AWAIT_APPROVE" } },
      { id: "b", data: { status: "PREPARING" } },
      { id: "c", data: { status: "SCHEDULED" } },
    ];
    expect(newPendingIds(null, orders)).toEqual([]);
    expect(newPendingIds(new Set(["a"]), orders)).toEqual(["c"]);
    expect(newPendingIds(new Set(["a", "c"]), orders)).toEqual([]);
  });
});

describe("itemNote", () => {
  it("lê a observação do item e ignora vazio", () => {
    expect(itemNote({ note: "  sem cebola " })).toBe("sem cebola");
    expect(itemNote({ note: "" })).toBe("");
    expect(itemNote({})).toBe("");
    expect(itemNote(null)).toBe("");
  });
});
