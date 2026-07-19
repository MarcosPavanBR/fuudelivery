import React, { useState, useEffect } from "react";
import { FiCreditCard, FiDollarSign, FiActivity, FiFilter, FiEye, FiTrendingUp, FiDownload, FiClock, FiCheck } from "react-icons/fi";
import api from "../services/api";
import { toast } from "react-toastify";

const statusColors = { completed: { bg: "#ECFDF5", text: "#047857", label: "Concluído" }, pending: { bg: "#FEF3C7", text: "#B45309", label: "Pendente" }, failed: { bg: "#FEE2E2", text: "#B91C1C", label: "Falhou" }, refunded: { bg: "#F3F4F6", text: "#4B5563", label: "Reembolsado" } };

export default function Payments() {
  const [payments, setPayments] = useState([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [modalOpen, setModalOpen] = useState(false);
  const [selected, setSelected] = useState(null);
  const [stats, setStats] = useState({ total: 0, completed: 0, pending: 0, revenue: 0 });

  useEffect(() => { loadPayments(); }, []);

  const loadPayments = async () => {
    try {
      const { data } = await api.get("/payments");
      const list = data || [];
      setPayments(list);
      setStats({
        total: list.length,
        completed: list.filter(p => p.status === "completed").length,
        pending: list.filter(p => p.status === "pending").length,
        revenue: list.filter(p => p.status === "completed").reduce((sum, p) => sum + (p.amount || 0), 0),
      });
    } catch (e) { console.error(e); toast.error("Erro ao carregar pagamentos"); }
    setLoading(false);
  };

  const filtered = payments.filter(p => {
    const m = p.id?.toString().includes(search) || p.user?.nome?.toLowerCase().includes(search.toLowerCase());
    const s = !statusFilter || p.status === statusFilter;
    return m && s;
  });

  if (loading) return <div className="flex items-center justify-center h-64"><FiActivity className="animate-spin h-8 w-8 text-fuu-red" /></div>;

  return (
    <div className="animate-fade-in space-y-6">
      <div className="flex items-center justify-between"><div><h1 className="text-2xl font-bold text-gray-900">Pagamentos</h1><p className="text-gray-500 mt-1">{filtered.length} de {payments.length}</p></div></div>

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        {[{label:"Total Transações",value:stats.total,icon:FiCreditCard,color:"#EA1D2C",bg:"#FEF2F2"},{label:"Concluídos",value:stats.completed,icon:FiCheck,color:"#10B981",bg:"#ECFDF5"},{label:"Pendentes",value:stats.pending,icon:FiClock,color:"#F59E0B",bg:"#FFFBEB"},{label:"Receita",value:`R$ ${stats.revenue.toFixed(2)}`,icon:FiDollarSign,color:"#F7A11E",bg:"#FFFBEB"}].map((s,i)=><div key={i} className="bg-white rounded-2xl p-5 shadow-card hover:shadow-card-hover transition-all border border-gray-100"><div className="flex items-start justify-between"><div><p className="text-sm font-medium text-gray-500">{s.label}</p><p className="text-3xl font-bold mt-2 text-gray-900">{s.value}</p></div><div className="p-3 rounded-xl" style={{background:s.bg,color:s.color}}><s.icon className="h-6 w-6" /></div></div></div>)}
      </div>

      <div className="bg-white rounded-2xl shadow-card border border-gray-100 p-4"><div className="flex flex-col sm:flex-row gap-4"><div className="flex-1 relative"><FiSearch className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" /><input type="text" placeholder="Buscar por ID, cliente..." value={search} onChange={e=>setSearch(e.target.value)} className="w-full pl-10 pr-4 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white" /></div><div className="relative"><FiFilter className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" /><select value={statusFilter} onChange={e=>setStatusFilter(e.target.value)} className="w-44 pl-10 pr-10 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white appearance-none"><option value="">Todos status</option><option value="completed">Concluído</option><option value="pending">Pendente</option><option value="failed">Falhou</option><option value="refunded">Reembolsado</option></select></div></div></div>

      <div className="bg-white rounded-2xl shadow-card border border-gray-100 overflow-hidden">
        <div className="overflow-x-auto"><table className="w-full"><thead className="bg-gray-50"><tr><th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">ID</th><th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Cliente</th><th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Valor</th><th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Método</th><th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Status</th><th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Data</th><th className="px-6 py-3 text-right text-xs font-semibold text-gray-500 uppercase tracking-wider">Ações</th></tr></thead><tbody className="divide-y divide-gray-100">
          {filtered.length===0?<tr><td colSpan={7} className="px-6 py-12 text-center text-gray-500">Nenhum pagamento</td></tr>:filtered.map(p=>{const sc=statusColors[p.status]||{bg:"#F3F4F6",text:"#4B5563",label:p.status};return <tr key={p.id} className="hover:bg-gray-50"><td className="px-6 py-4"><span className="font-medium text-gray-900">#{p.id?.toString().slice(-8)}</span></td><td className="px-6 py-4"><p className="text-sm text-gray-900">{p.user?.nome||"Cliente"}</p></td><td className="px-6 py-4 font-semibold text-gray-900">R$ {p.amount?.toFixed(2)}</td><td className="px-6 py-4 text-sm text-gray-600">{p.method}</td><td className="px-6 py-4"><span className="inline-flex items-center px-2.5 py-1 rounded-full text-xs font-medium" style={{background:sc.bg,color:sc.text}}>{sc.label}</span></td><td className="px-6 py-4 text-sm text-gray-500">{p.createdAt?new Date(p.createdAt).toLocaleString("pt-BR"):"-"}</td><td className="px-6 py-4"><div className="flex items-center gap-2 justify-end"><button className="p-2 text-gray-400 hover:text-fuu-red hover:bg-fuu-red-light rounded-lg"><FiEye className="h-4 w-4" /></button></div></td></tr>;})}
        </tbody></table></div>
      </div>
    </div>
  );
}