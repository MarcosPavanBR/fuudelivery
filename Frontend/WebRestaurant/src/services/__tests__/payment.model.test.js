import { describe, it, expect, vi, beforeEach } from "vitest";

vi.mock("../api", () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
  },
}));

import api from "../api";
import { requestWithdraw, newIdempotencyKey } from "../payment.model";

const mockedPost = api.post;

beforeEach(() => {
  vi.clearAllMocks();
  mockedPost.mockResolvedValue({ data: { ok: true } });
});

// O backend distingue "o dono clicou duas vezes no mesmo saque" de "o dono
// quer sacar de novo" pela Idempotency-Key. Sem chave ele cai num fallback
// derivado de (estabelecimento, valor, destino, janela de 1 min), que não
// prova serem a mesma requisição lógica e por isso responde 409 — e quem
// tomava esse 409 era o duplo clique legítimo: o saque tinha funcionado e a
// tela mostrava erro.
describe("requestWithdraw — chave de idempotência", () => {
  const saque = { amount: 50, destination: "chave-pix-do-dono", method: "PIX" };

  it("manda a chave no header e no corpo", async () => {
    await requestWithdraw({ ...saque, idempotencyKey: "chave-fixa" });

    const [url, body, config] = mockedPost.mock.calls[0];
    expect(url).toBe("/wallet/establishment/withdraw");
    expect(body.idempotency_key).toBe("chave-fixa");
    // Também no header: há proxy que descarta header desconhecido, e o
    // backend aceita as duas formas.
    expect(config.headers["Idempotency-Key"]).toBe("chave-fixa");
  });

  // O ponto que evita sacar duas vezes: a MESMA intenção reusa a chave, e o
  // backend responde 200 idempotente em vez de debitar de novo.
  it("duas chamadas da mesma intenção usam a mesma chave", async () => {
    await requestWithdraw({ ...saque, idempotencyKey: "mesma-intencao" });
    await requestWithdraw({ ...saque, idempotencyKey: "mesma-intencao" });

    expect(mockedPost.mock.calls[0][1].idempotency_key).toBe("mesma-intencao");
    expect(mockedPost.mock.calls[1][1].idempotency_key).toBe("mesma-intencao");
  });

  // E o inverso: sem chave vinda de fora, cada chamada gera a sua. É por isso
  // que quem chama TEM de segurar a chave por intenção (MinhaCarteira gera
  // uma ao abrir o modal) — gerar por requisição faria o duplo clique mandar
  // duas chaves distintas e sacar duas vezes, que é pior do que o 409.
  it("sem chave explícita, cada chamada gera a sua", async () => {
    await requestWithdraw(saque);
    await requestWithdraw(saque);

    const primeira = mockedPost.mock.calls[0][1].idempotency_key;
    const segunda = mockedPost.mock.calls[1][1].idempotency_key;
    expect(primeira).toBeTruthy();
    expect(segunda).toBeTruthy();
    expect(primeira).not.toBe(segunda);
  });

  it("o valor e o destino continuam indo no corpo", async () => {
    await requestWithdraw({ ...saque, idempotencyKey: "k" });

    expect(mockedPost.mock.calls[0][1]).toMatchObject({
      amount: 50,
      destination: "chave-pix-do-dono",
      method: "PIX",
    });
  });
});

describe("newIdempotencyKey", () => {
  it("gera chaves diferentes a cada chamada", () => {
    const chaves = new Set(
      Array.from({ length: 50 }, () => newIdempotencyKey())
    );
    expect(chaves.size).toBe(50);
  });

  // crypto.randomUUID não existe em contexto inseguro (http:// que não seja
  // localhost) nem em WebView antiga. Um saque não pode falhar por isso.
  it("funciona sem crypto.randomUUID", () => {
    // globalThis.crypto só tem getter no jsdom — daí o spy em vez da
    // atribuição direta.
    const spy = vi
      .spyOn(globalThis.crypto, "randomUUID")
      .mockImplementation(undefined);
    // eslint-disable-next-line no-undefined
    globalThis.crypto.randomUUID = undefined;
    try {
      const chave = newIdempotencyKey();
      expect(typeof chave).toBe("string");
      expect(chave.length).toBeGreaterThan(8);
      expect(chave).not.toBe(newIdempotencyKey());
    } finally {
      spy.mockRestore();
    }
  });
});
