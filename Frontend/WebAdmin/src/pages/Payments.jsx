import React, { useState, useEffect } from "react";
import {
  FiCreditCard,
  FiDollarSign,
  FiActivity,
  FiFilter,
  FiEye,
  FiClock,
  FiCheck,
  FiSearch,
  FiRefreshCw,
} from "react-icons/fi";
import api from "../services/api";
import { toast } from "react-toastify";

// Status reais do monolito (payments collection).
// amount é armazenado em CENTAVOS — dividimos por 100 para exibir em reais.
const statusColors = {
  CONFIRMED: { bg: "#ECFDF5", text: "#047857", label: "Confirmado" },
  PENDING: { bg: "#FEF3C7", text: "#B45309", label: "Pendente" },
  REFUNDED: { bg: "#F3F4F6", text: "#4B5563", label: "Estornado" },
  REJECTED: { bg: "#FEE2E2", text: "#B91C1C", label: "Rejeitado" },
  EXPIRED: { bg: "#F3F4F6", text: "#6B7280", label: "Expirado" },
  CANCELLED: { bg: "#F3F4F6", text: "#6B7280", label: "Cancelado" },
};

const statusOptions = [
  { value: "", label: "Todos status" },
  { value: "CONFIRMED", label: "Confirmado" },
  { value: "PENDING", label: "Pendente" },
  { value: "REFUNDED", label: "Estornado" },
  { value: "REJECTED", label: "Rejeitado" },
  { value: "EXPIRED", label: "Expirado" },
  { value: "CANCELLED", label: "Cancelado" },
];

// Identificador do cliente: sem user aninhado no payload — usamos o
// customer_phone (ou o customer_id como fallback).
function customerLabel(p) {
  return p.customer_phone || (p.customer_id != null ? `#${p.customer_id}` : "Cliente");
}

function formatMoney(cents) {
  return `R$ ${((cents || 0) / 100).toFixed(2)}`;
}

