import React, { useState, useEffect } from "react";
import { FiShoppingBag, FiSearch, FiFilter, FiEye, FiActivity, FiX } from "react-icons/fi";
import api from "../services/api";
import { toast } from "react-toastify";

const statusColors = {
  pending: { bg: "#FEF3C7", text: "#B45309", label: "Pendente" },
  approved: { bg: "#DBEAFE", text: "#1D4ED8", label: "Aprovado" },
  preparing: { bg: "#FEF3C7", text: "#B45309", label: "Preparando" },
  ready: { bg: "#DBEAFE", text: "#1D4ED8", label: "Pronto" },
  delivering: { bg: "#FEF3C7", text: "#B45309", label: "Em Rota" },
  delivered: { bg: "#ECFDF5", text: "#047857", label: "Entregue" },
  cancelled: { bg: "#FEE2E2", text: "#B91C1C", label: "Cancelado" },
};

export default function Orders() {
  const [orders, setOrders] = useState([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [modalOpen, setModalOpen] = useState(false);
  const [selectedOrder, setSelectedOrder] = useState(null);

  useEffect(() => { loadOrders(); }, []);

  const loadOrders = async () => {
    try {
      const { data } = await api.get("/orders");
      setOrders(data || []);
    } catch (e) { console.error(e); toast.error("Erro ao carregar pedidos"); }
    setLoading(false);
  };

  const filtered = orders.filter(o => {
    const matchesSearch = o.id?.toString().includes(search) || o.user?.nome?.toLowerCase().includes(search.toLowerCase());
    const matchesStatus = !statusFilter || o.status === statusFilter;
    return matchesSearch && matchesStatus;
  });

  if (loading) return <div className="flex items-center justify-center h-64"><FiActivity className="animate-spin h-8 w-8 text-fuu-red" /></div>;

  return (
    <div className="animate-fade-in space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Pedidos</h1>
          <p className="text-gray-500 mt-1">{filtered.length} de {orders.length} pedidos</p>
        </div>
      </div>

      <div className="bg-white rounded-2xl shadow-card border border-gray-100 p-4">
        <div className="flex flex-col sm:flex-row gap-4">
          <div className="flex-1 relative">
            <FiSearch className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" />
            <input type="text" placeholder="Buscar por ID, cliente..." value={search} onChange={e => setSearch(e.target.value)} className="w-full pl-10 pr-4 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white" />
          </div>
          <div className="relative">
            <FiFilter className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" />
            <select value={statusFilter} onChange={e => setStatusFilter(e.target.value)} className="w-44 pl-10 pr-10 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white appearance-none">
              <option value="">Todos status</option>
              {Object.entries(statusColors).map(([k, v]) => <option key={k} value={k}>{v.label}</option>)}
            </select>
          </div>
        </div>
      </div>

      <div className="bg-white rounded-2xl shadow-card border border-gray-100 overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Pedido</th>
                <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Cliente</th>
                <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Restaurante</th>
                <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Status</th>
                <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Total</th>
                <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Entregador</th>
                <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Criado</th>
                <th className="px-6 py-3 text-right text-xs font-semibold text-gray-500 uppercase tracking-wider">Acoes</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {filtered.length === 0 ? (
                <tr><td colSpan={8} className="px-6 py-12 text-center text-gray-500">Nenhum pedido encontrado</td></tr>
              ) : filtered.map(order => {
                const sc = statusColors[order.status] || { bg: "#F3F4F6", text: "#4B5563", label: order.status };
                return (
                  <tr key={order.id} className="hover:bg-gray-50">
                    <td className="px-6 py-4"><span className="font-medium text-gray-900">#{order.id?.toString().slice(-8)}</span></td>
                    <td className="px-6 py-4">
                      <div className="flex items-center gap-2">
                        <div className="w-8 h-8 rounded-full bg-fuu-red-light flex items-center justify-center">
                          <span className="text-xs font-bold text-fuu-red">{order.user?.nome?.charAt(0) || "C"}</span>
                        </div>
                        <span className="text-sm text-gray-900">{order.user?.nome || "Cliente"}</span>
                      </div>
                    </td>
                    <td className="px-6 py-4 text-sm text-gray-600">{order.establishment?.name || "-"}</td>
                    <td className="px-6 py-4">
                      <span className="inline-flex items-center px-2.5 py-1 rounded-full text-xs font-medium" style={{ background: sc.bg, color: sc.text }}>{sc.label}</span>
                    </td>
                    <td className="px-6 py-4 font-semibold text-gray-900">R$ {order.total?.toFixed(2) || "0,00"}</td>
                    <td className="px-6 py-4 text-sm text-gray-600">{order.deliveryman?.name || "-"}</td>
                    <td className="px-6 py-4 text-sm text-gray-500">{order.createdAt ? new Date(order.createdAt).toLocaleString("pt-BR") : "-"}</td>
                    <td className="px-6 py-4">
                      <div className="flex items-center gap-2 justify-end">
                        <button onClick={() => { setSelectedOrder(order); setModalOpen(true); }} className="p-2 text-gray-400 hover:text-fuu-red hover:bg-fuu-red-light rounded-lg"><FiEye className="h-4 w-4" /></button>
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </div>

      {modalOpen && selectedOrder && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4 animate-fade-in">
          <div className="bg-white rounded-2xl w-full max-w-2xl max-h-[90vh] overflow-hidden shadow-modal animate-slide-up">
            <div className="flex items-center justify-between px-6 py-4 border-b border-gray-100">
              <h2 className="text-lg font-bold text-gray-900">Detalhes do Pedido #{selectedOrder.id?.toString().slice(-8)}</h2>
              <button onClick={() => { setModalOpen(false); setSelectedOrder(null); }} className="p-2 rounded-xl hover:bg-gray-100"><FiX className="h-5 w-5 text-gray-500" /></button>
            </div>
            <div className="p-6 overflow-y-auto max-h-[70vh]">
              <div className="grid grid-cols-2 gap-4 mb-6">
                <div><p className="text-xs font-semibold text-gray-500 uppercase">ID</p><p className="font-medium text-gray-900">#{selectedOrder.id}</p></div>
                <div><p className="text-xs font-semibold text-gray-500 uppercase">Status</p><span className="inline-flex items-center px-2.5 py-1 rounded-full text-xs font-medium" style={{ background: (statusColors[selectedOrder.status] || { bg: "#F3F4F6" }).bg, color: (statusColors[selectedOrder.status] || { text: "#4B5563" }).text }}>{(statusColors[selectedOrder.status] || { label: selectedOrder.status }).label}</span></div>
                <div><p className="text-xs font-semibold text-gray-500 uppercase">Total</p><p className="font-bold text-2xl text-gray-900">R$ {selectedOrder.total?.toFixed(2)}</p></div>
                <div><p className="text-xs font-semibold text-gray-500 uppercase">Criado em</p><p className="font-medium text-gray-900">{selectedOrder.createdAt ? new Date(selectedOrder.createdAt).toLocaleString("pt-BR") : "-"}</p></div>
              </div>
              <h3 className="font-semibold text-gray-900 mb-3">Itens</h3>
              <div className="space-y-2">
                {selectedOrder.items?.map((item, i) => (
                  <div key={i} className="flex items-center justify-between p-3 bg-gray-50 rounded-xl">
                    <div><p className="font-medium text-gray-900">{item.name || item.product?.name || "Item"}</p><p className="text-xs text-gray-500">Qtd: {item.quantity || 1}</p></div>
                    <p className="font-semibold text-gray-900">R$ {(item.price || item.subtotal || 0).toFixed(2)}</p>
                  </div>
                ))}
                {(!selectedOrder.items || selectedOrder.items.length === 0) && <p className="text-sm text-gray-500">Sem itens detalhados</p>}
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
