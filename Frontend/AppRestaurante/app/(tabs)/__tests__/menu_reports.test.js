jest.mock("@/services/api", () => ({ __esModule: true, default: { get: jest.fn() } }));
jest.mock("expo-router/react-navigation", () => ({ useFocusEffect: jest.fn() }));
jest.mock("@/contexts/ApiContext", () => ({ useApi: () => ({ getUserData: () => null }) }));

import { normalizeProduct } from "../menu";
import { computeStats } from "../reports";

describe("cardápio", () => {
  it("lê o produto no formato do servidor (Name/Price/Categories)", () => {
    const p = normalizeProduct({ ID: 1, Name: "Pizza", Price: 49.9, Categories: [{ ID: 2, Name: "Pizzas" }] });
    expect(p).toEqual({ id: 1, name: "Pizza", description: undefined, price: 49.9, image: undefined, available: true, categories: [{ id: 2, name: "Pizzas" }] });
  });

  it("lê o item pausado (Available=false) e trata ausência como à venda", () => {
    expect(normalizeProduct({ ID: 1, Name: "X", Price: 1, Available: false }).available).toBe(false);
    expect(normalizeProduct({ ID: 1, Name: "X", Price: 1 }).available).toBe(true);
  });

  it("aceita o formato antigo", () => {
    expect(normalizeProduct({ id: 3, name: "Suco", price: 8 }).name).toBe("Suco");
  });
});

describe("relatórios", () => {
  it("usa created_at, order_total e os status do servidor", () => {
    const now = new Date(2026, 8, 25, 20, 0);
    const at = (d, h) => new Date(2026, 8, d, h, 0).toISOString();
    const stats = computeStats(
      [
        { created_at: at(25, 12), order_total: 50, status: "FINISHED" },
        { created_at: at(25, 13), order_total: 30, status: "DENIED" },
        { created_at: at(23, 12), order_total: 70, status: "DONE" },
        { created_at: at(1, 12), order_total: 999, status: "FINISHED" },
      ],
      now
    );
    expect(stats).toEqual({ todayOrders: 1, weekRevenue: 120, avgTicket: 60 });
  });
});
