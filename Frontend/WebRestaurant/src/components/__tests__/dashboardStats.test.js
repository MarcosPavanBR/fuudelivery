import { describe, it, expect } from "vitest";
import { countToday, formatBRL, localDayKey, topProducts, weekdayLabel } from "../dashboardStats";

const pizza = { item: { Name: "Pizza" }, quantity: 2 };
const suco = { item: { Name: "Suco" }, quantity: 1 };

describe("painel da loja", () => {
  it("dia da semana do relatório não volta um dia no fuso do Brasil", () => {
    // 2026-09-25 é sexta-feira.
    expect(weekdayLabel("2026-09-25")).toBe("Sex");
    expect(weekdayLabel("2026-09-20")).toBe("Dom");
  });

  it("chave de dia local", () => {
    expect(localDayKey(new Date(2026, 8, 25, 23, 30))).toBe("2026-09-25");
    expect(localDayKey("x")).toBe("");
  });

  it("pedidos de hoje pela data local", () => {
    const now = new Date(2026, 8, 25, 22, 0).getTime();
    const orders = [
      { created_at: new Date(2026, 8, 25, 21, 30).toISOString() },
      { created_at: new Date(2026, 8, 24, 23, 0).toISOString() },
    ];
    expect(countToday(orders, now)).toBe(1);
  });

  it("mais vendidos da semana, sem recusados/cancelados nem pedidos antigos", () => {
    const now = Date.parse("2026-09-25T15:00:00Z");
    const orders = [
      { created_at: "2026-09-25T12:00:00Z", status: "DONE", cart: [pizza, suco] },
      { created_at: "2026-09-24T12:00:00Z", status: "FINISHED", cart: [pizza] },
      { created_at: "2026-09-24T12:00:00Z", status: "CANCELLED", cart: [suco, suco, suco] },
      { created_at: "2026-09-01T12:00:00Z", status: "FINISHED", cart: [suco, suco, suco] },
    ];
    expect(topProducts(orders, now)).toEqual([
      { name: "Pizza", count: 4 },
      { name: "Suco", count: 1 },
    ]);
  });

  it("moeda em reais", () => {
    expect(formatBRL(1480, true).replace(/\s/g, " ")).toBe("R$ 1.480");
    expect(formatBRL(78.4).replace(/\s/g, " ")).toBe("R$ 78,40");
  });
});
