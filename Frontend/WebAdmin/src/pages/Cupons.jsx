import React, { useCallback, useEffect, useState } from "react";
import {
  FiTag,
  FiPlus,
  FiTrash2,
  FiLoader,
  FiPercent,
  FiDollarSign,
  FiTruck,
  FiBriefcase,
  FiHome,
  FiAlertCircle,
} from "react-icons/fi";
import api from "../services/api";
import { toast } from "react-toastify";

// Os três tipos que o backend conhece (orders_api/handlers/coupon.go).
const discountTypes = [
  {
    value: "PERCENTAGE",
    label: "Percentual",
    icon: FiPercent,
    hint: "% sobre o valor dos produtos (o frete não entra na conta)",
    unidade: "%",
  },
  {
    value: "FIXED",
    label: "Valor fixo",
    icon: FiDollarSign,
    hint: "R$ abatidos dos produtos; num pedido menor, o desconto para no subtotal",
    unidade: "R$",
  },
  {
    value: "FREE_DELIVERY",
    label: "Frete grátis",
    icon: FiTruck,
    hint: "Desconta exatamente o frete do pedido",
    unidade: "",
  },
];

// Quem absorve o desconto. É a escolha que o split lê para subtrair do lado
// certo — sem ela a promoção sairia rateada entre plataforma e restaurante.
const fundedByOptions = [
  {
    value: "platform",
    label: "Eu (plataforma)",
    icon: FiBriefcase,
    hint: "O desconto sai da taxa da plataforma. O restaurante recebe como se não houvesse cupom.",
  },
  {
    value: "establishment",
    label: "O restaurante",
    icon: FiHome,
    hint: "O desconto sai do repasse do restaurante. A taxa da plataforma não muda. Combine antes com ele.",
  },
];

const formaVazia = {
  code: "",
  description: "",
  discount_type: "PERCENTAGE",
  discount_value: "",
  min_order_value: "",
  max_uses: "",
  max_uses_per_user: "",
  establishment_id: "",
  funded_by: "platform",
  start_date: "",
  expiry_date: "",
};

const brl = (v) =>
  Number(v || 0).toLocaleString("pt-BR", { style: "currency", currency: "BRL" });

function dataCurta(iso) {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "—";
  return d.toLocaleDateString("pt-BR");
}

