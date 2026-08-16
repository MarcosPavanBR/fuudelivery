import React, { useState, useEffect } from "react";
import { FiCreditCard, FiDollarSign, FiClock, FiCheck, FiAlertTriangle } from "react-icons/fi";
import { toast } from "react-toastify";
import paymentApi from "../services/paymentApi";

function StatCard({ icon: Icon, label, value, color, bg }) {
  return (
    <div className="card p-6">
      <div className="flex items-center gap-3">
        <div
          className="w-11 h-11 rounded-xl flex items-center justify-center flex-shrink-0"
          style={{ background: bg || color + "15", color }}
        >
          <Icon size={22} />
        </div>
        <div className="min-w-0">
          <div className="text-xs font-medium text-gray-500">{label}</div>
          <div className="text-2xl font-bold text-gray-900">{value}</div>
        </div>
      </div>
    </div>
  );
}

const tabs = [
  { id: "stats", label: "Resumo" },
  { id: "payments", label: "Pagamentos" },
  { id: "wallets", label: "Carteiras" },
  { id: "chargebacks", label: "Chargebacks" },
];

export default function Financeiro() {
  const [stats, setStats] = useState(null);
  const [payments, setPayments] = useState([]);
  const [wallets, setWallets] = useState([]);
  const [chargebacks, setChargebacks] = useState([]);
  const [cbSummary, setCbSummary] = useState({ credit_total: 0, debit_total: 0, net: 0 });
  const [cbFilters, setCbFilters] = useState({ type: "", user_id: "", payment_id: "" });
  const [loading, setLoading] = useState(true);
  const [tab, setTab] = useState("stats");
  const [isProcessing, setIsProcessing] = useState(false);
  const [rejectModal, setRejectModal] = useState({ open: false, id: null, motivo: "" });

  useEffect(() => { loadData(); }, []);

  async function loadData() {
    try {
      const [s, p, w, c] = await Promise.all([
        paymentApi.get("/payments/stats").then(r => r.data).catch(() => ({})),
        paymentApi.get("/payments/").then(r => r.data).catch(() => []),
        paymentApi.get("/wallets").then(r => r.data).catch(() => []),
        paymentApi.get("/chargebacks").then(r => r.data).catch(() => ({})),
      ]);
      setStats(s);
      setPayments(Array.isArray(p) ? p : []);
      setWallets(Array.isArray(w) ? w : []);
      setChargebacks(Array.isArray(c?.chargebacks) ? c.chargebacks : []);
      setCbSummary(c?.summary || { credit_total: 0, debit_total: 0, net: 0 });
    } catch (e) { toast.error("Erro ao carregar dados: " + e.message); }
    setLoading(false);
  }

  // Carrega o ledger com os filtros atuais da aba Chargebacks.
  async function loadChargebacks(filters = cbFilters) {
    const params = new URLSearchParams();
    if (filters.type) params.append("type", filters.type);
    if (filters.user_id) params.append("user_id", filters.user_id);
    if (filters.payment_id) params.append("payment_id", filters.payment_id);
    const qs = params.toString();
    try {
      const { data } = await paymentApi.get("/chargebacks" + (qs ? "?" + qs : ""));
      setChargebacks(Array.isArray(data?.chargebacks) ? data.chargebacks : []);
      setCbSummary(data?.summary || { credit_total: 0, debit_total: 0, net: 0 });
    } catch (e) { toast.error("Erro ao buscar chargebacks: " + e.message); }
  }

  function applyCbFilters() { loadChargebacks(cbFilters); }

  function resetCbFilters() {
    setCbFilters({ type: "", user_id: "", payment_id: "" });
    loadChargebacks({ type: "", user_id: "", payment_id: "" });
  }

  async function approvePayment(id) {
    setIsProcessing(true);
    try {
      await paymentApi.post("/payments/" + id + "/approve");
      toast.success("Pagamento aprovado");
      await loadData();
    } catch (e) { toast.error(e.message); }
    setIsProcessing(false);
  }

  async function rejectPayment(id) {
    setRejectModal({ open: true, id, motivo: "" });
  }

  async function confirmReject() {
    const { id, motivo } = rejectModal;
    if (!motivo?.trim()) return;
    setIsProcessing(true);
    try {
      await paymentApi.post("/payments/" + id + "/reject", { reason: motivo });
      toast.success("Pagamento rejeitado");
      setRejectModal({ open: false, id: null, motivo: "" });
      await loadData();
    } catch (e) { toast.error(e.message); }
    setIsProcessing(false);
  }

  if (loading) {
    return (
      <div className="animate-fade-in space-y-6">
        <div className="space-y-2">
          <div className="skeleton h-8 w-40" />
        </div>
        <div className="flex gap-2">
          {[...Array(4)].map((_, i) => <div key={i} className="skeleton h-10 w-28" />)}
        </div>
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          {[...Array(4)].map((_, i) => (
            <div key={i} className="card p-6">
              <div className="skeleton h-12 w-12 rounded-xl mb-2" />
              <div className="skeleton h-4 w-20 mb-2" />
              <div className="skeleton h-7 w-16" />
            </div>
          ))}
        </div>
        <div className="card">
          <div className="p-6 space-y-3">
            {[...Array(6)].map((_, i) => <div key={i} className="skeleton h-9 w-full" />)}
          </div>
        </div>
      </div>
    );
  }

  const totalAmount = payments.reduce((s, p) => s + (p.amount || 0), 0);
  const pending = payments.filter(p => p.status === "PENDING");
  const approved = payments.filter(p => p.status === "CONFIRMED");
  const rejected = payments.filter(p => p.status === "REJECTED");

  return (
    <div className="animate-fade-in space-y-6 min-w-0">
      <h1 className="text-2xl font-bold text-gray-900">Financeiro</h1>

      {/* Tabs */}
      <div className="flex flex-wrap gap-2">
        {tabs.map(t => (
          <button
            key={t.id}
            onClick={() => setTab(t.id)}
            className={tab === t.id ? "btn btn-primary" : "btn btn-ghost"}
          >
            {t.label}
          </button>
        ))}
      </div>

      {tab === "stats" && (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          <StatCard icon={FiDollarSign} label="Total" value={"R$ " + (totalAmount / 100).toFixed(2)} color="#6366f1" bg="#EEF2FF" />
          <StatCard icon={FiClock} label="Pendentes" value={pending.length} color="#D97706" bg="#FFFBEB" />
          <StatCard icon={FiCheck} label="Aprovados" value={approved.length} color="#16A34A" bg="#F0FDF4" />
          <StatCard icon={FiAlertTriangle} label="Rejeitados" value={rejected.length} color="#DC2626" bg="#FEF2F2" />
        </div>
      )}

      {tab === "payments" && (
        <div className="card overflow-hidden">
          <table className="w-full">
            <thead className="bg-gray-50">
              <tr>
                {["ID", "Pedido", "Valor", "Status", "Ações"].map(h => (
                  <th key={h} className="px-4 py-2 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">{h}</th>
                ))}
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {payments.slice(0, 50).map(p => (
                <tr key={p.id || p._id} className="hover:bg-gray-50 transition-colors">
                  <td className="px-4 py-2 text-sm font-mono">{(p.id || p._id || "").slice(0, 8)}</td>
                  <td className="px-4 py-2 text-sm">{p.orderId || p.order_id || "-"}</td>
                  <td className="px-4 py-2 text-sm font-semibold">R$ {((p.amount || 0) / 100).toFixed(2)}</td>
                  <td className="px-4 py-2">
                    <span
                      className="inline-flex items-center px-2.5 py-1 rounded-full text-xs font-semibold"
                      style={{
                        background: p.status === "CONFIRMED" ? "#D1FAE5" : p.status === "PENDING" ? "#FEF3C7" : "#FEE2E2",
                        color: p.status === "CONFIRMED" ? "#065F46" : p.status === "PENDING" ? "#92400E" : "#991B1B",
                      }}
                    >
                      {p.status}
                    </span>
                  </td>
                  <td className="px-4 py-2">
                    {p.status === "PENDING" && (
                      <div className="flex gap-2">
                        <button disabled={isProcessing} onClick={() => approvePayment(p.id || p._id)} className="btn btn-primary text-xs">
                          {isProcessing ? "..." : "Aprovar"}
                        </button>
                        <button disabled={isProcessing} onClick={() => rejectPayment(p.id || p._id)} className="btn btn-danger text-xs">
                          {isProcessing ? "..." : "Rejeitar"}
                        </button>
                      </div>
                    )}
                  </td>
                </tr>
              ))}
              {payments.length === 0 && (
                <tr><td colSpan={5} className="px-4 py-16 text-center text-gray-400">Nenhum pagamento encontrado</td></tr>
              )}
            </tbody>
          </table>
        </div>
      )}

      {tab === "wallets" && (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
          {wallets.map((w, i) => (
            <div key={w.id || w._id || i} className="card p-6">                      <div className="flex items-center gap-2 mb-2">
                <FiCreditCard className="h-5 w-5 text-fuu-red" />
                <span className="font-semibold text-gray-900">{w.ownerName || w.owner_type || "Carteira"}</span>
              </div>
              <div className="text-3xl font-bold text-green-600">R$ {((w.balance || 0) / 100).toFixed(2)}</div>
              <div className="text-xs text-gray-500 mt-1">ID: {(w.id || w._id || "").slice(0, 8)}</div>
            </div>
          ))}
          {wallets.length === 0 && (
            <div className="card p-16 text-center text-gray-400 sm:col-span-2 lg:col-span-3">Nenhuma carteira encontrada</div>
          )}
        </div>
      )}

      {tab === "chargebacks" && (
        <div className="space-y-4">
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
            <StatCard icon={FiCheck} label="Total de créditos" value={"R$ " + (cbSummary?.credit_total || 0).toFixed(2)} color="#16A34A" bg="#F0FDF4" />
            <StatCard icon={FiAlertTriangle} label="Total de débitos" value={"R$ " + (cbSummary?.debit_total || 0).toFixed(2)} color="#DC2626" bg="#FEF2F2" />
            <StatCard icon={FiDollarSign} label="Saldo líquido" value={"R$ " + (cbSummary?.net || 0).toFixed(2)} color="#6366f1" bg="#EEF2FF" />
          </div>

          <div className="flex flex-wrap gap-2 items-center">
            <select
              value={cbFilters.type}
              onChange={e => setCbFilters({ ...cbFilters, type: e.target.value })}
              className="input w-auto"
            >
              <option value="">Todos os tipos</option>
              <option value="credit">Crédito</option>
              <option value="debit">Débito</option>
            </select>
            <input
              value={cbFilters.user_id}
              onChange={e => setCbFilters({ ...cbFilters, user_id: e.target.value })}
              placeholder="user_id"
              className="input w-32"
            />
            <input
              value={cbFilters.payment_id}
              onChange={e => setCbFilters({ ...cbFilters, payment_id: e.target.value })}
              placeholder="payment_id"
              className="input w-48"
            />
            <button onClick={applyCbFilters} className="btn btn-primary">Filtrar</button>
            <button onClick={resetCbFilters} className="btn btn-ghost">Limpar</button>
          </div>

          <div className="card overflow-hidden">
            <div className="overflow-x-auto">
              <table className="w-full">
                <thead className="bg-gray-50">
                  <tr>
                    {["Usuário", "Tipo", "Valor", "Payment", "Saldo após", "Descrição", "Data"].map(h => (
                      <th key={h} className="px-4 py-2 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">{h}</th>
                    ))}
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-100">
                  {chargebacks.map((c, i) => (
                    <tr key={c._id || i} className="hover:bg-gray-50 transition-colors">
                      <td className="px-4 py-2 text-sm">
                        <div className="font-semibold text-gray-900">#{c.user_id ?? "-"}</div>
                        <div className="text-xs text-gray-400">{c.owner_type || ""}</div>
                      </td>
                      <td className="px-4 py-2">
                        <span
                          className="inline-flex items-center px-2.5 py-1 rounded-full text-xs font-semibold"
                          style={{
                            background: c.type === "credit" ? "#D1FAE5" : c.type === "debit" ? "#FEE2E2" : "#F3F4F6",
                            color: c.type === "credit" ? "#065F46" : c.type === "debit" ? "#991B1B" : "#374151",
                          }}
                        >
                          {c.type === "credit" ? "Crédito" : c.type === "debit" ? "Débito" : (c.type || "-")}
                        </span>
                      </td>
                      <td className="px-4 py-2 text-sm font-semibold" style={{ color: c.type === "debit" ? "#DC2626" : "#059669" }}>
                        {c.type === "debit" ? "-" : "+"}R$ {(c.amount || 0).toFixed(2)}
                      </td>
                      <td className="px-4 py-2 text-xs font-mono text-gray-500">{c.payment_id || "-"}</td>
                      <td className="px-4 py-2 text-sm">R$ {(c.balance_after || 0).toFixed(2)}</td>
                      <td className="px-4 py-2 text-sm text-gray-600">{c.description || "-"}</td>
                      <td className="px-4 py-2 text-xs text-gray-500">{c.created_at ? new Date(c.created_at).toLocaleString("pt-BR") : "-"}</td>
                    </tr>
                  ))}
                  {chargebacks.length === 0 && (
                    <tr><td colSpan={7} className="px-4 py-16 text-center text-gray-400">Nenhum lançamento encontrado</td></tr>
                  )}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      )}

      {rejectModal.open && (
        <div
          className="fixed inset-0 bg-black/40 flex items-center justify-center z-[1000] p-4"
          onClick={() => !isProcessing && setRejectModal({ ...rejectModal, open: false })}
        >
          <div className="card p-6 w-full max-w-md" onClick={e => e.stopPropagation()}>
            <h3 className="text-lg font-bold text-gray-900 mb-2">Motivo da Rejeição</h3>
            <textarea
              value={rejectModal.motivo}
              onChange={e => setRejectModal({ ...rejectModal, motivo: e.target.value })}
              placeholder="Descreva o motivo..."
              rows={4}
              className="input resize-y"
            />
            <div className="flex gap-2 justify-end mt-4">
              <button onClick={() => setRejectModal({ ...rejectModal, open: false })} className="btn btn-ghost">
                Cancelar
              </button>
              <button
                onClick={confirmReject}
                disabled={!rejectModal.motivo?.trim() || isProcessing}
                className="btn btn-danger"
              >
                {isProcessing ? "Enviando..." : "Rejeitar"}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
