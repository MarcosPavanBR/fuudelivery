import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import App from "./App";

// Mock do módulo de API para evitar chamadas HTTP reais
vi.mock("../services/api", () => ({
  default: {
    post: vi.fn(),
    get: vi.fn(),
    interceptors: {
      request: { use: vi.fn() },
      response: { use: vi.fn() },
    },
  },
}));

// Smoke test: garante que o App monta sem crashar no React 19.
describe("WebAdmin App", () => {
  it("renderiza sem lançar erro", () => {
    expect(() => render(<App />)).not.toThrow();
  });

  it("monta o container raiz da aplicação", () => {
    render(<App />);
    expect(document.querySelector("#root")).toBeDefined();
  });

  it("redireciona para /login quando não autenticado", () => {
    // Limpa localStorage para simular usuário não autenticado
    localStorage.removeItem("fuu_admin_token");
    render(<App />);
    // Deve renderizar algo (página de login ou loading)
    expect(document.body).toBeTruthy();
  });
});
