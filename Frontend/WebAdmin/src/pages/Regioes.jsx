import React, { useCallback, useEffect, useState } from "react";
import {
  FiMapPin,
  FiPlus,
  FiTrash2,
  FiLoader,
  FiAlertCircle,
  FiEdit2,
  FiX,
} from "react-icons/fi";
import api from "../services/api";
import { toast } from "react-toastify";

const formaVazia = {
  id: null,
  name: "",
  cep_start: "",
  cep_end: "",
  city: "",
  uf: "",
  fee: "",
  priority: 100,
  active: true,
};

const brl = (v) =>
  Number(v || 0).toLocaleString("pt-BR", { style: "currency", currency: "BRL" });

// Exibe 8 dígitos como 01310-100. O backend guarda inteiro; a máscara é só
// para leitura humana.
export function formatCep(valor) {
  const d = String(valor ?? "").replace(/\D/g, "").padStart(8, "0");
  if (d.length !== 8) return String(valor ?? "");
  return `${d.slice(0, 5)}-${d.slice(5)}`;
}

export default function Regioes() {
  const [regioes, setRegioes] = useState([]);
  const [carregando, setCarregando] = useState(true);
  const [salvando, setSalvando] = useState(false);
  const [form, setForm] = useState(formaVazia);
  const [mostrarForm, setMostrarForm] = useState(false);

  const carregar = useCallback(async () => {
    setCarregando(true);
    try {
      const { data } = await api.get("/delivery/regions");
      setRegioes(Array.isArray(data) ? data : []);
    } catch (err) {
      toast.error(err.response?.data?.error || "Erro ao carregar regiões");
      setRegioes([]);
    } finally {
      setCarregando(false);
    }
  }, []);

  useEffect(() => {
    carregar();
  }, [carregar]);

  const alterar = (campo) => (e) =>
    setForm((f) => ({ ...f, [campo]: e.target.value }));

  const abrirNova = () => {
    setForm(formaVazia);
    setMostrarForm(true);
  };

  const editar = (r) => {
    setForm({
      id: r.id,
      name: r.name ?? "",
      cep_start: formatCep(r.cep_start),
      cep_end: formatCep(r.cep_end),
      city: r.city ?? "",
      uf: r.uf ?? "",
      fee: String(r.fee ?? ""),
      priority: r.priority ?? 100,
      active: r.active !== false,
    });
    setMostrarForm(true);
  };

  const enviar = async (e) => {
    e.preventDefault();

    if (!form.name.trim()) {
      toast.error("Informe o nome da região");
      return;
    }
    const inicio = form.cep_start.replace(/\D/g, "");
    const fim = form.cep_end.replace(/\D/g, "");
    if (inicio.length !== 8 || fim.length !== 8) {
      toast.error("CEP inicial e final precisam ter 8 dígitos");
      return;
    }
    if (Number(inicio) > Number(fim)) {
      toast.error("O CEP inicial tem de ser menor ou igual ao final");
      return;
    }
    if (Number(form.fee) < 0) {
      toast.error("O valor do frete não pode ser negativo");
      return;
    }

    const corpo = {
      name: form.name.trim(),
      cep_start: inicio,
      cep_end: fim,
      city: form.city.trim(),
      uf: form.uf.trim().toUpperCase(),
      fee: Number(form.fee || 0),
      priority: Number(form.priority || 100),
      active: form.active,
    };

    setSalvando(true);
    try {
      if (form.id) {
        await api.put(`/delivery/regions/${form.id}`, corpo);
        toast.success(`Região ${corpo.name} atualizada`);
      } else {
        await api.post("/delivery/regions", corpo);
        toast.success(`Região ${corpo.name} criada`);
      }
      setForm(formaVazia);
      setMostrarForm(false);
      carregar();
    } catch (err) {
      toast.error(err.response?.data?.error || "Erro ao salvar região");
    } finally {
      setSalvando(false);
    }
  };

  const remover = async (r) => {
    if (
      !window.confirm(
        `Remover a região ${r.name}? Pedidos novos com CEP dessa faixa passam a cair na taxa por km do restaurante.`
      )
    ) {
      return;
    }
    try {
      await api.delete(`/delivery/regions/${r.id}`);
      toast.success("Região removida");
      carregar();
    } catch (err) {
      toast.error(err.response?.data?.error || "Erro ao remover região");
    }
  };

  return (
    <div className="max-w-6xl mx-auto">
      <div className="mb-8 flex items-start justify-between gap-4 flex-wrap">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-xl flex items-center justify-center bg-gradient-to-br from-fuu-red to-fuu-red-dark">
            <FiMapPin className="h-5 w-5 text-white" />
          </div>
          <div>
            <h1 className="text-2xl font-bold text-gray-900">Regiões de entrega</h1>
            <p className="text-sm text-gray-500">
              O preço do frete por faixa de CEP
            </p>
          </div>
        </div>
        <button
          type="button"
          onClick={mostrarForm ? () => setMostrarForm(false) : abrirNova}
          className="inline-flex items-center gap-2 rounded-lg bg-fuu-red px-4 py-2 text-sm font-medium text-white hover:bg-fuu-red-dark"
        >
          {mostrarForm ? <FiX className="h-4 w-4" /> : <FiPlus className="h-4 w-4" />}
          {mostrarForm ? "Fechar" : "Nova região"}
        </button>
      </div>

      {/* Como o preço é decidido. Sem isto, o admin não tem como saber por que
          um pedido específico saiu com um valor e não outro. */}
      <div className="mb-6 flex items-start gap-2 rounded-lg border border-gray-200 bg-gray-50 p-4 text-xs text-gray-600">
        <FiAlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
        <div>
          <p className="mb-1">
            O frete sai da região que contém o <strong>CEP do endereço de
            entrega</strong>. Quando duas faixas se sobrepõem, ganha a de menor
            prioridade; empatando, ganha a <strong>mais estreita</strong> — dá
            para cadastrar a cidade toda e depois recortar um bairro caro sem
            reordenar nada.
          </p>
          <p>
            CEP fora de todas as faixas cai na taxa por km do restaurante.
            Nunca sai frete grátis por falta de cadastro.
          </p>
        </div>
      </div>

      {mostrarForm && (
        <form
          onSubmit={enviar}
          className="mb-8 rounded-xl border border-gray-200 bg-white p-6 shadow-sm"
        >
          <div className="grid gap-4 sm:grid-cols-2">
            <div>
              <label htmlFor="reg-nome" className="block text-sm font-medium text-gray-700 mb-1">
                Nome da região
              </label>
              <input
                id="reg-nome"
                value={form.name}
                onChange={alterar("name")}
                placeholder="Centro"
                className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-fuu-red focus:outline-none"
              />
            </div>

            <div>
              <label htmlFor="reg-fee" className="block text-sm font-medium text-gray-700 mb-1">
                Frete (R$)
              </label>
              <input
                id="reg-fee"
                type="number"
                step="0.01"
                min="0"
                value={form.fee}
                onChange={alterar("fee")}
                placeholder="6.00"
                className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-fuu-red focus:outline-none"
              />
            </div>

            <div>
              <label htmlFor="reg-cep-inicio" className="block text-sm font-medium text-gray-700 mb-1">
                CEP inicial
              </label>
              <input
                id="reg-cep-inicio"
                value={form.cep_start}
                onChange={alterar("cep_start")}
                placeholder="01000-000"
                className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-fuu-red focus:outline-none"
              />
            </div>

            <div>
              <label htmlFor="reg-cep-fim" className="block text-sm font-medium text-gray-700 mb-1">
                CEP final
              </label>
              <input
                id="reg-cep-fim"
                value={form.cep_end}
                onChange={alterar("cep_end")}
                placeholder="01999-999"
                className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-fuu-red focus:outline-none"
              />
              <p className="mt-1 text-xs text-gray-500">
                As duas pontas entram na faixa.
              </p>
            </div>

            <div>
              <label htmlFor="reg-cidade" className="block text-sm font-medium text-gray-700 mb-1">
                Cidade
              </label>
              <input
                id="reg-cidade"
                value={form.city}
                onChange={alterar("city")}
                placeholder="deixe vazio para qualquer cidade"
                className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-fuu-red focus:outline-none"
              />
            </div>

            <div>
              <label htmlFor="reg-uf" className="block text-sm font-medium text-gray-700 mb-1">
                UF
              </label>
              <input
                id="reg-uf"
                value={form.uf}
                onChange={alterar("uf")}
                maxLength={2}
                placeholder="vazio = qualquer"
                className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm uppercase focus:border-fuu-red focus:outline-none"
              />
            </div>

            <div>
              <label htmlFor="reg-prioridade" className="block text-sm font-medium text-gray-700 mb-1">
                Prioridade
              </label>
              <input
                id="reg-prioridade"
                type="number"
                min="1"
                value={form.priority}
                onChange={alterar("priority")}
                className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-fuu-red focus:outline-none"
              />
              <p className="mt-1 text-xs text-gray-500">
                Menor ganha. Deixe 100 se não precisar forçar.
              </p>
            </div>

            <div className="flex items-end">
              <label className="flex items-center gap-2 text-sm text-gray-700">
                <input
                  type="checkbox"
                  checked={form.active}
                  onChange={(e) =>
                    setForm((f) => ({ ...f, active: e.target.checked }))
                  }
                  className="h-4 w-4 rounded border-gray-300"
                />
                Ativa
              </label>
            </div>
          </div>

          <div className="mt-6 flex justify-end gap-3">
            <button
              type="button"
              onClick={() => {
                setForm(formaVazia);
                setMostrarForm(false);
              }}
              className="rounded-lg border border-gray-300 px-4 py-2 text-sm text-gray-700 hover:bg-gray-50"
            >
              Cancelar
            </button>
            <button
              type="submit"
              disabled={salvando}
              className="inline-flex items-center gap-2 rounded-lg bg-fuu-red px-5 py-2 text-sm font-medium text-white hover:bg-fuu-red-dark disabled:opacity-60"
            >
              {salvando && <FiLoader className="h-4 w-4 animate-spin" />}
              {form.id ? "Salvar alterações" : "Criar região"}
            </button>
          </div>
        </form>
      )}

      <div className="overflow-x-auto rounded-xl border border-gray-200 bg-white shadow-sm">
        <table className="min-w-full divide-y divide-gray-200 text-sm">
          <thead className="bg-gray-50">
            <tr>
              {["Região", "Faixa de CEP", "Cidade/UF", "Frete", "Prioridade", "Situação", ""].map(
                (h) => (
                  <th
                    key={h}
                    className="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-gray-500"
                  >
                    {h}
                  </th>
                )
              )}
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
            {carregando && (
              <tr>
                <td colSpan={7} className="px-4 py-10 text-center text-gray-400">
                  <FiLoader className="mx-auto h-5 w-5 animate-spin" />
                </td>
              </tr>
            )}

            {!carregando && regioes.length === 0 && (
              <tr>
                <td colSpan={7} className="px-4 py-10 text-center text-gray-500">
                  Nenhuma região cadastrada — todo pedido está caindo na taxa por
                  km do restaurante. Crie a primeira em "Nova região".
                </td>
              </tr>
            )}

            {!carregando &&
              regioes.map((r) => (
                <tr key={r.id} className="hover:bg-gray-50">
                  <td className="px-4 py-3 font-medium text-gray-900">{r.name}</td>
                  <td className="px-4 py-3 font-mono text-gray-700">
                    {formatCep(r.cep_start)} — {formatCep(r.cep_end)}
                  </td>
                  <td className="px-4 py-3 text-gray-700">
                    {r.city || r.uf ? `${r.city || "—"}${r.uf ? ` / ${r.uf}` : ""}` : "qualquer"}
                  </td>
                  <td className="px-4 py-3 font-medium text-gray-900">{brl(r.fee)}</td>
                  <td className="px-4 py-3 text-gray-700">{r.priority}</td>
                  <td className="px-4 py-3">
                    {r.active !== false ? (
                      <span className="rounded-full bg-green-100 px-2 py-0.5 text-xs text-green-700">
                        Ativa
                      </span>
                    ) : (
                      <span className="rounded-full bg-gray-100 px-2 py-0.5 text-xs text-gray-600">
                        Inativa
                      </span>
                    )}
                  </td>
                  <td className="px-4 py-3 text-right">
                    <button
                      type="button"
                      onClick={() => editar(r)}
                      aria-label={`Editar ${r.name}`}
                      className="rounded-lg p-2 text-gray-400 hover:bg-gray-100 hover:text-gray-700"
                    >
                      <FiEdit2 className="h-4 w-4" />
                    </button>
                    <button
                      type="button"
                      onClick={() => remover(r)}
                      aria-label={`Remover ${r.name}`}
                      className="rounded-lg p-2 text-gray-400 hover:bg-red-50 hover:text-fuu-red"
                    >
                      <FiTrash2 className="h-4 w-4" />
                    </button>
                  </td>
                </tr>
              ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
