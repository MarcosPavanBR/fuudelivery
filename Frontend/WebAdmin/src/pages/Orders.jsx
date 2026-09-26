import React, { useState, useEffect } from "react";
import { FiSearch, FiFilter, FiEye, FiActivity, FiX, FiRefreshCw, FiPhone } from "react-icons/fi";
import api from "../services/api";
import { toast } from "react-toastify";
import { normalizeOrder, statusInfo, STATUS_FILTER, money } from "../helpers/orders";

// Status em que o pedido ainda pode ser cancelado (orders.go validTransitions).
const CANCELLABLE = ["AWAIT_APPROVE", "REQUEST_APPROVE", "SCHEDULED", "APPROVED", "PREPARING"];

function StatusPill({ status }) {
  const sc = statusInfo(status);
  return <span className="inline-flex items-center px-2.5 py-1 rounded-full text-xs font-medium whitespace-nowrap" style={{ background: sc.bg, color: sc.text }}>{sc.label}</span>;
}

const fmtDate = (d) => (d ? new Date(d).toLocaleString("pt-BR", { day: "2-digit", month: "2-digit", hour: "2-digit", minute: "2-digit" }) : "-");

export default function Orders() {
  const [orders, setOrders] = useState([]);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [selected, setSelected] = useState(null);

  useEffect(() => { loadOrders(); }, []);

  const loadOrders = async () => {
    setRefreshing(true);
    try {
      const { data } = await api.get("/orders/all");
      setOrders((Array.isArray(data) ? data : []).map(normalizeOrder));
    } catch (e) { console.error(e); toast.error("Erro ao carregar pedidos"); }
    setLoading(false);
    setRefreshing(false);
  };

  const term = search.trim().toLowerCase();
  const filtered = orders.filter((o) => {
    const matchesSearch = !term || [o.id, o.customer, o.phone, o.establishment].some((v) => String(v).toLowerCase().includes(term));
    const matchesStatus = !statusFilter || o.status === statusFilter || (statusFilter === "AWAIT_APPROVE" && o.status === "REQUEST_APPROVE");
    return matchesSearch && matchesStatus;
  });

  const handleCancel = async (order) => {
    if (!confirm(`Cancelar o pedido #${order.id.slice(-8)} de ${order.customer}? A loja e o cliente serão avisados.`)) return;
    try {
      await api.put("/orders/status", { id: order.id, status: "CANCELLED" });
      toast.success("Pedido cancelado");
      setSelected(null);
      loadOrders();
    } catch (e) {
      toast.error(e.response?.data?.error || "Não foi possível cancelar");
    }
  };

  if (loading) return <div className="flex items-center justify-center h-64"><FiActivity className="animate-spin h-8 w-8 text-fuu-red" /></div>;

  return (
    <div className="animate-fade-in space-y-6">
      <div className="flex items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Pedidos</h1>
          <p className="text-gray-500 mt-1">{filtered.length} de {orders.length} pedidos (últimos 500)</p>
        </div>
        <button onClick={loadOrders} disabled={refreshing} className="flex items-center gap-2 px-4 py-2 rounded-xl text-sm font-medium text-gray-700 bg-white border border-gray-200 hover:bg-gray-50 disabled:opacity-60">
          <FiRefreshCw className={`h-4 w-4 ${refreshing ? "animate-spin" : ""}`} /> Atualizar
        </button>
      </div>

      <div className="bg-white rounded-xl shadow-card border border-gray-100 p-4">
        <div className="flex flex-col sm:flex-row gap-4">
          <div className="flex-1 relative">
            <FiSearch className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" />
            <input type="text" placeholder="Buscar por nº do pedido, cliente, telefone, restaurante..." value={search} onChange={(e) => setSearch(e.target.value)} className="w-full pl-10 pr-4 py-2.5 bg-gray-50 border border-gray-200 rounded-lg text-sm focus:bg-white" />
          </div>
          <div className="relative">
            <FiFilter className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" />
            <select value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)} className="w-full sm:w-52 pl-10 pr-10 py-2.5 bg-gray-50 border border-gray-200 rounded-lg text-sm focus:bg-white appearance-none">
              <option value="">Todos os status</option>
              {STATUS_FILTER.map((k) => <option key={k} value={k}>{statusInfo(k).label}</option>)}
            </select>
          </div>
        </div>
      </div>

      <div className="bg-white rounded-xl shadow-card border border-gray-100 overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-6 py-2 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Pedido</th>
                <th className="px-6 py-2 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Cliente</th>
                <th className="px-6 py-2 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Restaurante</th>
                <th className="px-6 py-2 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Status</th>
                <th className="px-6 py-2 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Total</th>
                <th className="px-6 py-2 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Pagamento</th>
                <th className="px-6 py-2 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Criado</th>
                <th className="px-6 py-2 text-right text-xs font-semibold text-gray-500 uppercase tracking-wider">Ações</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {filtered.length === 0 ? (
                <tr><td colSpan={8} className="px-6 py-12 text-center text-gray-500">Nenhum pedido encontrado</td></tr>
              ) : filtered.map((order) => (
                <tr key={order.id} className="hover:bg-gray-50 cursor-pointer" onClick={() => setSelected(order)}>
                  <td className="px-6 py-4"><span className="font-mono text-sm font-medium text-gray-900">#{order.id.slice(-8)}</span></td>
                  <td className="px-6 py-4">
                    <div className="flex items-center gap-2">
                      <div className="w-8 h-8 rounded-full bg-fuu-red-light flex items-center justify-center flex-shrink-0">
                        <span className="text-xs font-bold text-fuu-red">{order.customer.charAt(0).toUpperCase()}</span>
                      </div>
                      <span className="text-sm text-gray-900">{order.customer}</span>
                    </div>
                  </td>
                  <td className="px-6 py-4 text-sm text-gray-600">{order.establishment}</td>
                  <td className="px-6 py-4"><StatusPill status={order.status} /></td>
                  <td className="px-6 py-4 font-semibold text-gray-900 whitespace-nowrap">{money(order.total)}</td>
                  <td className="px-6 py-4 text-sm text-gray-600">{order.payment}</td>
                  <td className="px-6 py-4 text-sm text-gray-500 whitespace-nowrap">{fmtDate(order.createdAt)}</td>
                  <td className="px-6 py-4">
                    <div className="flex items-center gap-2 justify-end">
                      <button onClick={(e) => { e.stopPropagation(); setSelected(order); }} className="p-2 text-gray-400 hover:text-fuu-red hover:bg-fuu-red-light rounded-lg" title="Detalhes"><FiEye className="h-4 w-4" /></button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {selected && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4 animate-fade-in" onClick={() => setSelected(null)}>
          <div className="bg-white rounded-xl w-full max-w-2xl max-h-[90vh] overflow-hidden shadow-modal animate-slide-up" onClick={(e) => e.stopPropagation()}>
            <div className="flex items-center justify-between px-6 py-4 border-b border-gray-100">
              <h2 className="text-lg font-bold text-gray-900">Pedido #{selected.id.slice(-8)}</h2>
              <button onClick={() => setSelected(null)} className="p-2 rounded-xl hover:bg-gray-100"><FiX className="h-5 w-5 text-gray-500" /></button>
            </div>
            <div className="p-6 overflow-y-auto max-h-[70vh] space-y-6">
              <div className="grid grid-cols-2 gap-4">
                <div><p className="text-xs font-semibold text-gray-500 uppercase">Status</p><div className="mt-1"><StatusPill status={selected.status} /></div></div>
                <div><p className="text-xs font-semibold text-gray-500 uppercase">Total</p><p className="font-bold text-2xl text-gray-900">{money(selected.total)}</p></div>
                <div><p className="text-xs font-semibold text-gray-500 uppercase">Criado em</p><p className="font-medium text-gray-900">{fmtDate(selected.createdAt)}</p></div>
                <div><p className="text-xs font-semibold text-gray-500 uppercase">Pagamento</p><p className="font-medium text-gray-900">{selected.payment}</p></div>
                <div><p className="text-xs font-semibold text-gray-500 uppercase">Restaurante</p><p className="font-medium text-gray-900">{selected.establishment}</p></div>
                <div><p className="text-xs font-semibold text-gray-500 uppercase">Entregador</p><p className="font-medium text-gray-900">{selected.deliveryman || "—"}</p></div>
                {selected.scheduledAt && <div><p className="text-xs font-semibold text-gray-500 uppercase">Agendado para</p><p className="font-medium text-gray-900">{fmtDate(selected.scheduledAt)}</p></div>}
                {selected.pickupCode && <div><p className="text-xs font-semibold text-gray-500 uppercase">Código de retirada</p><p className="font-mono font-medium text-gray-900">{selected.pickupCode}</p></div>}
              </div>

              <div>
                <h3 className="font-semibold text-gray-900 mb-2">Cliente</h3>
                <p className="text-sm text-gray-900">{selected.customer}</p>
                {selected.phone && <a href={`tel:${selected.phone}`} className="text-sm text-gray-600 hover:text-fuu-red inline-flex items-center gap-1"><FiPhone className="h-3.5 w-3.5" />{selected.phone}</a>}
                {selected.address && <p className="text-sm text-gray-600 mt-1">{selected.address}</p>}
              </div>

              <div>
                <h3 className="font-semibold text-gray-900 mb-2">Itens</h3>
                <div className="space-y-2">
                  {selected.items.map((item, i) => (
                    <div key={i} className="flex items-start justify-between gap-4 p-3 bg-gray-50 rounded-lg">
                      <div>
                        <p className="font-medium text-gray-900">{item.quantity}× {item.name}</p>
                        {item.additionals.length > 0 && <p className="text-xs text-gray-500">+ {item.additionals.join(", ")}</p>}
                        {item.note && <p className="text-xs text-amber-700">Obs.: {item.note}</p>}
                      </div>
                      <p className="font-semibold text-gray-900 whitespace-nowrap">{money(item.subtotal)}</p>
                    </div>
                  ))}
                  {selected.items.length === 0 && <p className="text-sm text-gray-500">Sem itens detalhados</p>}
                  {selected.deliveryValue > 0 && (
                    <div className="flex justify-between px-3 text-sm text-gray-600"><span>Entrega</span><span>{money(selected.deliveryValue)}</span></div>
                  )}
                </div>
              </div>

              {CANCELLABLE.includes(selected.status) && (
                <div className="pt-4 border-t border-gray-100 flex justify-end">
                  <button onClick={() => handleCancel(selected)} className="px-4 py-2 rounded-xl text-sm font-medium text-red-600 border border-red-200 hover:bg-red-50">Cancelar pedido</button>
                </div>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
