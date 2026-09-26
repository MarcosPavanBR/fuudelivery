import { describe, it, expect } from "vitest";
import { normalizeOrder, statusInfo, money, isToday } from "./orders";

const raw = {
  _id: "ee12c0f7dc9d5cb4b09d8711", order_id: "ee12c0f7dc9d5cb4b09d8711",
  cart: [{ item: { Name: "Pizza", Price: 49.9, Additional: [{ Name: "Borda", Price: 5 }] }, note: "sem cebola", quantity: 2 }],
  created_at: "2026-09-26T02:06:04Z", deliveryValue: 7, deliveryman: { name: "" },
  establishment: { Id: 1, name: "Bella Napoli" }, establishmentId: 1,
  location: { logradouro: "Av. Paulista", numero: "1000", bairro: "Bela Vista", localidade: "São Paulo", uf: "SP" },
  order_total: 116.8, paymentMethod: { type: "money" }, pickup_code: "555037", status: "FINISHED",
  user: { nome: "Ana", phone: "+5511988887777" },
};

describe("normalizeOrder", () => {
  it("lê os campos que o servidor manda", () => {
    const o = normalizeOrder(raw);
    expect(o.id).toBe("ee12c0f7dc9d5cb4b09d8711");
    expect(o.total).toBe(116.8);
    expect(o.createdAt).toBe("2026-09-26T02:06:04Z");
    expect(o.establishment).toBe("Bella Napoli");
    expect(o.payment).toBe("Dinheiro");
    expect(o.items).toEqual([{ name: "Pizza", quantity: 2, subtotal: 109.8, note: "sem cebola", additionals: ["Borda"] }]);
    expect(o.address).toBe("Av. Paulista, 1000 · Bela Vista · São Paulo - SP");
    expect(statusInfo(o.status).label).toBe("Entregue");
  });

  it("aguenta pedido vazio", () => {
    const o = normalizeOrder({});
    expect(o.total).toBe(0);
    expect(o.items).toEqual([]);
    expect(statusInfo("XYZ").label).toBe("XYZ");
  });

  it("formatos", () => {
    expect(money(12.5)).toMatch(/12,50/);
    expect(isToday(new Date().toISOString())).toBe(true);
    expect(isToday(null)).toBe(false);
  });
});
