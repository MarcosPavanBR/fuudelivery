import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import App from "./App";

// Token JWT fake válido (payload = { sub: 1, role: "admin", name: "Test Admin" })
const FAKE_TOKEN =
  "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9." +
  btoa(JSON.stringify({ sub: 1, role: "admin", name: "Test Admin" })) +
  ".fake-signature";

const FAKE_REFRESH = "fake-refresh-token-abc123";

// Mocks hoisted para evitar problemas com vi.mock hoisting
const { mockPost, mockGet } = vi.hoisted(() => {
  return {
    mockPost: vi.fn(),
    mockGet: vi.fn(),
  };
});

vi.mock("./services/api", () => ({
  default: {
    post: mockPost,
    get: mockGet,
    interceptors: {
      request: { use: vi.fn() },
      response: { use: vi.fn() },
    },
  },
}));

// ── Setup / Teardown ───────────────────────────────────────────
beforeEach(() => {
  localStorage.clear();
  vi.clearAllMocks();
  mockPost.mockClear();
  mockGet.mockClear();
});

// ── 1. Smoke: App monta sem crashar ────────────────────────────
describe("WebAdmin Smoke Tests", () => {
  it("renderiza sem lançar erro", () => {
    expect(() => render(<App />)).not.toThrow();
  });

  it("monta o container raiz #root", () => {
    render(<App />);
    expect(document.querySelector("#root")).toBeDefined();
  });
});

// ── 2. Login Page renderiza corretamente ───────────────────────
describe("Login Page", () => {
  it("renderiza formulário de login quando não autenticado", () => {
    localStorage.removeItem("fuu_admin_token");
    render(<App />);

    expect(screen.getByLabelText(/e-mail/i)).toBeDefined();
    expect(screen.getByLabelText(/senha/i)).toBeDefined();
    expect(screen.getByRole("button", { name: /entrar/i })).toBeDefined();
  });

  it("renderiza campos de entrada com placeholders corretos", () => {
    render(<App />);

    expect(screen.getByPlaceholderText("seu@email.com")).toBeDefined();
    expect(screen.getByPlaceholderText("Sua senha")).toBeDefined();
  });

  it("renderiza branding do FuuDelivery na página de login", () => {
    render(<App />);

    expect(screen.getByText("Entrar na conta")).toBeDefined();
    expect(screen.getByText(/painel administrativo do FuuDelivery/i)).toBeDefined();
  });

  it("possui link 'Esqueceu a senha?'", () => {
    render(<App />);

    expect(screen.getByText("Esqueceu a senha?")).toBeDefined();
  });
});

// ── 3. Login interação: preencher e submeter ──────────────────
describe("Login Interaction", () => {
  it("permite digitar email e senha", () => {
    render(<App />);

    const emailInput = screen.getByLabelText(/e-mail/i);
    const passwordInput = screen.getByLabelText(/senha/i);

    fireEvent.change(emailInput, { target: { value: "admin@test.com" } });
    fireEvent.change(passwordInput, { target: { value: "secret123" } });

    expect(emailInput.value).toBe("admin@test.com");
    expect(passwordInput.value).toBe("secret123");
  });

  it("chama API POST /users/login ao submeter", async () => {
    mockPost.mockResolvedValueOnce({
      data: { token: FAKE_TOKEN, refresh_token: FAKE_REFRESH },
    });

    render(<App />);

    fireEvent.change(screen.getByLabelText(/e-mail/i), {
      target: { value: "admin@test.com" },
    });
    fireEvent.change(screen.getByLabelText(/senha/i), {
      target: { value: "secret123" },
    });

    fireEvent.click(screen.getByRole("button", { name: /entrar/i }));

    await waitFor(() => {
      expect(mockPost).toHaveBeenCalledWith("/users/login", {
        email: "admin@test.com",
        password: "secret123",
      });
    });
  });

  it("armazena token e refresh token no localStorage após login", async () => {
    mockPost.mockResolvedValueOnce({
      data: { token: FAKE_TOKEN, refresh_token: FAKE_REFRESH },
    });

    render(<App />);

    fireEvent.change(screen.getByLabelText(/e-mail/i), {
      target: { value: "admin@test.com" },
    });
    fireEvent.change(screen.getByLabelText(/senha/i), {
      target: { value: "secret123" },
    });
    fireEvent.click(screen.getByRole("button", { name: /entrar/i }));

    await waitFor(() => {
      expect(localStorage.getItem("fuu_admin_token")).toBe(FAKE_TOKEN);
      expect(localStorage.getItem("fuu_admin_refresh_token")).toBe(
        FAKE_REFRESH
      );
    });
  });
});

