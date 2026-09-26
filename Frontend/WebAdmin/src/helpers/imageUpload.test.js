import { describe, it, expect } from "vitest";
import { checkImageFile, fitSize } from "./imageUpload";

describe("imagem", () => {
  it("fitSize mantém a proporção e não amplia", () => {
    expect(fitSize(4000, 3000, 1200)).toEqual({ width: 1200, height: 900 });
    expect(fitSize(3000, 4000, 512)).toEqual({ width: 384, height: 512 });
    expect(fitSize(300, 200, 1200)).toEqual({ width: 300, height: 200 });
  });

  it("checkImageFile aceita foto grande de celular e barra SVG", () => {
    expect(checkImageFile({ type: "image/jpeg", size: 8 * 1024 * 1024 })).toBeNull();
    expect(checkImageFile({ type: "image/svg+xml", size: 100 })).toMatch(/SVG/);
    expect(checkImageFile({ type: "application/pdf", size: 100 })).toMatch(/imagem/);
    expect(checkImageFile(null)).toMatch(/Nenhum/);
  });
});
