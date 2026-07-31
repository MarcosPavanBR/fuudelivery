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
  const [loading, setLoading] = useState(true);
  const [tab, setTab] = useState("stats");
  const [processing, setProcessing] = useState(null);

  useEffect(() => { loadData(); }, []);

  async function loadData() {
    try {
      const [s, p, w] = await Promise.all([
        paymentApi.get("/payments/stats").then(r => r.data).catch(() => ({})),
        paymentApi.get("/payments/").then(r => r.data).catch(() => []),
        paymentApi.get("/wallets").then(r => r.data).catch(() => []),
      ]);
      setStats(s);
      setPayments(Array.isArray(p) ? p : []);
      setWallets(Array.isArray(w) ? w : []);
    } catch (e) { toast.error("Erro ao carregar dados: " + e.message); }
    setLoading(false);
  }

  async function approvePayment(id) {
    setProcessing(id);
    try {
      await paymentApi.post("/payments/" + id + "/approve");
      toast.success("Pagamento aprovado");
      await loadData();
    } catch (e) { toast.error(e.message); }
    setProcessing(null);
  }

  async function rejectPayment(id) {
    const motivo = prompt("Motivo da rejeicao:");
    if (!motivo?.trim()) return;
    setProcessing(id);
    try {
      await paymentApi.post("/payments/" + id + "/reject", { reason: motivo });
      toast.success("Pagamento rejeitado");
      await loadData();
    } catch (e) { toast.error(e.message); }
    setProcessing(null);
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
        {["stats", "payments", "wallets"].map(t => (
          <button key={t} onClick={() => setTab(t)}
            style={{ padding: "8px 20px", borderRadius: 8, border: "none", cursor: "pointer", fontWeight: 600,
              background: tab === t ? "#6366f1" : "#f3f4f6", color: tab === t ? "white" : "#374151" }}>
            {t === "stats" ? "Resumo" : t === "payments" ? "Pagamentos" : "Carteiras"}
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
                        <button disabled={processing === (p.id || p._id)} onClick={() => approvePayment(p.id || p._id)}
                          style={{ padding: "4px 12px", borderRadius: 6, border: "none", background: "#10b981", color: "white", fontSize: 12, cursor: processing ? "wait" : "pointer", opacity: processing === (p.id || p._id) ? 0.5 : 1 }}>
                          {processing === (p.id || p._id) ? "..." : "Aprovar"}
                        </button>
                        <button disabled={processing === (p.id || p._id)} onClick={() => rejectPayment(p.id || p._id)}
                          style={{ padding: "4px 12px", borderRadius: 6, border: "none", background: "#ef4444", color: "white", fontSize: 12, cursor: processing ? "wait" : "pointer", opacity: processing === (p.id || p._id) ? 0.5 : 1 }}>
                          {processing === (p.id || p._id) ? "..." : "Rejeitar"}
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
    </div>
  );
}
