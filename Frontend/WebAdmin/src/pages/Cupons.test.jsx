import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import Cupons from "./Cupons.jsx";

// A tela de cupons é onde a escolha "quem paga a promoção" é feita. O resto do
// sistema (orders_api grava funded_by no pedido, payment_api subtrai do lado
// certo no split) depende de esse campo sair daqui com o valor certo — mandar
// "platform" quando o admin escolheu o restaurante tira dinheiro de terceiro
// sem ninguém ver, porque a soma do split continua fechando.

const { mockPost, mockGet, mockDelete } = vi.hoisted(() => ({
  mockPost: vi.fn(),
  mockGet: vi.fn(),
  mockDelete: vi.fn(),
}));

vi.mock("../services/api", () => ({
  default: { post: mockPost, get: mockGet, delete: mockDelete },
}));

vi.mock("react-toastify", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

// Preenche o formulário com um cupom válido. Devolve o botão de submit.
async function preencherCupomValido({ codigo = "PROMO10" } = {}) {
  fireEvent.click(screen.getByRole("button", { name: /novo cupom/i }));

  fireEvent.change(screen.getByPlaceholderText("PROMO10"), {
    target: { value: codigo },
  });
  fireEvent.change(screen.getByPlaceholderText("10"), {
    target: { value: "10" },
  });
  fireEvent.change(screen.getByLabelText(/^Início$/i), {
    target: { value: "2026-01-01T00:00" },
  });
  fireEvent.change(screen.getByLabelText(/^Validade$/i), {
    target: { value: "2026-02-01T00:00" },
  });
  return screen.getByRole("button", { name: /criar cupom/i });
}

describe("Cupons — quem paga o desconto", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGet.mockResolvedValue({ data: [] });
    mockPost.mockResolvedValue({ data: {} });
    mockDelete.mockResolvedValue({ data: {} });
  });

  it("manda funded_by=platform quando o admin escolhe bancar", async () => {
    render(<Cupons />);
    await waitFor(() => expect(mockGet).toHaveBeenCalledWith("/coupons"));

    const submit = await preencherCupomValido();
    fireEvent.click(screen.getByRole("button", { name: /Eu \(plataforma\)/i }));
    fireEvent.click(submit);

    await waitFor(() => expect(mockPost).toHaveBeenCalled());
    expect(mockPost.mock.calls[0][1]).toMatchObject({
      code: "PROMO10",
      funded_by: "platform",
    });
  });

  // O caso que mexe no dinheiro de outra pessoa: se a tela mandasse "platform"
  // aqui, o split tiraria da plataforma uma promoção que o restaurante
  // aceitou bancar — e vice-versa.
  it("manda funded_by=establishment quando o admin escolhe o restaurante", async () => {
    render(<Cupons />);
    await waitFor(() => expect(mockGet).toHaveBeenCalledWith("/coupons"));

    const submit = await preencherCupomValido();
    fireEvent.click(screen.getByRole("button", { name: /O restaurante/i }));
    fireEvent.click(submit);

    await waitFor(() => expect(mockPost).toHaveBeenCalled());
    expect(mockPost.mock.calls[0][1].funded_by).toBe("establishment");
  });

  // Sem escolha explícita a tela precisa mandar algo — e esse algo é a
  // plataforma. Mandar o restaurante por omissão cobraria de quem não decidiu.
  it("o padrão é a plataforma, não o restaurante", async () => {
    render(<Cupons />);
    await waitFor(() => expect(mockGet).toHaveBeenCalledWith("/coupons"));

    const submit = await preencherCupomValido();
    fireEvent.click(submit);

    await waitFor(() => expect(mockPost).toHaveBeenCalled());
    expect(mockPost.mock.calls[0][1].funded_by).toBe("platform");
  });
});

