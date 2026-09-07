import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import App from "./App";

// ── Mocks ──────────────────────────────────────────────────────
// Mock completo da API — vi.hoisted garante que as variaveis existem quando o vi.mock (hoisted) roda.
const { mockPost, mockGet } = vi.hoisted(() => ({
  mockPost: vi.fn(),
  mockGet: vi.fn(),
}));

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

// Sessão agora vive num cookie HttpOnly — o "usuário logado" é sempre o
// que o backend devolve em /auth/session (GET pra restaurar, POST no
// login), nunca um token decodificado no cliente.
const FAKE_USER = { id: 1, role: "admin", name: "Test Admin", establishment_name: "Restaurante Teste" };

// ── Setup / Teardown ───────────────────────────────────────────
beforeEach(() => {
  vi.clearAllMocks();
  // Por padrão, ninguém está logado: GET /auth/session (checagem de sessão
  // no mount) responde 401. Testes que precisam de sessão ativa sobrescrevem.
  mockGet.mockImplementation((url) => {
    if (url === "/auth/session") {
      return Promise.reject({ response: { status: 401 } });
    }
    return Promise.resolve({ data: [] });
  });
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
  it("renderiza formulário de login quando não autenticado", async () => {
    render(<App />);

    await waitFor(() => {
      expect(screen.getByLabelText(/e-mail/i)).toBeDefined();
    });
    expect(screen.getByLabelText(/senha/i)).toBeDefined();
    expect(screen.getByRole("button", { name: /entrar/i })).toBeDefined();
  });

  it("renderiza campos de entrada com placeholders corretos", async () => {
    render(<App />);

    await waitFor(() => {
      expect(screen.getByPlaceholderText("seu@email.com")).toBeDefined();
    });
    expect(screen.getByPlaceholderText("Sua senha")).toBeDefined();
  });

  it("renderiza branding do FuuDelivery na página de login", async () => {
    render(<App />);

    await waitFor(() => {
      expect(screen.getByText("Entrar na conta")).toBeDefined();
    });
    expect(screen.getByText(/painel administrativo do FuuDelivery/i)).toBeDefined();
  });

  it("possui link 'Esqueceu a senha?'", async () => {
    render(<App />);

    await waitFor(() => {
      expect(screen.getByText("Esqueceu a senha?")).toBeDefined();
    });
  });
});

// ── 3. Login interação: preencher e submeter ──────────────────
describe("Login Interaction", () => {
  it("permite digitar email e senha", async () => {
    render(<App />);
    await waitFor(() => screen.getByLabelText(/e-mail/i));

    const emailInput = screen.getByLabelText(/e-mail/i);
    const passwordInput = screen.getByLabelText(/senha/i);

    fireEvent.change(emailInput, { target: { value: "admin@test.com" } });
    fireEvent.change(passwordInput, { target: { value: "secret123" } });

    expect(emailInput.value).toBe("admin@test.com");
    expect(passwordInput.value).toBe("secret123");
  });

  it("chama API POST /auth/session ao submeter", async () => {
    mockPost.mockResolvedValueOnce({ data: { user: FAKE_USER } });

    render(<App />);
    await waitFor(() => screen.getByLabelText(/e-mail/i));

    fireEvent.change(screen.getByLabelText(/e-mail/i), {
      target: { value: "admin@test.com" },
    });
    fireEvent.change(screen.getByLabelText(/senha/i), {
      target: { value: "secret123" },
    });

    fireEvent.click(screen.getByRole("button", { name: /entrar/i }));

    await waitFor(() => {
      expect(mockPost).toHaveBeenCalledWith("/auth/session", {
        email: "admin@test.com",
        password: "secret123",
      });
    });
  });

  it("mantém o usuário autenticado (sessão via cookie) após login", async () => {
    mockPost.mockResolvedValueOnce({ data: { user: FAKE_USER } });

    render(<App />);
    await waitFor(() => screen.getByLabelText(/e-mail/i));

    fireEvent.change(screen.getByLabelText(/e-mail/i), {
      target: { value: "admin@test.com" },
    });
    fireEvent.change(screen.getByLabelText(/senha/i), {
      target: { value: "secret123" },
    });
    fireEvent.click(screen.getByRole("button", { name: /entrar/i }));

    // Não há mais token pra guardar — a prova de que o login funcionou é
    // a UI sair do formulário de login (ver "Login Redirect" abaixo) e o
    // nome do usuário aparecer, vindo direto da resposta do POST.
    await waitFor(() => {
      expect(screen.getByText("Test Admin")).toBeDefined();
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
    await waitFor(() => screen.getByLabelText(/e-mail/i));

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
      .mockResolvedValueOnce({ data: { user: FAKE_USER } });

    render(<App />);
    await waitFor(() => screen.getByLabelText(/e-mail/i));

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
    mockPost.mockResolvedValueOnce({ data: { user: FAKE_USER } });

    render(<App />);
    await waitFor(() => screen.getByLabelText(/e-mail/i));

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
  it("redireciona para login quando a sessão é inválida/expirada", async () => {
    // GET /auth/session rejeitando (cookie ausente ou expirado) — é o
    // mock padrão do beforeEach, mantido explícito aqui pela clareza.
    mockGet.mockImplementation((url) => {
      if (url === "/auth/session") {
        return Promise.reject({ response: { status: 401 } });
      }
      return Promise.resolve({ data: [] });
    });

    render(<App />);

    await waitFor(() => {
      expect(screen.getByLabelText(/e-mail/i)).toBeDefined();
    });
  });
});

// ── 7. Toggle mostrar/esconder senha ──────────────────────────
describe("Password Visibility Toggle", () => {
  it("alterna entre mostrar e esconder senha", async () => {
    render(<App />);
    await waitFor(() => screen.getByLabelText(/senha/i));

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
  beforeEach(() => {
    // Sessão já ativa (cookie válido): GET /auth/session devolve o usuário;
    // qualquer outra chamada GET do Dashboard/Layout cai no fallback [].
    mockGet.mockImplementation((url) => {
      if (url === "/auth/session") {
        return Promise.resolve({ data: { user: FAKE_USER } });
      }
      return Promise.resolve({ data: [] });
    });
  });

  it("mostra sidebar com menu quando autenticado", async () => {
    render(<App />);

    await waitFor(() => {
      expect(screen.getByText("Fuu")).toBeDefined();
    });
  });

  it("mostra nome do usuário no header", async () => {
    render(<App />);

    await waitFor(() => {
      expect(screen.getByText("Test Admin")).toBeDefined();
    });
  });
});
