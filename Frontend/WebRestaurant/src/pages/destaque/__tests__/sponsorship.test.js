import { canCancel, dayRange, firstFullDay, shortDay, totalFor, weekdayShort } from "../sponsorship";

describe("destaque", () => {
  it("dayRange atravessa o mês", () => {
    expect(dayRange("2026-09-29", 3)).toEqual(["2026-09-29", "2026-09-30", "2026-10-01"]);
    expect(dayRange("x", 3)).toEqual([]);
  });

  it("firstFullDay acha o primeiro dia sem vaga do período", () => {
    const cal = [
      { day: "2026-09-26", free: 2 },
      { day: "2026-09-27", free: 0 },
      { day: "2026-09-28", free: 1 },
    ];
    expect(firstFullDay(cal, "2026-09-26", 1)).toBeNull();
    expect(firstFullDay(cal, "2026-09-26", 3)).toBe("2026-09-27");
  });

  it("totalFor arredonda centavos", () => {
    expect(totalFor(15, 3)).toBe(45);
    expect(totalFor(12.345, 1)).toBe(12.35);
  });

  it("formata dia", () => {
    expect(shortDay("2026-09-26")).toBe("26/09");
    expect(weekdayShort("2026-09-26")).toBe("sáb");
  });

  it("canCancel só antes de começar", () => {
    expect(canCancel({ status: "active", start_day: "2026-09-27" }, "2026-09-26")).toBe(true);
    expect(canCancel({ status: "active", start_day: "2026-09-26" }, "2026-09-26")).toBe(false);
    expect(canCancel({ status: "cancelled", start_day: "2026-09-30" }, "2026-09-26")).toBe(false);
  });
});