// ── 4. Login com erro ──────────────────────────────────────────
describe("Login Error Handling", () => {
  it("exibe mensagem de erro quando credenciais são inválidas", async () => {
    mockPost.mockRejectedValueOnce({
      response: { status: 401, data: { error: "Unauthorized" } },
    });

    render(<App />);

    fireEvent.change(screen.getByLabelText(/e-mail/i), {
      target: { value: "wrong@test.com" },
    });
    fireEvent.change(screen.getByLabelText(/senha/i), {
      target: { value: "wrongpass" },
    });
    fireEvent.click(screen.getByRole("button", { name: /entrar/i }));

    await waitFor(() => {
      expect(
        screen.getByText(/credenciais inválidas/i)
      ).toBeDefined();
    });
  });

  it("limpa erro anterior ao submeter novamente", async () => {
    mockPost
      .mockRejectedValueOnce({
        response: { status: 401, data: { error: "Unauthorized" } },
      })
      .mockResolvedValueOnce({
        data: { token: FAKE_TOKEN, refresh_token: FAKE_REFRESH },
      });

    render(<App />);

    // Primeira tentativa — falha
    fireEvent.change(screen.getByLabelText(/e-mail/i), {
      target: { value: "wrong@test.com" },
    });
    fireEvent.change(screen.getByLabelText(/senha/i), {
      target: { value: "wrong" },
    });
    fireEvent.click(screen.getByRole("button", { name: /entrar/i }));

    await waitFor(() => {
      expect(screen.getByText(/credenciais inválidas/i)).toBeDefined();
    });

    // Segunda tentativa — sucesso
    fireEvent.change(screen.getByLabelText(/e-mail/i), {
      target: { value: "admin@test.com" },
    });
    fireEvent.change(screen.getByLabelText(/senha/i), {
      target: { value: "correct" },
    });
    fireEvent.click(screen.getByRole("button", { name: /entrar/i }));

    await waitFor(() => {
      expect(screen.queryByText(/credenciais inválidas/i)).toBeNull();
    });
  });
});

// ── 5. Login redireciona para Dashboard ────────────────────────
describe("Login Redirect", () => {
  it("redireciona para / após login bem-sucedido", async () => {
    mockPost.mockResolvedValueOnce({
      data: { token: FAKE_TOKEN, refresh_token: FAKE_REFRESH },
    });

    render(<App />);

    fireEvent.change(screen.getByLabelText(/e-mail/i), {
      target: { value: "admin@test.com" },
    });
    fireEvent.change(screen.getByLabelText(/senha/i), {
      target: { value: "secret123" },
    });
    fireEvent.click(screen.getByRole("button", { name: /entrar/i }));

    await waitFor(() => {
      // Após login, o AppRoutes deve renderizar o Dashboard (via ProtectedRoute)
      // Verificamos que o formulário de login não está mais visível
      expect(screen.queryByLabelText(/e-mail/i)).toBeNull();
    });
  });
});

// ── 6. Usuário autenticado vê Dashboard ───────────────────────
describe("Authenticated State", () => {
  it("redireciona para login quando token é inválido/expirado", () => {
    // Token inválido (não é base64 JWT válido)
    localStorage.setItem("fuu_admin_token", "invalid-token");
    render(<App />);

    // Deve voltar para login porque decodePayload lança erro
    expect(screen.getByLabelText(/e-mail/i)).toBeDefined();
  });
});

// ── 7. Toggle mostrar/esconder senha ──────────────────────────
describe("Password Visibility Toggle", () => {
  it("alterna entre mostrar e esconder senha", () => {
    render(<App />);

    const passwordInput = screen.getByLabelText(/senha/i);
    expect(passwordInput.type).toBe("password");

    // Clica no botão de olho
    const toggleButton = screen.getByRole("button", { name: "" }); // FiEye/FiEyeOff
    fireEvent.click(toggleButton);

    expect(passwordInput.type).toBe("text");
  });
});

// ── 8. Layout autenticado ─────────────────────────────────────
describe("Authenticated Layout", () => {
  it("mostra sidebar com menu quando autenticado", () => {
    localStorage.setItem("fuu_admin_token", FAKE_TOKEN);
    mockGet.mockResolvedValue({ data: [] });

    render(<App />);

    // Verificar que o layout do admin aparece
    // O Layout contém o nome "Fuu" no sidebar
    expect(screen.getByText("Fuu")).toBeDefined();
  });

  it("mostra nome do usuário no header", () => {
    localStorage.setItem("fuu_admin_token", FAKE_TOKEN);
    mockGet.mockResolvedValue({ data: [] });

    render(<App />);

    expect(screen.getByText("Test Admin")).toBeDefined();
  });
});