describe("Cupons — o que a tela não deixa criar", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGet.mockResolvedValue({ data: [] });
    mockPost.mockResolvedValue({ data: {} });
  });

  it("não envia cupom sem código", async () => {
    render(<Cupons />);
    await waitFor(() => expect(mockGet).toHaveBeenCalled());

    fireEvent.click(screen.getByRole("button", { name: /novo cupom/i }));
    fireEvent.click(screen.getByRole("button", { name: /criar cupom/i }));

    await waitFor(() => expect(mockPost).not.toHaveBeenCalled());
  });

  it("não envia percentual acima de 100", async () => {
    render(<Cupons />);
    await waitFor(() => expect(mockGet).toHaveBeenCalled());

    const submit = await preencherCupomValido();
    fireEvent.change(screen.getByPlaceholderText("10"), {
      target: { value: "150" },
    });
    fireEvent.click(submit);

    await waitFor(() => expect(mockPost).not.toHaveBeenCalled());
  });

  // Validade antes do início criaria um cupom que nasce morto — e o erro só
  // apareceria para o cliente que tentasse usar.
  it("não envia validade anterior ao início", async () => {
    render(<Cupons />);
    await waitFor(() => expect(mockGet).toHaveBeenCalled());

    const submit = await preencherCupomValido();
    fireEvent.change(screen.getByLabelText(/^Validade$/i), {
      target: { value: "2025-12-01T00:00" },
    });
    fireEvent.click(submit);

    await waitFor(() => expect(mockPost).not.toHaveBeenCalled());
  });

  // FREE_DELIVERY não tem valor próprio: o desconto é o frete do pedido, que
  // só o servidor conhece. Mandar discount_value != 0 aqui seria inventar um
  // número que o backend ignora — e que a tela mostraria como se valesse.
  it("frete grátis vai com discount_value zero", async () => {
    render(<Cupons />);
    await waitFor(() => expect(mockGet).toHaveBeenCalled());

    const submit = await preencherCupomValido();
    fireEvent.click(screen.getByRole("button", { name: /Frete grátis/i }));
    fireEvent.click(submit);

    await waitFor(() => expect(mockPost).toHaveBeenCalled());
    expect(mockPost.mock.calls[0][1]).toMatchObject({
      discount_type: "FREE_DELIVERY",
      discount_value: 0,
    });
  });
});

describe("Cupons — a lista", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockPost.mockResolvedValue({ data: {} });
    mockDelete.mockResolvedValue({ data: {} });
  });

  // O backend serializa o modelo Go direto: as chaves chegam em PascalCase.
  // Ler só snake_case deixaria a tabela inteira em branco.
  it("lê os campos em PascalCase que o backend devolve", async () => {
    mockGet.mockResolvedValue({
      data: [
        {
          ID: 1,
          Code: "DALOJA",
          DiscountType: "FIXED",
          DiscountValue: 8,
          FundedBy: "establishment",
          UsedCount: 3,
          MaxUses: 10,
          IsActive: true,
          ExpiryDate: "2030-01-01T00:00:00Z",
        },
      ],
    });

    render(<Cupons />);

    expect(await screen.findByText("DALOJA")).toBeTruthy();
    expect(screen.getByText("Restaurante")).toBeTruthy();
    expect(screen.getByText("3 / 10")).toBeTruthy();
  });

  it("mostra a plataforma quando o cupom é bancado por ela", async () => {
    mockGet.mockResolvedValue({
      data: [
        {
          ID: 2,
          Code: "DAPLAT",
          DiscountType: "PERCENTAGE",
          DiscountValue: 10,
          FundedBy: "platform",
          IsActive: true,
          ExpiryDate: "2030-01-01T00:00:00Z",
        },
      ],
    });

    render(<Cupons />);

    expect(await screen.findByText("DAPLAT")).toBeTruthy();
    expect(screen.getByText("Plataforma")).toBeTruthy();
  });

  // Cupom gravado antes da migração 22 chega sem funded_by. Mostrar em branco
  // seria pior do que mostrar o padrão que o backend de fato aplica.
  it("cupom antigo sem funded_by aparece como plataforma", async () => {
    mockGet.mockResolvedValue({
      data: [
        {
          ID: 3,
          Code: "ANTIGO",
          DiscountType: "FIXED",
          DiscountValue: 5,
          IsActive: true,
          ExpiryDate: "2030-01-01T00:00:00Z",
        },
      ],
    });

    render(<Cupons />);

    expect(await screen.findByText("ANTIGO")).toBeTruthy();
    expect(screen.getByText("Plataforma")).toBeTruthy();
  });
});
