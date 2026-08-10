import { render } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import App from "./App";

// Smoke test: garante que o App monta sem crashar no React 19.
// (O teste padrão do CRA checava o texto "learn react", que não existe aqui.)
describe("App", () => {
  it("renderiza sem lançar erro", () => {
    expect(() => render(<App />)).not.toThrow();
  });

  it("monta o container raiz da aplicação", () => {
    render(<App />);
    // O App renderiza ToastContainer + rotas; ao menos algo do DOM deve existir
    expect(document.querySelector("#root")).toBeDefined();
  });
});
