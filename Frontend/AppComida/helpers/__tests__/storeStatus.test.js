import { closedLabel, isStoreOpen, ratingLabel } from "../storeStatus";

describe("vitrine", () => {
  const sexta = new Date(2026, 8, 25, 12, 0); // sexta-feira

  it("aberta e servidor antigo não mostram 'Fechado'", () => {
    expect(closedLabel({ is_open: true }, sexta)).toBeNull();
    expect(closedLabel({}, sexta)).toBeNull();
    expect(isStoreOpen({})).toBe(true);
  });

  it("fechada diz quando abre", () => {
    expect(closedLabel({ is_open: false, opens_at: "18:00" }, sexta)).toBe("Fechado · abre às 18:00");
    expect(closedLabel({ is_open: false, opens_at: "10:00", opens_day: 1 }, sexta)).toBe("Fechado · abre amanhã às 10:00");
    expect(closedLabel({ is_open: false, opens_at: "10:00", opens_day: 3 }, sexta)).toBe("Fechado · abre seg às 10:00");
    expect(closedLabel({ is_open: false }, sexta)).toBe("Fechado");
  });

  it("nota", () => {
    expect(ratingLabel({ rating: 4.63, reviews_count: 23 })).toBe("★ 4,6 (23)");
    expect(ratingLabel({ rating: 0, reviews_count: 0 })).toBe("Novo");
  });
});
