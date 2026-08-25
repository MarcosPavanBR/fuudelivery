import helpers from "../helpers";

describe("helpers.formatCurrency", () => {
  it("formata valores em BRL", () => {
    expect(helpers.formatCurrency(10)).toContain("10,00");
    expect(helpers.formatCurrency(1234.56)).toContain("1.234,56");
  });

  it("formata zero e negativos sem quebrar", () => {
    expect(helpers.formatCurrency(0)).toContain("0,00");
    const neg = helpers.formatCurrency(-5);
    expect(neg).toContain("5,00");
    expect(neg).toContain("-");
  });
});

describe("helpers.genCode", () => {
  it("é determinístico (mesma entrada → mesmo código)", () => {
    const a = helpers.genCode("68a1f0c9e4b0d1234567890a", 1);
    const b = helpers.genCode("68a1f0c9e4b0d1234567890a", 1);
    expect(a).toBe(b);
  });

  it("retorna código de até 4 dígitos", () => {
    const code = helpers.genCode("abc123def456", 1);
    expect(code).toMatch(/^\d{1,4}$/);
  });

  it("retorna 0000 quando não há dígitos", () => {
    expect(helpers.genCode("abcdef", 1)).toBe("0000");
  });
});

describe("helpers.haversineDistancia", () => {
  it("distância entre Av. Paulista e MASP (~1km) é plausível", () => {
    // Pontos próximos em São Paulo
    const d = helpers.haversineDistancia(
      -23.561414, // Paulista
      -46.655881,
      -23.562862, // MASP
      -46.654851
    );
    expect(d).toBeGreaterThan(0.05);
    expect(d).toBeLessThan(1);
  });

  it("distância SP → RJ fica na faixa esperada (~360-400km)", () => {
    const d = helpers.haversineDistancia(-23.5505, -46.6333, -22.9068, -43.1729);
    expect(d).toBeGreaterThan(300);
    expect(d).toBeLessThan(450);
  });
});

describe("helpers.generateId", () => {
  it("gera ids únicos em sequência", () => {
    const ids = new Set(Array.from({ length: 200 }, () => helpers.generateId(15)));
    expect(ids.size).toBe(200);
  });

  it("respeita o tamanho solicitado", () => {
    expect(helpers.generateId(15).length).toBe(15);
  });
});

describe("helpers.formatPhoneNumber", () => {
  it("mascara celular com DDD", () => {
    expect(helpers.formatPhoneNumber("11999998888")).toBe("(11) 99999-8888");
  });
});
