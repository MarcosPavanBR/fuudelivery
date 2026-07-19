import React, { useState, useEffect } from "react";
import { FiBuilding2, FiPlus, FiSearch, FiEdit, FiTrash2, FiEye, FiMapPin, FiClock, FiMoreVertical, FiFilter, FiActivity, FiX } from "react-icons/fi";
import api from "../services/api";
import { toast } from "react-toastify";

const statusOptions = [
  { value: "active", label: "Ativo", color: "bg-green-100 text-green-800" },
  { value: "inactive", label: "Inativo", color: "bg-gray-100 text-gray-800" },
  { value: "pending", label: "Pendente", color: "bg-yellow-100 text-yellow-800" },
];

export default function Establishments() {
  const [establishments, setEstablishments] = useState([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState(null);
  const [formData, setFormData] = useState({
    name: "",
    email: "",
    phone: "",
    address: "",
    city: "",
    state: "",
    zip_code: "",
    latitude: "",
    longitude: "",
    status: "active",
    delivery_fee: 0,
    min_order: 0,
    delivery_time: 30,
  });

  useEffect(() => {
    loadEstablishments();
  }, []);

  const loadEstablishments = async () => {
    try {
      const { data } = await api.get("/establishments");
      setEstablishments(data || []);
    } catch (e) {
      console.error(e);
      toast.error("Erro ao carregar estabelecimentos");
    }
    setLoading(false);
  };

  const filtered = establishments.filter((e) => {
    const matchesSearch = e.name?.toLowerCase().includes(search.toLowerCase()) ||
      e.email?.toLowerCase().includes(search.toLowerCase());
    const matchesStatus = !statusFilter || e.status === statusFilter;
    return matchesSearch && matchesStatus;
  });

  const handleSubmit = async (e) => {
    e.preventDefault();
    try {
      if (editing) {
        await api.put(`/establishments/${editing.id}`, formData);
        toast.success("Estabelecimento atualizado");
      } else {
        await api.post("/establishments", formData);
        toast.success("Estabelecimento criado");
      }
      setModalOpen(false);
      loadEstablishments();
    } catch (err) {
      toast.error("Erro ao salvar");
      console.error(err);
    }
  };

  const handleDelete = async (id) => {
    if (!confirm("Excluir este estabelecimento?")) return;
    try {
      await api.delete(`/establishments/${id}`);
      toast.success("Excluído");
      loadEstablishments();
    } catch (e) {
      toast.error("Erro ao excluir");
    }
  };

  const openEdit = (est) => {
    setEditing(est);
    setFormData({
      name: est.name || "",
      email: est.email || "",
      phone: est.phone || "",
      address: est.address || "",
      city: est.city || "",
      state: est.state || "",
      zip_code: est.zip_code || "",
      latitude: est.latitude || "",
      longitude: est.longitude || "",
      status: est.status || "active",
      delivery_fee: est.delivery_fee || 0,
      min_order: est.min_order || 0,
      delivery_time: est.delivery_time || 30,
    });
    setModalOpen(true);
  };

  const openNew = () => {
    setEditing(null);
    setFormData({ name: "", email: "", phone: "", address: "", city: "", state: "", zip_code: "", latitude: "", longitude: "", status: "active", delivery_fee: 0, min_order: 0, delivery_time: 30 });
    setModalOpen(true);
  };

  if (loading) return <div className="flex items-center justify-center h-64"><FiActivity className="animate-spin h-8 w-8 text-fuu-red" /></div>;

  return (
    <div className="animate-fade-in space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Estabelecimentos</h1>
          <p className="text-gray-500 mt-1">{filtered.length} de {establishments.length} estabelecimentos</p>
        </div>
        <button onClick={openNew} className="flex items-center gap-2 px-4 py-2 rounded-xl text-white font-semibold text-sm" style={{ background: "linear-gradient(135deg, #EA1D2C, #C41420)" }}>
          <FiPlus className="h-4 w-4" />
          Novo
        </button>
      </div>

      {/* Filters */}
      <div className="bg-white rounded-2xl shadow-card border border-gray-100 p-4">
        <div className="flex flex-col sm:flex-row gap-4">
          <div className="flex-1 relative">
            <FiSearch className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" />
            <input
              type="text"
              placeholder="Buscar por nome, email..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="w-full pl-10 pr-4 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white"
            />
          </div>
          <div className="relative">
            <FiFilter className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" />
            <select value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)} className="w-48 pl-10 pr-10 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white appearance-none">
              <option value="">Todos os status</option>
              {statusOptions.map((s) => <option key={s.value} value={s.value}>{s.label}</option>)}
            </select>
          </div>
        </div>
      </div>

      {/* Table */}
      <div className="bg-white rounded-2xl shadow-card border border-gray-100 overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Estabelecimento</th>
                <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Contato</th>
                <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Endereço</th>
                <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Status</th>
                <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Taxa / Mín</th>
                <th className="px-6 py-3 text-right text-xs font-semibold text-gray-500 uppercase tracking-wider">Ações</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {filtered.length === 0 ? (
                <tr><td colSpan={6} className="px-6 py-12 text-center text-gray-500">Nenhum estabelecimento encontrado</td></tr>
              ) : (
                filtered.map((est) => (
                  <tr key={est.id} className="hover:bg-gray-50 transition-colors">
                    <td className="px-6 py-4">
                      <div>
                        <p className="font-medium text-gray-900">{est.name}</p>
                        <p className="text-xs text-gray-500">ID: {est.id?.toString().slice(-8)}</p>
                      </div>
                    </td>
                    <td className="px-6 py-4">
                      <p className="text-sm text-gray-900">{est.email}</p>
                      <p className="text-xs text-gray-500">{est.phone}</p>
                    </td>
                    <td className="px-6 py-4 text-sm text-gray-600">
                      {est.address}, {est.city} - {est.state}
                    </td>
                    <td className="px-6 py-4">
                      <span className={`inline-flex items-center px-2.5 py-1 rounded-full text-xs font-medium ${statusOptions.find(s => s.value === est.status)?.color || "bg-gray-100 text-gray-800"}`}>
                        {statusOptions.find(s => s.value === est.status)?.label || est.status}
                      </span>
                    </td>
                    <td className="px-6 py-4 text-sm text-gray-600">
                      R$ {est.delivery_fee?.toFixed(2)} / R$ {est.min_order?.toFixed(2)}
                    </td>
                    <td className="px-6 py-4">
                      <div className="flex items-center gap-2 justify-end">
                        <button onClick={() => openEdit(est)} className="p-2 text-gray-400 hover:text-fuu-red hover:bg-fuu-red-light rounded-lg transition-colors" title="Editar"><FiEdit className="h-4 w-4" /></button>
                        <button onClick={() => handleDelete(est.id)} className="p-2 text-gray-400 hover:text-red-500 hover:bg-red-50 rounded-lg transition-colors" title="Excluir"><FiTrash2 className="h-4 w-4" /></button>
                      </div>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>

      {/* Modal */}
      {modalOpen && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4 animate-fade-in">
          <div className="bg-white rounded-2xl w-full max-w-2xl max-h-[90vh] overflow-hidden shadow-modal animate-slide-up">
            <div className="flex items-center justify-between px-6 py-4 border-b border-gray-100">
              <h2 className="text-lg font-bold text-gray-900">{editing ? "Editar" : "Novo"} Estabelecimento</h2>
              <button onClick={() => { setModalOpen(false); setEditing(null); }} className="p-2 rounded-xl hover:bg-gray-100"><FiX className="h-5 w-5 text-gray-500" /></button>
            </div>
            <form onSubmit={handleSubmit} className="p-6 overflow-y-auto max-h-[70vh] space-y-4">
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div><label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">Nome *</label><input required name="name" value={formData.name} onChange={e => setFormData({...formData, name: e.target.value})} className="w-full px-4 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white" /></div>
                <div><label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">Email *</label><input required type="email" name="email" value={formData.email} onChange={e => setFormData({...formData, email: e.target.value})} className="w-full px-4 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white" /></div>
                <div><label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">Telefone</label><input name="phone" value={formData.phone} onChange={e => setFormData({...formData, phone: e.target.value})} className="w-full px-4 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white" /></div>
                <div><label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">Status</label><select name="status" value={formData.status} onChange={e => setFormData({...formData, status: e.target.value})} className="w-full px-4 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white">{statusOptions.map(s => <option key={s.value} value={s.value}>{s.label}</option>)}</select></div>
                <div className="sm:col-span-2"><label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">Endereço</label><input name="address" value={formData.address} onChange={e => setFormData({...formData, address: e.target.value})} className="w-full px-4 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white" /></div>
                <div><label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">Cidade</label><input name="city" value={formData.city} onChange={e => setFormData({...formData, city: e.target.value})} className="w-full px-4 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white" /></div>
                <div><label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">Estado</label><input name="state" value={formData.state} onChange={e => setFormData({...formData, state: e.target.value})} className="w-full px-4 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white" /></div>
                <div><label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">CEP</label><input name="zip_code" value={formData.zip_code} onChange={e => setFormData({...formData, zip_code: e.target.value})} className="w-full px-4 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white" /></div>
                <div><label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">Latitude</label><input type="number" step="any" name="latitude" value={formData.latitude} onChange={e => setFormData({...formData, latitude: e.target.value})} className="w-full px-4 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white" /></div>
                <div><label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">Longitude</label><input type="number" step="any" name="longitude" value={formData.longitude} onChange={e => setFormData({...formData, longitude: e.target.value})} className="w-full px-4 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white" /></div>
                <div><label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">Taxa Entrega (R$)</label><input type="number" step="0.01" name="delivery_fee" value={formData.delivery_fee} onChange={e => setFormData({...formData, delivery_fee: parseFloat(e.target.value)})} className="w-full px-4 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white" /></div>
                <div><label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">Valor Mínimo (R$)</label><input type="number" step="0.01" name="min_order" value={formData.min_order} onChange={e => setFormData({...formData, min_order: parseFloat(e.target.value)})} className="w-full px-4 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white" /></div>
                <div><label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">Tempo Entrega (min)</label><input type="number" name="delivery_time" value={formData.delivery_time} onChange={e => setFormData({...formData, delivery_time: parseInt(e.target.value)})} className="w-full px-4 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white" /></div>
              </div>
              <div className="flex justify-end gap-3 pt-4 border-t border-gray-100">
                <button type="button" onClick={() => { setModalOpen(false); setEditing(null); }} className="px-5 py-2.5 rounded-xl text-sm font-medium text-gray-700 bg-white border border-gray-200 hover:bg-gray-50">Cancelar</button>
                <button type="submit" className="flex items-center gap-2 px-5 py-2.5 rounded-xl text-sm font-medium text-white transition-all" style={{ background: "linear-gradient(135deg, #EA1D2C, #C41420)" }}><FiPlus className="h-4 w-4" />{editing ? " Atualizar" : " Criar"}</button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}