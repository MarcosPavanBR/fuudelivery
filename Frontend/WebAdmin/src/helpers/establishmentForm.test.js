import { describe, it, expect } from "vitest";
import { updatePayload, createPayload, formFromEstablishment, matchesSearch, matchesStatus, formatPhone, geocodeAddress } from "./establishmentForm";

const est = {
  id: 7, name: "Pizzaria", description: "Forno a lenha", image: "https://x/logo.jpg",
  primary_color: "#111111", secondary_color: "#222222", horarioFuncionamento: "Seg–Dom 18h–23h",
  location_string: "Rua A, 10", lat: -23.5, long: -46.6, max_distance_delivery: 8, zone_id: 3,
  owner_name: "Rui", owner_email: "rui@x.com", owner_phone: "11988887777", accepting_orders: true,
};

describe("establishmentForm", () => {
  it("editar não apaga foto, cores e horário", () => {
    const form = { ...formFromEstablishment(est), name: " Pizzaria Nova ", lat: "-23.6", max_distance_delivery: "12" };
    const { establishment: p } = updatePayload(est, form);
    expect(p.name).toBe("Pizzaria Nova");
    expect(p.image).toBe(est.image);
    expect(p.primary_color).toBe("#111111");
    expect(p.horarioFuncionamento).toBe(est.horarioFuncionamento);
    expect(p.lat).toBe(-23.6);
    expect(p.max_distance_delivery).toBe(12);
    expect(p.zone_id).toBe(3);
  });

  it("sem região não manda zone_id", () => {
    expect(updatePayload(est, { ...formFromEstablishment(est), zone_id: "" }).establishment).not.toHaveProperty("zone_id");
    expect(createPayload({ name: "X", zone_id: "" })).not.toHaveProperty("zone_id");
  });

  it("criar usa os nomes que o servidor lê", () => {
    expect(createPayload({ name: "X", location_string: "Rua B", lat: "1.5", long: "", max_distance_delivery: "6", zone_id: "2" }))
      .toEqual({ name: "X", description: "", address: "Rua B", latitude: 1.5, longitude: 0, max_distance_delivery: 6, zone_id: 2 });
  });

  it("busca e filtro", () => {
    expect(matchesSearch(est, "rui@")).toBe(true);
    expect(matchesSearch(est, "rua a")).toBe(true);
    expect(matchesSearch(est, "sushi")).toBe(false);
    expect(matchesStatus(est, "open")).toBe(true);
    expect(matchesStatus({ ...est, accepting_orders: false }, "open")).toBe(false);
    expect(matchesStatus(est, "")).toBe(true);
  });

  it("telefone", () => {
    expect(formatPhone("+5511988887777")).toBe("(11) 98888-7777");
    expect(formatPhone("")).toBe("");
  });

  it("geocode", async () => {
    const ok = async () => ({ ok: true, json: async () => [{ lat: "-23.1", lon: "-46.2" }] });
    expect(await geocodeAddress("Rua A", ok)).toEqual({ lat: -23.1, long: -46.2 });
    expect(await geocodeAddress("", ok)).toBeNull();
    expect(await geocodeAddress("x", async () => ({ ok: true, json: async () => [] }))).toBeNull();
  });
});
