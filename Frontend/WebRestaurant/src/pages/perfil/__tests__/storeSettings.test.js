import {
  defaultHours,
  establishmentPayload,
  geocodeAddress,
  geocodeUrl,
  hoursPayload,
  mergeHours,
  needsGeocode,
  parseGeocode,
  summarizeHours,
  validateHours,
} from "../storeSettings";

describe("horários", () => {
  it("mergeHours completa a semana e respeita o que veio do servidor", () => {
    const merged = mergeHours([{ day_of_week: 0, is_open: true, open_time: "10:00", close_time: "14:00" }]);
    expect(merged).toHaveLength(7);
    expect(merged[0]).toMatchObject({ is_open: true, open_time: "10:00", close_time: "14:00" });
    expect(merged[1]).toMatchObject({ is_open: true, open_time: "08:00" });
    expect(mergeHours(null)).toEqual(defaultHours());
  });

  it("horários iguais são 24 h", () => {
    const h = defaultHours().map((d) => ({ ...d, is_open: true, open_time: "00:00", close_time: "00:00" }));
    expect(validateHours(h)).toBeNull();
    expect(summarizeHours(h)).toBe("Todos os dias, 24 h");
  });

  it("validateHours barra dia aberto sem horário e aceita virada da madrugada", () => {
    const h = defaultHours();
    expect(validateHours(h)).toBeNull();
    h[5] = { ...h[5], open_time: "18:00", close_time: "02:00" };
    expect(validateHours(h)).toBeNull();
    h[2] = { ...h[2], close_time: "" };
    expect(validateHours(h)).toMatch(/Terça/);
    const fechado = defaultHours().map((d) => ({ ...d, is_open: false, open_time: "" }));
    expect(validateHours(fechado)).toBeNull();
  });

  it("hoursPayload manda establishment_id numérico (o servidor espera uint)", () => {
    const p = hoursPayload(defaultHours(), "7");
    expect(p).toHaveLength(7);
    expect(p[0].establishment_id).toBe(7);
    expect(p[0].is_open).toBe(false);
  });

  it("summarizeHours resume a grade", () => {
    expect(summarizeHours(defaultHours())).toBe("Seg a Sáb, 08:00–22:00");
    expect(summarizeHours(defaultHours().map((d) => ({ ...d, is_open: true })))).toBe("Todos os dias, 08:00–22:00");
    expect(summarizeHours(defaultHours().map((d) => ({ ...d, is_open: false })))).toBe("Fechado");
    const h = defaultHours();
    h[3] = { ...h[3], close_time: "23:00" };
    expect(summarizeHours(h)).toMatch(/variados/);
    const fds = defaultHours().map((d) => ({ ...d, is_open: d.day_of_week === 0 || d.day_of_week === 6 }));
    expect(summarizeHours(fds)).toBe("Dom, Sáb, 08:00–22:00");
  });
});

describe("endereço → coordenadas", () => {
  it("geocodeUrl codifica o endereço e limita ao Brasil", () => {
    const url = geocodeUrl("Rua Augusta, 1200 - São Paulo");
    expect(url).toContain("countrycodes=br");
    expect(url).toContain(encodeURIComponent("Rua Augusta, 1200 - São Paulo, Brasil"));
  });

  it("parseGeocode lê a resposta do Nominatim", () => {
    expect(parseGeocode([{ lat: "-23.55", lon: "-46.63" }])).toEqual({ lat: -23.55, long: -46.63 });
    expect(parseGeocode([])).toBeNull();
    expect(parseGeocode([{ lat: "x", lon: "1" }])).toBeNull();
    expect(parseGeocode({})).toBeNull();
  });

  it("geocodeAddress devolve null em erro de rede ou resposta ruim", async () => {
    const ok = vi.fn().mockResolvedValue({ ok: true, json: async () => [{ lat: "1.5", lon: "2.5" }] });
    await expect(geocodeAddress("Rua A", ok)).resolves.toEqual({ lat: 1.5, long: 2.5 });
    await expect(geocodeAddress("Rua A", vi.fn().mockResolvedValue({ ok: false }))).resolves.toBeNull();
    await expect(geocodeAddress("Rua A", vi.fn().mockRejectedValue(new Error("offline")))).resolves.toBeNull();
    const never = vi.fn();
    await expect(geocodeAddress("   ", never)).resolves.toBeNull();
    expect(never).not.toHaveBeenCalled();
  });

  it("needsGeocode: endereço mudou ou loja ainda em 0,0", () => {
    const est = { location_string: "Rua A, 1", lat: -23.5, long: -46.6 };
    expect(needsGeocode("Rua A, 1", est)).toBe(false);
    expect(needsGeocode("Rua B, 2", est)).toBe(true);
    expect(needsGeocode("Rua A, 1", { ...est, lat: 0, long: 0 })).toBe(true);
    expect(needsGeocode("", { location_string: "" })).toBe(false);
  });
});

describe("establishmentPayload", () => {
  it("converte os números digitados (o servidor recusava \"6\" como texto)", () => {
    const p = establishmentPayload(
      { name: " Loja ", description: "d", max_distance_delivery: "6", lat: "-23.5", long: -46.6, location_string: "Rua A", id: 7, owner_id: 3 },
      defaultHours()
    );
    expect(p).toMatchObject({ name: "Loja", max_distance_delivery: 6, lat: -23.5, long: -46.6 });
    expect(p.horarioFuncionamento).toBe("Seg a Sáb, 08:00–22:00");
    expect(p).not.toHaveProperty("id");
    expect(establishmentPayload({ max_distance_delivery: "" }).max_distance_delivery).toBe(0);
  });
});
