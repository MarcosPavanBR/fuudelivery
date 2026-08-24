import React, { useState, useEffect } from "react";
import { FiShield, FiFilter, FiRefreshCw, FiActivity } from "react-icons/fi";
import api from "../services/api";
import { toast } from "react-toastify";
import Pagination from "../components/Pagination";

// Rotulo e cor por acao administrativa (semantica do Design System Fuu):
// verde = acao de sucesso/confirmacao, vermelho = destrutiva, azul = criacao/info.
const ACTION_META = {
  PAYMENT_APPROVED: { label: "Aprovou pagamento", color: "#16A34A", bg: "#F0FDF4" },
  PAYMENT_REJECTED: { label: "Rejeitou pagamento", color: "#DC2626", bg: "#FEF2F2" },
  USER_CREATED: { label: "Criou usuário", color: "#2563EB", bg: "#EFF6FF" },
  USER_DELETED: { label: "Excluiu usuário", color: "#DC2626", bg: "#FEF2F2" },
  DELIVERY_MAN_DELETED: { label: "Excluiu entregador", color: "#DC2626", bg: "#FEF2F2" },
  ESTABLISHMENT_DELETED: { label: "Excluiu estabelecimento", color: "#DC2626", bg: "#FEF2F2" },
};

const ACTION_KEYS = Object.keys(ACTION_META);

function actionMeta(action) {
  return ACTION_META[action] || { label: action || "-", color: "#4B5563", bg: "#F3F4F6" };
}

function formatDetails(details) {
  if (!details) return "-";
  try {
    const obj = JSON.parse(details);
    return Object.entries(obj)
      .map(([k, v]) => `${k}: ${typeof v === "object" ? JSON.stringify(v) : v}`)
      .join(" · ");
  } catch {
    return details;
  }
}

export default function Audit() {
  const [entries, setEntries] = useState([]);
  const [loading, setLoading] = useState(true);
  const [actionFilter, setActionFilter] = useState("");
  const [adminFilter, setAdminFilter] = useState("");
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [total, setTotal] = useState(0);
  const [totalPages, setTotalPages] = useState(0);

  const loadAudit = async (opts = {}) => {
    const params = new URLSearchParams();
    params.set("page", opts.page ?? page);
    params.set("limit", opts.pageSize ?? pageSize);
    if (opts.action ?? actionFilter) params.set("action", opts.action ?? actionFilter);
    if (opts.admin ?? adminFilter) params.set("admin", opts.admin ?? adminFilter);
    try {
      const { data } = await api.get(`/audit-log?${params.toString()}`);
      setEntries(data?.data || []);
      setTotal(data?.total || 0);
      setTotalPages(data?.total_pages || 0);
    } catch (e) {
      console.error(e);
      toast.error("Erro ao carregar auditoria");
    }
    setLoading(false);
  };

  useEffect(() => { loadAudit(); }, [page, pageSize, actionFilter, adminFilter]);

  const refresh = () => { setLoading(true); loadAudit(); };
  const changePageSize = (n) => { setPageSize(n); setPage(1); };

  if (loading) {
    return (
      <div className="animate-fade-in space-y-6">
        <div className="space-y-2">
          <div className="skeleton h-8 w-44" />
          <div className="skeleton h-4 w-64" />
        </div>
        <div className="card">
          <div className="p-4"><div className="skeleton h-10 w-full" /></div>
          <div className="p-6 space-y-3">
            {[...Array(8)].map((_, i) => <div key={i} className="skeleton h-9 w-full" />)}
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="animate-fade-in space-y-6 min-w-0">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Auditoria</h1>
          <p className="text-gray-500 mt-1">
            Quem fez o quê, quando e de onde — {total} registro(s)
          </p>
        </div>
        <button onClick={refresh} className="btn btn-ghost" title="Atualizar">
          <FiRefreshCw className="h-4 w-4" /> Atualizar
        </button>
      </div>

      <div className="card p-4">
        <div className="flex flex-col sm:flex-row gap-4">
          <div className="flex-1 relative">
            <FiShield className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" />
            <input
              type="text"
              placeholder="Buscar por admin (nome ou e-mail)"
              value={adminFilter}
              onChange={(e) => { setAdminFilter(e.target.value); setPage(1); }}
              className="input pl-10"
            />
          </div>
          <div className="relative">
            <FiFilter className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" />
            <select
              value={actionFilter}
              onChange={(e) => { setActionFilter(e.target.value); setPage(1); }}
              className="input w-56 pl-10"
            >
              <option value="">Todas as ações</option>
              {ACTION_KEYS.map((k) => (
                <option key={k} value={k}>{ACTION_META[k].label}</option>
              ))}
            </select>
          </div>
        </div>
      </div>

      <div className="card overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-6 py-2 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Quando</th>
                <th className="px-6 py-2 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Admin</th>
                <th className="px-6 py-2 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Ação</th>
                <th className="px-6 py-2 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Recurso</th>
                <th className="px-6 py-2 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Detalhes</th>
                <th className="px-6 py-2 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">IP</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {entries.length === 0 ? (
                <tr>
                  <td colSpan={6} className="px-6 py-16 text-center text-gray-400">
                    Nenhuma ação registrada
                  </td>
                </tr>
              ) : entries.map((e) => {
                const meta = actionMeta(e.action);
                return (
                  <tr key={e.id} className="hover:bg-gray-50 transition-colors">
                    <td className="px-6 py-4 text-sm text-gray-500 whitespace-nowrap">
                      {e.created_at ? new Date(e.created_at).toLocaleString("pt-BR") : "-"}
                    </td>
                    <td className="px-6 py-4">
                      <p className="text-sm font-medium text-gray-900">{e.admin_name || "—"}</p>
                      <p className="text-xs text-gray-400">
                        {e.admin_email || (e.admin_user_id ? `#${e.admin_user_id}` : "")}
                      </p>
                    </td>
                    <td className="px-6 py-4">
                      <span
                        className="inline-flex items-center px-2.5 py-1 rounded-full text-xs font-semibold"
                        style={{ background: meta.bg, color: meta.color }}
                      >
                        {meta.label}
                      </span>
                    </td>
                    <td className="px-6 py-4">
                      <p className="text-xs font-medium text-gray-700">{e.resource_type || "-"}</p>
                      <p className="text-xs font-mono text-gray-400">{e.resource_id || "-"}</p>
                    </td>
                    <td className="px-6 py-4 text-xs text-gray-500 max-w-[320px] truncate" title={e.details}>
                      {formatDetails(e.details)}
                    </td>
                    <td className="px-6 py-4 text-xs font-mono text-gray-400">{e.ip || "-"}</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
        <Pagination
          page={page}
          totalPages={totalPages}
          total={total}
          pageSize={pageSize}
          onPageChange={setPage}
          onPageSizeChange={changePageSize}
        />
      </div>
    </div>
  );
}
