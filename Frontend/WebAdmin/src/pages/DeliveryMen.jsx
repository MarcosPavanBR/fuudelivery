import React, { useState, useEffect } from "react";
import { FiTruck, FiPlus, FiSearch, FiFilter, FiEdit, FiTrash2, FiActivity, FiX } from "react-icons/fi";
import api from "../services/api";
import { toast } from "react-toastify";

const statusOptions = [
  { value: "online", label: "Online", color: "bg-green-100 text-green-800" },
  { value: "offline", label: "Offline", color: "bg-gray-100 text-gray-800" },
  { value: "busy", label: "Ocupado", color: "bg-yellow-100 text-yellow-800" },
  { value: "on_delivery", label: "Em Entrega", color: "bg-blue-100 text-blue-800" },
];

const vehicleOptions = [
  { value: "moto", label: "Moto" },
  { value: "car", label: "Carro" },
  { value: "bicycle", label: "Bicicleta" },
  { value: "scooter", label: "Patinete" },
];

export default function DeliveryMen() {
  const [drivers, setDrivers] = useState([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState(null);
  const [formData, setFormData] = useState({ name: "", email: "", phone: "", vehicle_type: "moto", vehicle_plate: "", cnh: "", cpf: "", status: "offline" });

  useEffect(() => { loadDrivers(); }, []);

  const loadDrivers = async () => {
    try {
      const { data } = await api.get("/delivery-man");
      setDrivers(data || []);
    } catch (e) { console.error(e); toast.error("Erro ao carregar entregadores"); }
    setLoading(false);
  };

  const filtered = drivers.filter(d => {
    const matchesSearch = d.name?.toLowerCase().includes(search.toLowerCase()) || d.email?.toLowerCase().includes(search.toLowerCase());
    const matchesStatus = !statusFilter || d.status === statusFilter;
    return matchesSearch && matchesStatus;
  });

  const handleSubmit = async (e) => {
    e.preventDefault();
    try {
      if (editing) { await api.put(`/delivery-man/${editing.id}`, formData); toast.success("Atualizado"); }
      else { await api.post("/delivery-man", formData); toast.success("Criado"); }
      setModalOpen(false); setEditing(null); loadDrivers();
    } catch (err) { toast.error("Erro ao salvar"); console.error(err); }
  };

  const handleDelete = async (id) => {
    if (!confirm("Excluir entregador?")) return;
    try { await api.delete(`/delivery-man/${id}`); toast.success("Excluido"); loadDrivers(); }
    catch (e) { toast.error("Erro ao excluir"); }
  };

  const openEdit = (d) => {
    setEditing(d);
    setFormData({ name: d.name, email: d.email, phone: d.phone, vehicle_type: d.vehicle_type, vehicle_plate: d.vehicle_plate, cnh: d.cnh, cpf: d.cpf, status: d.status });
    setModalOpen(true);
  };
  const openNew = () => {
    setEditing(null);
    setFormData({ name: "", email: "", phone: "", vehicle_type: "moto", vehicle_plate: "", cnh: "", cpf: "", status: "offline" });
    setModalOpen(true);
  };

  if (loading) return <div className="flex items-center justify-center h-64"><FiActivity className="animate-spin h-8 w-8 text-fuu-red" /></div>;

  return (
    <div className="animate-fade-in space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Entregadores</h1>
          <p className="text-gray-500 mt-1">{filtered.length} de {drivers.length}</p>
        </div>
        <button onClick={openNew} className="flex items-center gap-2 px-4 py-2 rounded-xl text-white font-semibold text-sm" style={{ background: "linear-gradient(135deg, #EA1D2C, #C41420)" }}>
          <FiPlus className="h-4 w-4" />Novo
        </button>
      </div>

      <div className="bg-white rounded-2xl shadow-card border border-gray-100 p-4">
        <div className="flex flex-col sm:flex-row gap-4">
          <div className="flex-1 relative">
            <FiSearch className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" />
            <input type="text" placeholder="Buscar..." value={search} onChange={e => setSearch(e.target.value)} className="w-full pl-10 pr-4 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white" />
          </div>
          <div className="relative">
            <FiFilter className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" />
            <select value={statusFilter} onChange={e => setStatusFilter(e.target.value)} className="w-44 pl-10 pr-10 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white appearance-none">
              <option value="">Todos status</option>
              {statusOptions.map(s => <option key={s.value} value={s.value}>{s.label}</option>)}
            </select>
          </div>
        </div>
      </div>

      <div className="bg-white rounded-2xl shadow-card border border-gray-100 overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Entregador</th>
                <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Contato</th>
                <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Veiculo</th>
                <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Status</th>
                <th className="px-6 py-3 text-right text-xs font-semibold text-gray-500 uppercase tracking-wider">Acoes</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {filtered.length === 0 ? (
                <tr><td colSpan={5} className="px-6 py-12 text-center text-gray-500">Nenhum entregador</td></tr>
              ) : filtered.map(d => (
                <tr key={d.id} className="hover:bg-gray-50">
                  <td className="px-6 py-4">
                    <div className="flex items-center gap-3">
                      <div className="w-10 h-10 rounded-full bg-blue-100 flex items-center justify-center"><FiTruck className="h-5 w-5 text-blue-600" /></div>
                      <div><p className="font-medium text-gray-900">{d.name}</p><p className="text-xs text-gray-500">ID: {d.id?.toString().slice(-6)}</p></div>
                    </div>
                  </td>
                  <td className="px-6 py-4"><p className="text-sm text-gray-900">{d.email}</p><p className="text-xs text-gray-500">{d.phone}</p></td>
                  <td className="px-6 py-4 text-sm text-gray-600">{d.vehicle_type} - {d.vehicle_plate}</td>
                  <td className="px-6 py-4"><span className="inline-flex items-center px-2.5 py-1 rounded-full text-xs font-medium bg-green-100 text-green-800">{d.status}</span></td>
                  <td className="px-6 py-4">
                    <div className="flex items-center gap-2 justify-end">
                      <button onClick={() => openEdit(d)} className="p-2 text-gray-400 hover:text-fuu-red hover:bg-fuu-red-light rounded-lg"><FiEdit className="h-4 w-4" /></button>
                      <button onClick={() => handleDelete(d.id)} className="p-2 text-gray-400 hover:text-red-500 hover:bg-red-50 rounded-lg"><FiTrash2 className="h-4 w-4" /></button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {modalOpen && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4 animate-fade-in">
          <div className="bg-white rounded-2xl w-full max-w-lg max-h-[90vh] overflow-hidden shadow-modal animate-slide-up">
            <div className="flex items-center justify-between px-6 py-4 border-b border-gray-100">
              <h2 className="text-lg font-bold text-gray-900">{editing ? "Editar" : "Novo"} Entregador</h2>
              <button onClick={() => { setModalOpen(false); setEditing(null); }} className="p-2 rounded-xl hover:bg-gray-100"><FiX className="h-5 w-5 text-gray-500" /></button>
            </div>
            <form onSubmit={handleSubmit} className="p-6 space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div className="col-span-2">
                  <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">Nome *</label>
                  <input required name="name" value={formData.name} onChange={e => setFormData({...formData, name: e.target.value})} className="w-full px-4 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white" />
                </div>
                <div>
                  <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">Email *</label>
                  <input required type="email" name="email" value={formData.email} onChange={e => setFormData({...formData, email: e.target.value})} className="w-full px-4 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white" />
                </div>
                <div>
                  <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">Telefone *</label>
                  <input required name="phone" value={formData.phone} onChange={e => setFormData({...formData, phone: e.target.value})} className="w-full px-4 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white" />
                </div>
                <div>
                  <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">Veiculo</label>
                  <select name="vehicle_type" value={formData.vehicle_type} onChange={e => setFormData({...formData, vehicle_type: e.target.value})} className="w-full px-4 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white">
                    {vehicleOptions.map(v => <option key={v.value} value={v.value}>{v.label}</option>)}
                  </select>
                </div>
                <div>
                  <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">Placa</label>
                  <input name="vehicle_plate" value={formData.vehicle_plate} onChange={e => setFormData({...formData, vehicle_plate: e.target.value})} className="w-full px-4 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white" />
                </div>
                <div>
                  <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">CNH</label>
                  <input name="cnh" value={formData.cnh} onChange={e => setFormData({...formData, cnh: e.target.value})} className="w-full px-4 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white" />
                </div>
                <div>
                  <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">CPF</label>
                  <input name="cpf" value={formData.cpf} onChange={e => setFormData({...formData, cpf: e.target.value})} className="w-full px-4 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white" />
                </div>
              </div>
              <div className="flex justify-end gap-3 pt-4 border-t border-gray-100">
                <button type="button" onClick={() => { setModalOpen(false); setEditing(null); }} className="px-5 py-2.5 rounded-xl text-sm font-medium text-gray-700 bg-white border border-gray-200 hover:bg-gray-50">Cancelar</button>
                <button type="submit" className="flex items-center gap-2 px-5 py-2.5 rounded-xl text-sm font-medium text-white transition-all" style={{ background: "linear-gradient(135deg, #EA1D2C, #C41420)" }}>
                  <FiPlus className="h-4 w-4" />{editing ? " Atualizar" : " Criar"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
