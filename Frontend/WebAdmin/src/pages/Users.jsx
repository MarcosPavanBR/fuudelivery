import React, { useState, useEffect } from "react";
import { FiUsers, FiPlus, FiSearch, FiEdit, FiTrash2, FiFilter, FiActivity, FiX } from "react-icons/fi";
import api from "../services/api";
import { toast } from "react-toastify";

const roleOptions = [
  { value: "admin", label: "Admin", color: "bg-red-100 text-red-800" },
  { value: "restaurant", label: "Restaurante", color: "bg-blue-100 text-blue-800" },
  { value: "delivery", label: "Entregador", color: "bg-green-100 text-green-800" },
  { value: "client", label: "Cliente", color: "bg-purple-100 text-purple-800" },
];

const statusOptions = [
  { value: "active", label: "Ativo", color: "bg-green-100 text-green-800" },
  { value: "inactive", label: "Inativo", color: "bg-gray-100 text-gray-800" },
  { value: "pending", label: "Pendente", color: "bg-yellow-100 text-yellow-800" },
];

export default function Users() {
  const [users, setUsers] = useState([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");
  const [roleFilter, setRoleFilter] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState(null);
  const [formData, setFormData] = useState({ name: "", email: "", password: "", role: "client", status: "active", establishment_id: "" });
  const [establishments, setEstablishments] = useState([]);

  useEffect(() => {
    Promise.all([loadUsers(), loadEstablishments()]);
  }, []);

  const loadUsers = async () => {
    try {
      const { data } = await api.get("/users");
      setUsers(data || []);
    } catch (e) { console.error(e); toast.error("Erro ao carregar usuarios"); }
    setLoading(false);
  };

  const loadEstablishments = async () => {
    try {
      const { data } = await api.get("/establishments");
      setEstablishments(data || []);
    } catch (e) { console.error(e); }
  };

  const filtered = users.filter((u) => {
    const matchesSearch = u.name?.toLowerCase().includes(search.toLowerCase()) || u.email?.toLowerCase().includes(search.toLowerCase());
    const matchesRole = !roleFilter || u.role === roleFilter;
    const matchesStatus = !statusFilter || u.status === statusFilter;
    return matchesSearch && matchesRole && matchesStatus;
  });

  const handleSubmit = async (e) => {
    e.preventDefault();
    try {
      if (editing) {
        await api.put(`/users/${editing.id}`, formData);
        toast.success("Usuario atualizado");
      } else {
        await api.post("/users", formData);
        toast.success("Usuario criado");
      }
      setModalOpen(false);
      loadUsers();
    } catch (err) { toast.error("Erro ao salvar"); console.error(err); }
  };

  const handleDelete = async (id) => {
    if (!confirm("Excluir este usuario?")) return;
    try {
      await api.delete(`/users/${id}`);
      toast.success("Excluido");
      loadUsers();
    } catch (e) { toast.error("Erro ao excluir"); }
  };

  const openEdit = (u) => {
    setEditing(u);
    setFormData({ name: u.name || "", email: u.email || "", password: "", role: u.role || "client", status: u.status || "active", establishment_id: u.establishment_id || "" });
    setModalOpen(true);
  };

  const openNew = () => { setEditing(null); setFormData({ name: "", email: "", password: "", role: "client", status: "active", establishment_id: "" }); setModalOpen(true); };

  if (loading) return <div className="flex items-center justify-center h-64"><FiActivity className="animate-spin h-8 w-8 text-fuu-red" /></div>;

  return (
    <div className="animate-fade-in space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Usuarios</h1>
          <p className="text-gray-500 mt-1">{filtered.length} de {users.length} usuarios</p>
        </div>
        <button onClick={openNew} className="flex items-center gap-2 px-4 py-2 rounded-xl text-white font-semibold text-sm" style={{ background: "linear-gradient(135deg, #EA1D2C, #C41420)" }}>
          <FiPlus className="h-4 w-4" /> Novo
        </button>
      </div>

      <div className="bg-white rounded-2xl shadow-card border border-gray-100 p-4">
        <div className="flex flex-col sm:flex-row gap-4">
          <div className="flex-1 relative">
            <FiSearch className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" />
            <input type="text" placeholder="Buscar por nome, email..." value={search} onChange={e => setSearch(e.target.value)} className="w-full pl-10 pr-4 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white" />
          </div>
          <div className="relative">
            <FiFilter className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" />
            <select value={roleFilter} onChange={e => setRoleFilter(e.target.value)} className="w-44 pl-10 pr-10 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white appearance-none">
              <option value="">Todas as roles</option>
              {roleOptions.map(r => <option key={r.value} value={r.value}>{r.label}</option>)}
            </select>
          </div>
          <div className="relative">
            <FiFilter className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" />
            <select value={statusFilter} onChange={e => setStatusFilter(e.target.value)} className="w-40 pl-10 pr-10 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white appearance-none">
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
                <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Usuario</th>
                <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Role</th>
                <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Status</th>
                <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Estabelecimento</th>
                <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Criado em</th>
                <th className="px-6 py-3 text-right text-xs font-semibold text-gray-500 uppercase tracking-wider">Acoes</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {filtered.length === 0 ? (
                <tr><td colSpan={6} className="px-6 py-12 text-center text-gray-500">Nenhum usuario encontrado</td></tr>
              ) : filtered.map(u => (
                <tr key={u.id} className="hover:bg-gray-50">
                  <td className="px-6 py-4"><p className="font-medium text-gray-900">{u.name}</p><p className="text-xs text-gray-500">{u.email}</p></td>
                  <td className="px-6 py-4"><span className={`inline-flex items-center px-2.5 py-1 rounded-full text-xs font-medium ${roleOptions.find(r => r.value === u.role)?.color || "bg-gray-100 text-gray-800"}`}>{roleOptions.find(r => r.value === u.role)?.label || u.role}</span></td>
                  <td className="px-6 py-4"><span className={`inline-flex items-center px-2.5 py-1 rounded-full text-xs font-medium ${statusOptions.find(s => s.value === u.status)?.color || "bg-gray-100 text-gray-800"}`}>{statusOptions.find(s => s.value === u.status)?.label || u.status}</span></td>
                  <td className="px-6 py-4 text-sm text-gray-600">{establishments.find(e => e.id === u.establishment_id)?.name || "-"}</td>
                  <td className="px-6 py-4 text-sm text-gray-500">{u.createdAt ? new Date(u.createdAt).toLocaleDateString("pt-BR") : "-"}</td>
                  <td className="px-6 py-4">
                    <div className="flex items-center gap-2 justify-end">
                      <button onClick={() => openEdit(u)} className="p-2 text-gray-400 hover:text-fuu-red hover:bg-fuu-red-light rounded-lg"><FiEdit className="h-4 w-4" /></button>
                      <button onClick={() => handleDelete(u.id)} className="p-2 text-gray-400 hover:text-red-500 hover:bg-red-50 rounded-lg"><FiTrash2 className="h-4 w-4" /></button>
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
          <div className="bg-white rounded-2xl w-full max-w-md max-h-[90vh] overflow-hidden shadow-modal animate-slide-up">
            <div className="flex items-center justify-between px-6 py-4 border-b border-gray-100">
              <h2 className="text-lg font-bold text-gray-900">{editing ? "Editar" : "Novo"} Usuario</h2>
              <button onClick={() => { setModalOpen(false); setEditing(null); }} className="p-2 rounded-xl hover:bg-gray-100"><FiX className="h-5 w-5 text-gray-500" /></button>
            </div>
            <form onSubmit={handleSubmit} className="p-6 space-y-4">
              <div>
                <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">Nome *</label>
                <input required name="name" value={formData.name} onChange={e => setFormData({...formData, name: e.target.value})} className="w-full px-4 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white" />
              </div>
              <div>
                <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">Email *</label>
                <input required type="email" name="email" value={formData.email} onChange={e => setFormData({...formData, email: e.target.value})} className="w-full px-4 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white" />
              </div>
              <div>
                <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">Senha {editing ? "(deixe em branco para nao alterar)" : "*"}</label>
                <input type="password" name="password" value={formData.password} onChange={e => setFormData({...formData, password: e.target.value})} className="w-full px-4 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white" placeholder={editing ? "******" : "Minimo 6 caracteres"} />
              </div>
              <div>
                <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">Role *</label>
                <select name="role" value={formData.role} onChange={e => setFormData({...formData, role: e.target.value})} className="w-full px-4 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white">
                  {roleOptions.map(r => <option key={r.value} value={r.value}>{r.label}</option>)}
                </select>
              </div>
              <div>
                <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">Status</label>
                <select name="status" value={formData.status} onChange={e => setFormData({...formData, status: e.target.value})} className="w-full px-4 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white">
                  {statusOptions.map(s => <option key={s.value} value={s.value}>{s.label}</option>)}
                </select>
              </div>
              <div>
                <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">Estabelecimento</label>
                <select name="establishment_id" value={formData.establishment_id} onChange={e => setFormData({...formData, establishment_id: e.target.value})} className="w-full px-4 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white">
                  <option value="">Nenhum</option>
                  {establishments.map(e => <option key={e.id} value={e.id}>{e.name}</option>)}
                </select>
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
