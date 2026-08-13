import React, { useState, useEffect } from "react";
import { FiCreditCard, FiDollarSign, FiClock, FiCheck, FiAlertTriangle } from "react-icons/fi";
import { toast } from "react-toastify";
import paymentApi from "../services/paymentApi";

function StatCard({ icon: Icon, label, value, color }) {
  return (
    <div style={{ background: "white", borderRadius: 12, padding: 20, boxShadow: "0 1px 3px rgba(0,0,0,0.1)" }}>
      <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
        <div style={{ width: 44, height: 44, borderRadius: 10, background: color + "15", display: "flex", alignItems: "center", justifyContent: "center" }}>
          <Icon size={22} color={color} />
        </div>
        <div>
          <div style={{ fontSize: 12, color: "#6b7280" }}>{label}</div>
          <div style={{ fontSize: 24, fontWeight: 700 }}>{value}</div>
        </div>
      </div>
    </div>
  );
}

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

  if (loading) return <div style={{ padding: 40, textAlign: "center" }}>Carregando...</div>;

  const totalAmount = payments.reduce((s, p) => s + (p.amount || 0), 0);
  const pending = payments.filter(p => p.status === "PENDING");
  const approved = payments.filter(p => p.status === "CONFIRMED");
  const rejected = payments.filter(p => p.status === "REJECTED");

  return (
    <div style={{ padding: 24 }}>
      <h1 style={{ fontSize: 28, fontWeight: 700, marginBottom: 24 }}>Financeiro</h1>
      <div style={{ display: "flex", gap: 8, marginBottom: 24 }}>
        {["stats", "payments", "wallets", "chargebacks"].map(t => (
          <button key={t} onClick={() => setTab(t)}
            style={{ padding: "8px 20px", borderRadius: 8, border: "none", cursor: "pointer", fontWeight: 600,
              background: tab === t ? "#6366f1" : "#f3f4f6", color: tab === t ? "white" : "#374151" }}>
            {t === "stats" ? "Resumo" : t === "payments" ? "Pagamentos" : t === "wallets" ? "Carteiras" : "Chargebacks"}
          </button>
        ))}
      </div>

      {tab === "stats" && (
        <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(200px, 1fr))", gap: 16 }}>
          <StatCard icon={FiDollarSign} label="Total" value={"R$ " + (totalAmount / 100).toFixed(2)} color="#6366f1" />
          <StatCard icon={FiClock} label="Pendentes" value={pending.length} color="#f59e0b" />
          <StatCard icon={FiCheck} label="Aprovados" value={approved.length} color="#10b981" />
          <StatCard icon={FiAlertTriangle} label="Rejeitados" value={rejected.length} color="#ef4444" />
        </div>
      )}

      {tab === "payments" && (
        <div style={{ background: "white", borderRadius: 12, overflow: "hidden", boxShadow: "0 1px 3px rgba(0,0,0,0.1)" }}>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <thead><tr style={{ background: "#f9fafb" }}>
              {["ID", "Pedido", "Valor", "Status", "Acoes"].map(h => (
                <th key={h} style={{ padding: "12px 16px", textAlign: "left", fontSize: 12, color: "#6b7280" }}>{h}</th>
              ))}
            </tr></thead>
            <tbody>
              {payments.slice(0, 50).map(p => (
                <tr key={p.id || p._id} style={{ borderTop: "1px solid #f3f4f6" }}>
                  <td style={{ padding: "12px 16px", fontSize: 13, fontFamily: "monospace" }}>{(p.id || p._id || "").slice(0, 8)}</td>
                  <td style={{ padding: "12px 16px", fontSize: 13 }}>{p.orderId || p.order_id || "-"}</td>
                  <td style={{ padding: "12px 16px", fontSize: 13, fontWeight: 600 }}>R$ {((p.amount || 0) / 100).toFixed(2)}</td>
                  <td style={{ padding: "12px 16px" }}>
                    <span style={{ padding: "4px 10px", borderRadius: 20, fontSize: 12, fontWeight: 600,
                      background: p.status === "CONFIRMED" ? "#d1fae5" : p.status === "PENDING" ? "#fef3c7" : "#fee2e2",
                      color: p.status === "CONFIRMED" ? "#065f46" : p.status === "PENDING" ? "#92400e" : "#991b1b" }}>
                      {p.status}
                    </span>
                  </td>
                  <td style={{ padding: "12px 16px" }}>
                    {p.status === "PENDING" && (
                      <div style={{ display: "flex", gap: 6 }}>
                        <button disabled={isProcessing} onClick={() => approvePayment(p.id || p._id)}
                          style={{ padding: "4px 12px", borderRadius: 6, border: "none", background: "#10b981", color: "white", fontSize: 12, cursor: isProcessing ? "wait" : "pointer", opacity: isProcessing ? 0.5 : 1 }}>
                          {isProcessing ? "..." : "Aprovar"}
                        </button>
                        <button disabled={isProcessing} onClick={() => rejectPayment(p.id || p._id)}
                          style={{ padding: "4px 12px", borderRadius: 6, border: "none", background: "#ef4444", color: "white", fontSize: 12, cursor: isProcessing ? "wait" : "pointer", opacity: isProcessing ? 0.5 : 1 }}>
                          {isProcessing ? "..." : "Rejeitar"}
                        </button>
                      </div>
                    )}
                  </td>
                </tr>
              ))}
              {payments.length === 0 && <tr><td colSpan={5} style={{ padding: 40, textAlign: "center", color: "#9ca3af" }}>Nenhum pagamento encontrado</td></tr>}
            </tbody>
          </table>
        </div>
      )}

      {tab === "wallets" && (
        <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(280px, 1fr))", gap: 16 }}>
          {wallets.map((w, i) => (
            <div key={w.id || w._id || i} style={{ background: "white", borderRadius: 12, padding: 20, boxShadow: "0 1px 3px rgba(0,0,0,0.1)" }}>
              <div style={{ display: "flex", alignItems: "center", gap: 10, marginBottom: 12 }}>
                <FiCreditCard size={20} color="#6366f1" />
                <span style={{ fontWeight: 600 }}>{w.ownerName || w.owner_type || "Carteira"}</span>
              </div>
              <div style={{ fontSize: 28, fontWeight: 700, color: "#059669" }}>R$ {((w.balance || 0) / 100).toFixed(2)}</div>
              <div style={{ fontSize: 12, color: "#6b7280", marginTop: 4 }}>ID: {(w.id || w._id || "").slice(0, 8)}</div>
            </div>
          ))}
          {wallets.length === 0 && <div style={{ gridColumn: "1 / -1", padding: 40, textAlign: "center", color: "#9ca3af" }}>Nenhuma carteira encontrada</div>}
        </div>
      )}

      {tab === "chargebacks" && (
        <div>
          <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(200px, 1fr))", gap: 16, marginBottom: 16 }}>
            <StatCard icon={FiCheck} label="Total de créditos" value={"R$ " + (cbSummary?.credit_total || 0).toFixed(2)} color="#10b981" />
            <StatCard icon={FiAlertTriangle} label="Total de débitos" value={"R$ " + (cbSummary?.debit_total || 0).toFixed(2)} color="#ef4444" />
            <StatCard icon={FiDollarSign} label="Saldo líquido" value={"R$ " + (cbSummary?.net || 0).toFixed(2)} color="#6366f1" />
          </div>
          <div style={{ display: "flex", gap: 8, flexWrap: "wrap", alignItems: "center", marginBottom: 16 }}>
            <select value={cbFilters.type} onChange={e => setCbFilters({ ...cbFilters, type: e.target.value })}
              style={{ padding: "8px 12px", borderRadius: 8, border: "1px solid #d1d5db", background: "white", fontSize: 13 }}>
              <option value="">Todos os tipos</option>
              <option value="credit">Crédito</option>
              <option value="debit">Débito</option>
            </select>
            <input value={cbFilters.user_id} onChange={e => setCbFilters({ ...cbFilters, user_id: e.target.value })}
              placeholder="user_id" style={{ padding: "8px 12px", borderRadius: 8, border: "1px solid #d1d5db", background: "white", fontSize: 13 }} />
            <input value={cbFilters.payment_id} onChange={e => setCbFilters({ ...cbFilters, payment_id: e.target.value })}
              placeholder="payment_id" style={{ padding: "8px 12px", borderRadius: 8, border: "1px solid #d1d5db", background: "white", fontSize: 13 }} />
            <button onClick={applyCbFilters}
              style={{ padding: "8px 16px", borderRadius: 8, border: "none", background: "#6366f1", color: "white", fontSize: 13, fontWeight: 600, cursor: "pointer" }}>
              Filtrar
            </button>
            <button onClick={resetCbFilters}
              style={{ padding: "8px 16px", borderRadius: 8, border: "1px solid #d1d5db", background: "white", color: "#374151", fontSize: 13, fontWeight: 600, cursor: "pointer" }}>
              Limpar
            </button>
          </div>
          <div style={{ background: "white", borderRadius: 12, overflow: "hidden", boxShadow: "0 1px 3px rgba(0,0,0,0.1)" }}>
            <table style={{ width: "100%", borderCollapse: "collapse" }}>
              <thead><tr style={{ background: "#f9fafb" }}>
                {["Usuário", "Tipo", "Valor", "Payment", "Saldo após", "Descrição", "Data"].map(h => (
                  <th key={h} style={{ padding: "12px 16px", textAlign: "left", fontSize: 12, color: "#6b7280" }}>{h}</th>
                ))}
              </tr></thead>
              <tbody>
                {chargebacks.map((c, i) => (
                  <tr key={c._id || i} style={{ borderTop: "1px solid #f3f4f6" }}>
                    <td style={{ padding: "12px 16px", fontSize: 13 }}>
                      <div style={{ fontWeight: 600 }}>#{c.user_id ?? "-"}</div>
                      <div style={{ fontSize: 11, color: "#9ca3af" }}>{c.owner_type || ""}</div>
                    </td>
                    <td style={{ padding: "12px 16px" }}>
                      <span style={{ padding: "4px 10px", borderRadius: 20, fontSize: 12, fontWeight: 600,
                        background: c.type === "credit" ? "#d1fae5" : c.type === "debit" ? "#fee2e2" : "#f3f4f6",
                        color: c.type === "credit" ? "#065f46" : c.type === "debit" ? "#991b1b" : "#374151" }}>
                        {c.type === "credit" ? "Crédito" : c.type === "debit" ? "Débito" : (c.type || "-")}
                      </span>
                    </td>
                    <td style={{ padding: "12px 16px", fontSize: 13, fontWeight: 600, color: c.type === "debit" ? "#dc2626" : "#059669" }}>
                      {c.type === "debit" ? "-" : "+"}R$ {(c.amount || 0).toFixed(2)}
                    </td>
                    <td style={{ padding: "12px 16px", fontSize: 12, fontFamily: "monospace", color: "#6b7280" }}>{c.payment_id || "-"}</td>
                    <td style={{ padding: "12px 16px", fontSize: 13 }}>R$ {(c.balance_after || 0).toFixed(2)}</td>
                    <td style={{ padding: "12px 16px", fontSize: 13, color: "#374151" }}>{c.description || "-"}</td>
                    <td style={{ padding: "12px 16px", fontSize: 12, color: "#6b7280" }}>{c.created_at ? new Date(c.created_at).toLocaleString("pt-BR") : "-"}</td>
                  </tr>
                ))}
                {chargebacks.length === 0 && <tr><td colSpan={7} style={{ padding: 40, textAlign: "center", color: "#9ca3af" }}>Nenhum lançamento encontrado</td></tr>}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {rejectModal.open && (
        <div style={{ position: "fixed", inset: 0, background: "rgba(0,0,0,0.4)", display: "flex", alignItems: "center", justifyContent: "center", zIndex: 1000 }}
          onClick={() => !isProcessing && setRejectModal({ ...rejectModal, open: false })}>
          <div style={{ background: "white", borderRadius: 12, padding: 24, width: 400, maxWidth: "90vw" }}
            onClick={e => e.stopPropagation()}>
            <h3 style={{ fontSize: 18, fontWeight: 700, marginBottom: 12 }}>Motivo da Rejeicao</h3>
            <textarea
              value={rejectModal.motivo}
              onChange={e => setRejectModal({ ...rejectModal, motivo: e.target.value })}
              placeholder="Descreva o motivo..."
              rows={4}
              style={{ width: "100%", padding: 10, border: "1px solid #d1d5db", borderRadius: 8, fontSize: 14, resize: "vertical", boxSizing: "border-box" }}
            />
            <div style={{ display: "flex", gap: 8, justifyContent: "flex-end", marginTop: 16 }}>
              <button onClick={() => setRejectModal({ ...rejectModal, open: false })}
                style={{ padding: "8px 16px", borderRadius: 8, border: "1px solid #d1d5db", background: "white", cursor: "pointer", fontWeight: 600 }}>
                Cancelar
              </button>
              <button onClick={confirmReject} disabled={!rejectModal.motivo?.trim() || isProcessing}
                style={{ padding: "8px 16px", borderRadius: 8, border: "none", background: rejectModal.motivo?.trim() && !isProcessing ? "#ef4444" : "#fca5a5", color: "white", cursor: rejectModal.motivo?.trim() && !isProcessing ? "pointer" : "not-allowed", fontWeight: 600 }}>
                {isProcessing ? "Enviando..." : "Rejeitar"}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