export default function Cupons() {
  const [cupons, setCupons] = useState([]);
  const [carregando, setCarregando] = useState(true);
  const [salvando, setSalvando] = useState(false);
  const [form, setForm] = useState(formaVazia);
  const [mostrarForm, setMostrarForm] = useState(false);

  const carregar = useCallback(async () => {
    setCarregando(true);
    try {
      const { data } = await api.get("/coupons");
      setCupons(Array.isArray(data) ? data : []);
    } catch (err) {
      toast.error(err.response?.data?.error || "Erro ao carregar cupons");
      setCupons([]);
    } finally {
      setCarregando(false);
    }
  }, []);

  useEffect(() => {
    carregar();
  }, [carregar]);

  const alterar = (campo) => (e) =>
    setForm((f) => ({ ...f, [campo]: e.target.value }));

  const tipoAtual = discountTypes.find((t) => t.value === form.discount_type);

  const enviar = async (e) => {
    e.preventDefault();

    const code = form.code.trim().toUpperCase();
    if (!code) {
      toast.error("Informe o código do cupom");
      return;
    }
    if (!form.start_date || !form.expiry_date) {
      toast.error("Informe início e validade");
      return;
    }
    if (new Date(form.expiry_date) <= new Date(form.start_date)) {
      toast.error("A validade tem de ser depois do início");
      return;
    }
    // FREE_DELIVERY não usa valor: o desconto é o frete do pedido.
    if (form.discount_type !== "FREE_DELIVERY" && Number(form.discount_value) <= 0) {
      toast.error("O valor do desconto tem de ser maior que zero");
      return;
    }
    if (form.discount_type === "PERCENTAGE" && Number(form.discount_value) > 100) {
      toast.error("Percentual não pode passar de 100");
      return;
    }

    setSalvando(true);
    try {
      await api.post("/coupons", {
        code,
        description: form.description.trim(),
        discount_type: form.discount_type,
        discount_value:
          form.discount_type === "FREE_DELIVERY" ? 0 : Number(form.discount_value),
        min_order_value: Number(form.min_order_value || 0),
        max_uses: Number(form.max_uses || 0),
        max_uses_per_user: Number(form.max_uses_per_user || 0),
        establishment_id: Number(form.establishment_id || 0),
        funded_by: form.funded_by,
        // O backend espera RFC3339; o input datetime-local devolve sem fuso.
        start_date: new Date(form.start_date).toISOString(),
        expiry_date: new Date(form.expiry_date).toISOString(),
      });
      toast.success(`Cupom ${code} criado`);
      setForm(formaVazia);
      setMostrarForm(false);
      carregar();
    } catch (err) {
      toast.error(err.response?.data?.error || "Erro ao criar cupom");
    } finally {
      setSalvando(false);
    }
  };

  const desativar = async (cupom) => {
    if (
      !window.confirm(
        `Desativar o cupom ${cupom.Code || cupom.code}? Quem já usou não é afetado; ele apenas para de valer em pedidos novos.`
      )
    ) {
      return;
    }
    try {
      await api.delete(`/coupons/${cupom.ID ?? cupom.id}`);
      toast.success("Cupom desativado");
      carregar();
    } catch (err) {
      toast.error(err.response?.data?.error || "Erro ao desativar cupom");
    }
  };

  return (
    <div className="max-w-6xl mx-auto">
      <div className="mb-8 flex items-start justify-between gap-4 flex-wrap">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-xl flex items-center justify-center bg-gradient-to-br from-fuu-red to-fuu-red-dark">
            <FiTag className="h-5 w-5 text-white" />
          </div>
          <div>
            <h1 className="text-2xl font-bold text-gray-900">Cupons</h1>
            <p className="text-sm text-gray-500">
              Promoções e descontos — e de quem sai a conta
            </p>
          </div>
        </div>
        <button
          type="button"
          onClick={() => setMostrarForm((v) => !v)}
          className="inline-flex items-center gap-2 rounded-lg bg-fuu-red px-4 py-2 text-sm font-medium text-white hover:bg-fuu-red-dark"
        >
          <FiPlus className="h-4 w-4" />
          {mostrarForm ? "Fechar" : "Novo cupom"}
        </button>
      </div>

      {mostrarForm && (
        <form
          onSubmit={enviar}
          className="mb-8 rounded-xl border border-gray-200 bg-white p-6 shadow-sm"
        >
          <div className="grid gap-4 sm:grid-cols-2">
            <div>
              <label htmlFor="cupom-code" className="block text-sm font-medium text-gray-700 mb-1">
                Código
              </label>
              <input
                id="cupom-code"
                value={form.code}
                onChange={alterar("code")}
                placeholder="PROMO10"
                className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm uppercase focus:border-fuu-red focus:outline-none"
              />
              <p className="mt-1 text-xs text-gray-500">
                É o que o cliente digita. Vira maiúsculo automaticamente.
              </p>
            </div>

            <div>
              <label htmlFor="cupom-descricao" className="block text-sm font-medium text-gray-700 mb-1">
                Descrição
              </label>
              <input
                id="cupom-descricao"
                value={form.description}
                onChange={alterar("description")}
                placeholder="Promoção de lançamento"
                className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-fuu-red focus:outline-none"
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Tipo de desconto
              </label>
              <div className="grid grid-cols-3 gap-2">
                {discountTypes.map((t) => {
                  const Icon = t.icon;
                  const ativo = form.discount_type === t.value;
                  return (
                    <button
                      key={t.value}
                      type="button"
                      onClick={() =>
                        setForm((f) => ({ ...f, discount_type: t.value }))
                      }
                      className={`flex flex-col items-center gap-1 rounded-lg border px-2 py-3 text-xs ${
                        ativo
                          ? "border-fuu-red bg-fuu-red/5 text-fuu-red"
                          : "border-gray-300 text-gray-600 hover:border-gray-400"
                      }`}
                    >
                      <Icon className="h-4 w-4" />
                      {t.label}
                    </button>
                  );
                })}
              </div>
              <p className="mt-1 text-xs text-gray-500">{tipoAtual?.hint}</p>
            </div>

            <div>
              <label htmlFor="cupom-valor" className="block text-sm font-medium text-gray-700 mb-1">
                Valor do desconto {tipoAtual?.unidade && `(${tipoAtual.unidade})`}
              </label>
              <input
                type="number"
                step="0.01"
                min="0"
                id="cupom-valor"
                value={form.discount_value}
                onChange={alterar("discount_value")}
                disabled={form.discount_type === "FREE_DELIVERY"}
                placeholder={form.discount_type === "FREE_DELIVERY" ? "o frete do pedido" : "10"}
                className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-fuu-red focus:outline-none disabled:bg-gray-100 disabled:text-gray-400"
              />
            </div>
          </div>

          {/* Quem banca — o campo que o resto do sistema usa para decidir de
              qual lado o desconto sai. Fica em destaque de propósito: escolher
              "o restaurante" mexe no dinheiro de outra pessoa. */}
          <div className="mt-6 rounded-lg border border-gray-200 bg-gray-50 p-4">
            <label className="block text-sm font-medium text-gray-700 mb-2">
              Quem paga o desconto
            </label>
            <div className="grid gap-3 sm:grid-cols-2">
              {fundedByOptions.map((o) => {
                const Icon = o.icon;
                const ativo = form.funded_by === o.value;
                return (
                  <button
                    key={o.value}
                    type="button"
                    // O nome acessível precisa ser só o rótulo: sem isto ele
                    // vira o rótulo colado na explicação, e as duas opções
                    // passam a "conter" o nome uma da outra (a da plataforma
                    // menciona o restaurante e vice-versa).
                    aria-label={o.label}
                    aria-pressed={ativo}
                    onClick={() => setForm((f) => ({ ...f, funded_by: o.value }))}
                    className={`flex items-start gap-3 rounded-lg border p-3 text-left ${
                      ativo
                        ? "border-fuu-red bg-white ring-1 ring-fuu-red"
                        : "border-gray-300 bg-white hover:border-gray-400"
                    }`}
                  >
                    <Icon
                      className={`mt-0.5 h-4 w-4 shrink-0 ${
                        ativo ? "text-fuu-red" : "text-gray-400"
                      }`}
                    />
                    <span>
                      <span className="block text-sm font-medium text-gray-900">
                        {o.label}
                      </span>
                      <span className="block text-xs text-gray-500">{o.hint}</span>
                    </span>
                  </button>
                );
              })}
            </div>
            <p className="mt-3 flex items-start gap-2 text-xs text-gray-500">
              <FiAlertCircle className="mt-0.5 h-3.5 w-3.5 shrink-0" />
              O entregador recebe o frete cheio nos dois casos, e o cashback do
              cliente não muda. Cupom maior que a margem de quem banca acaba
              respingando no outro lado — o pedido só distribui o que foi pago.
            </p>
          </div>

          <div className="mt-6 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            <div>
              <label htmlFor="cupom-minimo" className="block text-sm font-medium text-gray-700 mb-1">
                Pedido mínimo (R$)
              </label>
              <input
                type="number"
                step="0.01"
                min="0"
                id="cupom-minimo"
                value={form.min_order_value}
                onChange={alterar("min_order_value")}
                placeholder="0 = sem mínimo"
                className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-fuu-red focus:outline-none"
              />
            </div>

            <div>
              <label htmlFor="cupom-max-usos" className="block text-sm font-medium text-gray-700 mb-1">
                Usos totais
              </label>
              <input
                type="number"
                min="0"
                id="cupom-max-usos"
                value={form.max_uses}
                onChange={alterar("max_uses")}
                placeholder="0 = ilimitado"
                className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-fuu-red focus:outline-none"
              />
              <p className="mt-1 text-xs text-gray-500">
                É o teto do prejuízo da promoção.
              </p>
            </div>

            <div>
              <label htmlFor="cupom-usos-cliente" className="block text-sm font-medium text-gray-700 mb-1">
                Usos por cliente
              </label>
              <input
                type="number"
                min="0"
                id="cupom-usos-cliente"
                value={form.max_uses_per_user}
                onChange={alterar("max_uses_per_user")}
                placeholder="0 = ilimitado"
                className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-fuu-red focus:outline-none"
              />
            </div>

            <div>
              <label htmlFor="cupom-estabelecimento" className="block text-sm font-medium text-gray-700 mb-1">
                Restaurante (id)
              </label>
              <input
                type="number"
                min="0"
                id="cupom-estabelecimento"
                value={form.establishment_id}
                onChange={alterar("establishment_id")}
                placeholder="0 = vale em todos"
                className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-fuu-red focus:outline-none"
              />
            </div>

            <div>
              <label htmlFor="cupom-inicio" className="block text-sm font-medium text-gray-700 mb-1">
                Início
              </label>
              <input
                type="datetime-local"
                id="cupom-inicio"
                value={form.start_date}
                onChange={alterar("start_date")}
                className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-fuu-red focus:outline-none"
              />
            </div>

            <div>
              <label htmlFor="cupom-validade" className="block text-sm font-medium text-gray-700 mb-1">
                Validade
              </label>
              <input
                type="datetime-local"
                id="cupom-validade"
                value={form.expiry_date}
                onChange={alterar("expiry_date")}
                className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-fuu-red focus:outline-none"
              />
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
              Criar cupom
            </button>
          </div>
        </form>
      )}

      <div className="overflow-x-auto rounded-xl border border-gray-200 bg-white shadow-sm">
        <table className="min-w-full divide-y divide-gray-200 text-sm">
          <thead className="bg-gray-50">
            <tr>
              {["Código", "Desconto", "Quem paga", "Usos", "Validade", "Situação", ""].map(
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

            {!carregando && cupons.length === 0 && (
              <tr>
                <td colSpan={7} className="px-4 py-10 text-center text-gray-500">
                  Nenhum cupom ainda. Crie o primeiro em "Novo cupom".
                </td>
              </tr>
            )}

            {!carregando &&
              cupons.map((c) => {
                // O backend serializa o modelo Go direto, sem json tags na
                // maioria dos campos: as chaves chegam em PascalCase. As duas
                // formas são aceitas para o dia em que isso for padronizado.
                const id = c.ID ?? c.id;
                const code = c.Code ?? c.code;
                const tipo = c.DiscountType ?? c.discount_type;
                const valor = c.DiscountValue ?? c.discount_value;
                const banca = c.FundedBy ?? c.funded_by ?? "platform";
                const usados = c.UsedCount ?? c.used_count ?? 0;
                const maxUsos = c.MaxUses ?? c.max_uses ?? 0;
                const ativo = c.IsActive ?? c.is_active;
                const validade = c.ExpiryDate ?? c.expiry_date;
                const expirado = validade && new Date(validade) < new Date();

                let descontoTexto = "—";
                if (tipo === "PERCENTAGE") descontoTexto = `${valor}%`;
                else if (tipo === "FIXED") descontoTexto = brl(valor);
                else if (tipo === "FREE_DELIVERY") descontoTexto = "Frete grátis";

                return (
                  <tr key={id} className="hover:bg-gray-50">
                    <td className="px-4 py-3 font-mono font-medium text-gray-900">
                      {code}
                    </td>
                    <td className="px-4 py-3 text-gray-700">{descontoTexto}</td>
                    <td className="px-4 py-3">
                      <span
                        className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium ${
                          banca === "establishment"
                            ? "bg-amber-100 text-amber-800"
                            : "bg-blue-100 text-blue-800"
                        }`}
                      >
                        {banca === "establishment" ? (
                          <>
                            <FiHome className="h-3 w-3" /> Restaurante
                          </>
                        ) : (
                          <>
                            <FiBriefcase className="h-3 w-3" /> Plataforma
                          </>
                        )}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-gray-700">
                      {usados}
                      {maxUsos > 0 ? ` / ${maxUsos}` : " / ∞"}
                    </td>
                    <td className="px-4 py-3 text-gray-700">{dataCurta(validade)}</td>
                    <td className="px-4 py-3">
                      {!ativo ? (
                        <span className="rounded-full bg-gray-100 px-2 py-0.5 text-xs text-gray-600">
                          Desativado
                        </span>
                      ) : expirado ? (
                        <span className="rounded-full bg-gray-100 px-2 py-0.5 text-xs text-gray-600">
                          Expirado
                        </span>
                      ) : (
                        <span className="rounded-full bg-green-100 px-2 py-0.5 text-xs text-green-700">
                          Ativo
                        </span>
                      )}
                    </td>
                    <td className="px-4 py-3 text-right">
                      {ativo && (
                        <button
                          type="button"
                          onClick={() => desativar(c)}
                          title="Desativar cupom"
                          className="rounded-lg p-2 text-gray-400 hover:bg-red-50 hover:text-fuu-red"
                        >
                          <FiTrash2 className="h-4 w-4" />
                        </button>
                      )}
                    </td>
                  </tr>
                );
              })}
          </tbody>
        </table>
      </div>
    </div>
  );
}