export default function Payments() {
  const [payments, setPayments] = useState([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [stats, setStats] = useState({ total: 0, confirmed: 0, pending: 0, revenue: 0 });

  useEffect(() => { loadPayments(); }, []);

  const loadPayments = async () => {
    try {
      const { data } = await api.get("/payments/all");
      const list = Array.isArray(data) ? data : [];
      setPayments(list);
      setStats({
        total: list.length,
        confirmed: list.filter(p => p.status === "CONFIRMED").length,
        pending: list.filter(p => p.status === "PENDING").length,
        revenue: list
          .filter(p => p.status === "CONFIRMED")
          .reduce((sum, p) => sum + (p.amount || 0), 0), // centavos
      });
    } catch (e) {
      console.error(e);
      toast.error("Erro ao carregar pagamentos");
    }
    setLoading(false);
  };

  const filtered = payments.filter(p => {
    const idStr = (p._id || p.id || "").toString();
    const matchesSearch =
      !search ||
      idStr.toLowerCase().includes(search.toLowerCase()) ||
      (p.customer_phone || "").toLowerCase().includes(search.toLowerCase()) ||
      (p.order_id || "").toLowerCase().includes(search.toLowerCase());
    const matchesStatus = !statusFilter || p.status === statusFilter;
    return matchesSearch && matchesStatus;
  });

  if (loading) {
    return (
      <div className="animate-fade-in space-y-6">
        <div className="space-y-2">
          <div className="skeleton h-8 w-40" />
          <div className="skeleton h-4 w-56" />
        </div>
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          {[...Array(4)].map((_, i) => (
            <div key={i} className="card p-5">
              <div className="skeleton h-12 w-12 rounded-xl mb-3" />
              <div className="skeleton h-4 w-20 mb-2" />
              <div className="skeleton h-7 w-16" />
            </div>
          ))}
        </div>
        <div className="card">
          <div className="p-4"><div className="skeleton h-10 w-full" /></div>
          <div className="p-6 space-y-3">
            {[...Array(6)].map((_, i) => <div key={i} className="skeleton h-9 w-full" />)}
          </div>
        </div>
      </div>
    );
  }

  const statCards = [
    { label: "Total Transações", value: stats.total, icon: FiCreditCard, color: "#EA1D2C", bg: "#FEF2F2" },
    { label: "Confirmados", value: stats.confirmed, icon: FiCheck, color: "#16A34A", bg: "#ECFDF5" },
    { label: "Pendentes", value: stats.pending, icon: FiClock, color: "#D97706", bg: "#FFFBEB" },
    { label: "Receita", value: formatMoney(stats.revenue), icon: FiDollarSign, color: "#F7A11E", bg: "#FFFBEB" },
  ];

  return (
    <div className="animate-fade-in space-y-6 min-w-0">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Pagamentos</h1>
          <p className="text-gray-500 mt-1">{filtered.length} de {payments.length}</p>
        </div>
        <button onClick={loadPayments} className="btn btn-ghost" title="Atualizar">
          <FiRefreshCw className="h-4 w-4" /> Atualizar
        </button>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        {statCards.map((s, i) => (
          <div key={i} className="card p-5 transition-all duration-150 hover:shadow-card-hover">
            <div className="flex items-start justify-between">
              <div className="min-w-0">
                <p className="text-sm font-medium text-gray-500">{s.label}</p>
                <p className="text-3xl font-bold mt-2 text-gray-900">{s.value}</p>
              </div>
              <div className="p-3 rounded-xl flex-shrink-0" style={{ background: s.bg, color: s.color }}>
                <s.icon className="h-6 w-6" />
              </div>
            </div>
          </div>
        ))}
      </div>

      <div className="card p-4">
        <div className="flex flex-col sm:flex-row gap-4">
          <div className="flex-1 relative">
            <FiSearch className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" />
            <input
              type="text"
              placeholder="Buscar por ID, cliente, pedido..."
              value={search}
              onChange={e => setSearch(e.target.value)}
              className="input pl-10"
            />
          </div>
          <div className="relative">
            <FiFilter className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" />
            <select
              value={statusFilter}
              onChange={e => setStatusFilter(e.target.value)}
              className="input w-44 pl-10"
            >
              {statusOptions.map(o => <option key={o.value} value={o.value}>{o.label}</option>)}
            </select>
          </div>
        </div>
      </div>

      <div className="card overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">ID</th>
                <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Cliente</th>
                <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Pedido</th>
                <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Valor</th>
                <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Método</th>
                <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Status</th>
                <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Data</th>
                <th className="px-6 py-3 text-right text-xs font-semibold text-gray-500 uppercase tracking-wider">Ações</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {filtered.length === 0 ? (
                <tr><td colSpan={8} className="px-6 py-16 text-center text-gray-400">Nenhum pagamento</td></tr>
              ) : filtered.map(p => {
                const sc = statusColors[p.status] || { bg: "#F3F4F6", text: "#4B5563", label: p.status || "-" };
                return (
                  <tr key={(p._id || p.id || "").toString()} className="hover:bg-gray-50 transition-colors">
                    <td className="px-6 py-4">
                      <span className="font-medium text-gray-900">#{(p._id || p.id || "").toString().slice(-8)}</span>
                    </td>
                    <td className="px-6 py-4">
                      <p className="text-sm text-gray-900">{customerLabel(p)}</p>
                      {p.customer_id != null && <p className="text-xs text-gray-400">ID: {p.customer_id}</p>}
                    </td>
                    <td className="px-6 py-4 text-sm text-gray-600">{p.order_id || "-"}</td>
                    <td className="px-6 py-4 font-semibold text-gray-900">{formatMoney(p.amount)}</td>
                    <td className="px-6 py-4 text-sm text-gray-600">{p.method || "-"}</td>
                    <td className="px-6 py-4">
                      <span className="inline-flex items-center px-2.5 py-1 rounded-full text-xs font-medium" style={{ background: sc.bg, color: sc.text }}>
                        {sc.label}
                      </span>
                    </td>
                    <td className="px-6 py-4 text-sm text-gray-500">
                      {p.created_at ? new Date(p.created_at).toLocaleString("pt-BR") : "-"}
                    </td>
                    <td className="px-6 py-4">
                      <div className="flex items-center gap-2 justify-end">
                        <button
                          className="p-2 text-gray-400 hover:text-fuu-red hover:bg-fuu-red-light rounded-lg transition-colors"
                          title={p.abacatepay_id || p.order_id || "Detalhes"}
                        >
                          <FiEye className="h-4 w-4" />
                        </button>
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
