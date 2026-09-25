import { establishmentIdOf } from "../session";

describe("establishmentIdOf", () => {
  it("usa o establishment_id da sessão, nunca o id do usuário", () => {
    expect(establishmentIdOf({ id: 3, establishment_id: 7 })).toBe(7);
    expect(establishmentIdOf({ id: 3, establishment: { id: 9 } })).toBe(9);
    expect(establishmentIdOf({ id: 3 })).toBeNull();
    expect(establishmentIdOf({ id: 3, establishment_id: 0 })).toBeNull();
    expect(establishmentIdOf(null)).toBeNull();
  });
});
