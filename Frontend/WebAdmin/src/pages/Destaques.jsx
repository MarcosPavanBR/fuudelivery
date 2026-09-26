import React, { useCallback, useEffect, useState } from "react";
import { FiTrendingUp, FiCheck, FiX, FiLoader } from "react-icons/fi";
import api from "../services/api";
import { toast } from "react-toastify";

// Destaque patrocinado por dia (payment_api/app/handlers/sponsor.go).
// Aqui o admin confirma o PIX das reservas pendentes e cancela quando
// preciso. Reserva paga pela carteira já nasce ativa.

const statusLabel = {
  pending_payment: { text: "Aguardando PIX", cls: "bg-amber-50 text-amber-700" },
  active: { text: "Pago", cls: "bg-green-50 text-green-700" },
  cancelled: { text: "Cancelado", cls: "bg-gray-100 text-gray-500" },
};

const brl = (v) =>
  Number(v || 0).toLocaleString("pt-BR", { style: "currency", currency: "BRL" });

const shortDay = (d) => {
  const [, m, day] = String(d || "").split("-");
  return day && m ? `${day}/${m}` : String(d || "");
};

export default function Destaques() {
  const [rows, setRows] = useState([]);
  const [filter, setFilter] = useState("pending_payment");
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const { data } = await api.get("/sponsorship/bookings", { params: filter ? { status: filter } : {} });
      setRows(Array.isArray(data) ? data : []);
    } catch (e) {
      toast.error("Não foi possível carregar os destaques.");
    }
    setLoading(false);
  }, [filter]);

  useEffect(() => {
    load();
  }, [load]);

  const act = async (id, action, confirmMsg) => {
    if (confirmMsg && !window.confirm(confirmMsg)) return;
    setBusy(id);
    try {
      await api.post(`/sponsorship/bookings/${id}/${action}`);
      toast.success(action === "confirm" ? "Pagamento confirmado. O destaque está ativo." : "Reserva cancelada.");
      await load();
    } catch (e) {
      toast.error(e.response?.data?.error || "Não foi possível concluir.");
    }
    setBusy(null);
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <FiTrendingUp className="h-6 w-6 text-red-600" />
          <h1 className="text-2xl font-bold text-gray-900">Destaques patrocinados</h1>
        </div>
        <select
          className="rounded-lg border border-gray-200 px-3 py-2 text-sm"
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
        >
          <option value="pending_payment">Aguardando PIX</option>
          <option value="active">Pagos</option>
          <option value="cancelled">Cancelados</option>
          <option value="">Todos</option>
        </select>
      </div>

      <p className="text-sm text-gray-600">
        A loja reserva dias no topo do app (preço por dia em <code>SPONSOR_DAILY_PRICE</code>, vagas por dia em{" "}
        <code>SPONSOR_SLOTS_PER_DAY</code>). O PIX é confirmado sozinho pelo webhook do gateway; confirme aqui só se o
        PIX automático estiver fora do ar e o pagamento vier por outro meio.
      </p>

      <div className="rounded-xl border border-gray-100 bg-white">
        {loading ? (
          <div className="flex h-24 items-center justify-center">
            <FiLoader className="h-5 w-5 animate-spin text-red-600" />
          </div>
        ) : rows.length === 0 ? (
          <p className="p-6 text-sm text-gray-500">Nenhuma reserva.</p>
        ) : (
          <table className="w-full text-sm">
            <thead className="bg-gray-50 text-left text-xs uppercase text-gray-500">
              <tr>
                <th className="px-4 py-3">Loja</th>
                <th className="px-4 py-3">Período</th>
                <th className="px-4 py-3">Valor</th>
                <th className="px-4 py-3">Pagamento</th>
                <th className="px-4 py-3">Status</th>
                <th className="px-4 py-3" />
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {rows.map(({ booking: b, establishment_name: name }) => {
                const st = statusLabel[b.status] || { text: b.status, cls: "bg-gray-100" };
                const expired = b.status === "pending_payment" && b.expires_at && new Date(b.expires_at) < new Date();
                return (
                  <tr key={b.id}>
                    <td className="px-4 py-3 font-medium text-gray-900">{name || `Loja ${b.establishment_id}`}</td>
                    <td className="px-4 py-3">
                      {shortDay(b.start_day)} a {shortDay(b.end_day)} ({b.days} dia{b.days > 1 ? "s" : ""})
                    </td>
                    <td className="px-4 py-3">{brl(b.total)}</td>
                    <td className="px-4 py-3">{b.pay_with === "wallet" ? "Carteira" : "PIX"}</td>
                    <td className="px-4 py-3">
                      <span className={`rounded-full px-2.5 py-1 text-xs font-semibold ${st.cls}`}>{st.text}</span>
                      {expired && !b.paid_at && <span className="ml-2 text-xs text-red-600">vaga liberada</span>}
                      {b.paid_at && b.status === "cancelled" && (
                        <span className="ml-2 text-xs text-gray-500">valor devolvido à carteira da loja</span>
                      )}
                    </td>
                    <td className="px-4 py-3 text-right whitespace-nowrap">
                      {b.status === "pending_payment" && (
                        <button
                          type="button"
                          disabled={busy === b.id}
                          onClick={() => act(b.id, "confirm", `Confirmar o PIX de ${brl(b.total)} de ${name}?`)}
                          className="mr-2 inline-flex items-center gap-1 rounded-lg bg-green-600 px-3 py-1.5 text-xs font-semibold text-white hover:bg-green-700 disabled:opacity-50"
                        >
                          <FiCheck /> Confirmar PIX
                        </button>
                      )}
                      {b.status !== "cancelled" && (
                        <button
                          type="button"
                          disabled={busy === b.id}
                          onClick={() =>
                            act(
                              b.id,
                              "cancel",
                              b.status === "active"
                                ? "Cancelar? Se ainda não começou, o valor volta para a carteira da loja."
                                : "Cancelar esta reserva?"
                            )
                          }
                          className="inline-flex items-center gap-1 rounded-lg border border-gray-200 px-3 py-1.5 text-xs font-semibold text-gray-700 hover:bg-gray-50 disabled:opacity-50"
                        >
                          <FiX /> Cancelar
                        </button>
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}
