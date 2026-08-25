import { describe, it, expect, vi, beforeEach } from "vitest";

vi.mock("../api", () => ({
  default: {
    get: vi.fn(),
    put: vi.fn(),
  },
}));

import api from "../api";
import ordersModels from "../orders.models";

const mockedGet = api.get;
const mockedPut = api.put;

beforeEach(() => {
  vi.clearAllMocks();
});

describe("ordersModels.getOrders", () => {
  it("mapeia pedidos para {id, column, data}", async () => {
    mockedGet.mockResolvedValueOnce({
      data: [
        { _id: "abc", status: "AWAIT_APPROVE", user: { nome: "Ana" } },
        { _id: "def", status: "APPROVED", user: { nome: "Bruno" } },
      ],
    });

    const orders = await ordersModels.getOrders(1);
    expect(orders).toHaveLength(2);
    expect(orders[0]).toMatchObject({ id: "abc", column: "AWAIT_APPROVE" });
    expect(orders[1].data.user.nome).toBe("Bruno");
  });

  it("exclui pedidos finalizados pelo entregador", async () => {
    mockedGet.mockResolvedValueOnce({
      data: [
        { _id: "ativo", status: "APPROVED" },
        { _id: "entregue", status: "DONE", deliveryman: { status: "FINISHED" } },
      ],
    });

    const orders = await ordersModels.getOrders(1);
    expect(orders.map((o) => o.id)).toEqual(["ativo"]);
  });

  it("propaga erro de API (não engole como lista vazia)", async () => {
    mockedGet.mockRejectedValueOnce(new Error("network down"));
    await expect(ordersModels.getOrders(1)).rejects.toThrow("network down");
  });
});

describe("ordersModels.alterStatus", () => {
  it("retorna true quando a API aceita", async () => {
    mockedPut.mockResolvedValueOnce({ data: {} });
    await expect(ordersModels.alterStatus("APPROVED", "abc")).resolves.toBe(true);
    expect(mockedPut).toHaveBeenCalledWith("/orders/status", {
      id: "abc",
      status: "APPROVED",
    });
  });

  it("retorna false quando a API falha (permite rollback no Kanban)", async () => {
    mockedPut.mockRejectedValueOnce(new Error("500"));
    await expect(ordersModels.alterStatus("DONE", "abc")).resolves.toBe(false);
  });
});
