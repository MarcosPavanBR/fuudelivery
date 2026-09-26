import React, { useState, useEffect } from "react";
import { FiPlus, FiSearch, FiEdit, FiTrash2, FiFilter, FiActivity, FiX } from "react-icons/fi";
import api from "../services/api";
import { toast } from "react-toastify";
import { useAuth } from "../context/AuthContext";

// A tabela users guarda só administradores e a equipe das lojas (clientes e
// entregadores têm cadastro próprio). No banco de produção o papel da loja é
// "restaurant"; em banco novo, "user" — os dois aparecem como "Loja".
const isAdminRole = (r) => r === "admin";
const roleLabel = (r) => (isAdminRole(r) ? "Admin" : "Loja");
const roleColor = (r) => (isAdminRole(r) ? "bg-red-100 text-red-800" : "bg-blue-100 text-blue-800");

const statusOptions = [
  { value: "active", label: "Ativo", color: "bg-green-100 text-green-800" },
  { value: "inactive", label: "Inativo", color: "bg-gray-100 text-gray-800" },
  { value: "pending", label: "Pendente", color: "bg-yellow-100 text-yellow-800" },
];

const inputCls = "w-full px-4 py-2.5 bg-gray-50 border border-gray-200 rounded-lg text-sm focus:bg-white";
const labelCls = "block text-xs font-semibold text-gray-500 uppercase mb-2";
const emptyForm = { name: "", email: "", phone: "", password: "", role: "restaurant", status: "active", establishment_id: "" };

export default function Users() {
  const { user: me } = useAuth() || {};
  const [users, setUsers] = useState([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");
  const [roleFilter, setRoleFilter] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState(null);
  const [saving, setSaving] = useState(false);
  const [formData, setFormData] = useState(emptyForm);
  const [establishments, setEstablishments] = useState([]);

  useEffect(() => {
    loadUsers();
    loadEstablishments();
  }, []);

  const loadUsers = async () => {
    try {
      const { data } = await api.get("/users");
      setUsers(Array.isArray(data) ? data : []);
    } catch (e) { console.error(e); toast.error("Erro ao carregar usuários"); }
    setLoading(false);
  };

  // Todas as lojas (inclusive fechadas) — GET /establishments é a vitrine.
  const loadEstablishments = async () => {
    try {
      const { data } = await api.get("/admin/establishments");
      setEstablishments(Array.isArray(data) ? data : []);
    } catch (e) { console.error(e); }
  };

  const term = search.trim().toLowerCase();
  const filtered = users.filter((u) => {
    const matchesSearch = !term || [u.name, u.email, u.phone].some((v) => String(v || "").toLowerCase().includes(term));
    const matchesRole = !roleFilter || (roleFilter === "admin" ? isAdminRole(u.role) : !isAdminRole(u.role));
    const matchesStatus = !statusFilter || (u.status || "active") === statusFilter;
    return matchesSearch && matchesRole && matchesStatus;
  });

  const set = (field) => (e) => setFormData({ ...formData, [field]: e.target.value });

  const handleSubmit = async (e) => {
    e.preventDefault();
    const estId = parseInt(formData.establishment_id, 10);
    // Sem loja é permitido (tirar alguém da equipe sem apagar a conta), mas
    // essa pessoa não consegue usar o site do restaurante até ser vinculada.
    if (formData.role !== "admin" && !estId &&
        !confirm("Sem loja, esta pessoa não consegue usar o site do restaurante até ser vinculada a uma. Continuar?")) {
      return;
    }
    // establishment_id vai como número: o texto "1" fazia o servidor recusar o JSON.
    const payload = {
      name: formData.name.trim(),
      email: formData.email.trim(),
      phone: formData.phone.trim(),
      role: formData.role,
      status: formData.status,
      // Na edição manda sempre: 0 tira da loja (admin também fica sem loja).
      ...(editing || estId ? { establishment_id: formData.role === "admin" ? 0 : estId || 0 } : {}),
      ...(formData.password ? { password: formData.password } : {}),
    };
    setSaving(true);
    try {
      if (editing) {
        await api.put(`/users/${editing.id}`, payload);
        toast.success(formData.password ? "Usuário atualizado e senha redefinida" : "Usuário atualizado");
      } else {
        await api.post("/users", payload);
        toast.success("Usuário criado");
      }
      setModalOpen(false);
      setEditing(null);
      loadUsers();
    } catch (err) {
      toast.error(err.response?.data?.error || "Erro ao salvar");
      console.error(err);
    }
    setSaving(false);
  };

  const handleDelete = async (u) => {
    if (!confirm(`Excluir ${u.name} (${u.email})? A pessoa perde o acesso ao painel.`)) return;
    try {
      await api.delete(`/users/${u.id}`);
      toast.success("Usuário excluído");
      loadUsers();
    } catch (e) { toast.error(e.response?.data?.error || "Erro ao excluir"); }
  };

  const openEdit = (u) => {
    setEditing(u);
    setFormData({
      name: u.name || "",
      email: u.email || "",
      phone: u.phone || "",
      password: "",
      role: isAdminRole(u.role) ? "admin" : "restaurant",
      status: u.status || "active",
      establishment_id: u.establishment_id ? String(u.establishment_id) : "",
    });
    setModalOpen(true);
  };

  const openNew = () => { setEditing(null); setFormData(emptyForm); setModalOpen(true); };
  const closeModal = () => { setModalOpen(false); setEditing(null); };

  if (loading) return <div className="flex items-center justify-center h-64"><FiActivity className="animate-spin h-8 w-8 text-fuu-red" /></div>;

  return (
    <div className="animate-fade-in space-y-6">
      <div className="flex items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Usuários</h1>
          <p className="text-gray-500 mt-1">Administradores e equipes das lojas · {filtered.length} de {users.length}</p>
        </div>
        <button onClick={openNew} className="flex items-center gap-2 px-4 py-2 rounded-xl text-white font-semibold text-sm bg-gradient-to-br from-fuu-red to-fuu-red-dark">
          <FiPlus className="h-4 w-4" /> Novo
        </button>
      </div>

      <div className="bg-white rounded-xl shadow-card border border-gray-100 p-4">
        <div className="flex flex-col sm:flex-row gap-4">
          <div className="flex-1 relative">
            <FiSearch className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" />
            <input type="text" placeholder="Buscar por nome, e-mail, telefone..." value={search} onChange={(e) => setSearch(e.target.value)} className="w-full pl-10 pr-4 py-2.5 bg-gray-50 border border-gray-200 rounded-lg text-sm focus:bg-white" />
          </div>
          <div className="relative">
            <FiFilter className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" />
            <select value={roleFilter} onChange={(e) => setRoleFilter(e.target.value)} className="w-full sm:w-44 pl-10 pr-10 py-2.5 bg-gray-50 border border-gray-200 rounded-lg text-sm focus:bg-white appearance-none">
              <option value="">Todos os papéis</option>
              <option value="admin">Admin</option>
              <option value="restaurant">Loja</option>
            </select>
          </div>
          <div className="relative">
            <FiFilter className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" />
            <select value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)} className="w-full sm:w-40 pl-10 pr-10 py-2.5 bg-gray-50 border border-gray-200 rounded-lg text-sm focus:bg-white appearance-none">
              <option value="">Todos os status</option>
              {statusOptions.map((s) => <option key={s.value} value={s.value}>{s.label}</option>)}
            </select>
          </div>
        </div>
      </div>

      <div className="bg-white rounded-xl shadow-card border border-gray-100 overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-6 py-2 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Usuário</th>
                <th className="px-6 py-2 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Papel</th>
                <th className="px-6 py-2 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Status</th>
                <th className="px-6 py-2 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Loja</th>
                <th className="px-6 py-2 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Criado em</th>
                <th className="px-6 py-2 text-right text-xs font-semibold text-gray-500 uppercase tracking-wider">Ações</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {filtered.length === 0 ? (
                <tr><td colSpan={6} className="px-6 py-12 text-center text-gray-500">Nenhum usuário encontrado</td></tr>
              ) : filtered.map((u) => {
                const st = statusOptions.find((s) => s.value === (u.status || "active"));
                return (
                  <tr key={u.id} className="hover:bg-gray-50">
                    <td className="px-6 py-4">
                      <p className="font-medium text-gray-900">{u.name}{me?.id === u.id && <span className="ml-2 text-xs text-gray-400">(você)</span>}</p>
                      <p className="text-xs text-gray-500">{u.email}</p>
                      {u.phone && <p className="text-xs text-gray-400">{u.phone}</p>}
                    </td>
                    <td className="px-6 py-4"><span className={`inline-flex items-center px-2.5 py-1 rounded-full text-xs font-medium ${roleColor(u.role)}`}>{roleLabel(u.role)}</span></td>
                    <td className="px-6 py-4"><span className={`inline-flex items-center px-2.5 py-1 rounded-full text-xs font-medium ${st?.color || "bg-gray-100 text-gray-800"}`}>{st?.label || u.status}</span></td>
                    <td className="px-6 py-4 text-sm text-gray-600">
                      {u.establishment_id ? (establishments.find((e) => e.id === u.establishment_id)?.name || `#${u.establishment_id} (removida)`) : isAdminRole(u.role) ? "—" : <span className="text-amber-600">Sem loja</span>}
                    </td>
                    <td className="px-6 py-4 text-sm text-gray-500">{u.createdAt ? new Date(u.createdAt).toLocaleDateString("pt-BR") : "—"}</td>
                    <td className="px-6 py-4">
                      <div className="flex items-center gap-2 justify-end">
                        <button onClick={() => openEdit(u)} title="Editar" className="p-2 text-gray-400 hover:text-fuu-red hover:bg-fuu-red-light rounded-lg"><FiEdit className="h-4 w-4" /></button>
                        {me?.id !== u.id && (
                          <button onClick={() => handleDelete(u)} title="Excluir" className="p-2 text-gray-400 hover:text-red-500 hover:bg-red-50 rounded-lg"><FiTrash2 className="h-4 w-4" /></button>
                        )}
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </div>

      {modalOpen && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4 animate-fade-in">
          <div className="bg-white rounded-xl w-full max-w-md max-h-[90vh] overflow-hidden shadow-modal animate-slide-up">
            <div className="flex items-center justify-between px-6 py-4 border-b border-gray-100">
              <h2 className="text-lg font-bold text-gray-900">{editing ? "Editar" : "Novo"} usuário</h2>
              <button onClick={closeModal} className="p-2 rounded-xl hover:bg-gray-100"><FiX className="h-5 w-5 text-gray-500" /></button>
            </div>
            <form onSubmit={handleSubmit} className="p-6 space-y-4 overflow-y-auto max-h-[75vh]">
              <div><label className={labelCls}>Nome *</label><input required value={formData.name} onChange={set("name")} className={inputCls} /></div>
              <div><label className={labelCls}>E-mail *</label><input required type="email" value={formData.email} onChange={set("email")} className={inputCls} /></div>
              <div><label className={labelCls}>Telefone</label><input type="tel" value={formData.phone} onChange={set("phone")} placeholder="(11) 99999-9999" className={inputCls} /></div>
              <div>
                <label className={labelCls}>{editing ? "Nova senha (deixe em branco para manter)" : "Senha *"}</label>
                <input type="password" autoComplete="new-password" required={!editing} minLength={6} value={formData.password} onChange={set("password")} className={inputCls} placeholder="Mínimo 6 caracteres" />
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className={labelCls}>Papel *</label>
                  <select value={formData.role} onChange={set("role")} className={inputCls}>
                    <option value="restaurant">Loja</option>
                    <option value="admin">Admin</option>
                  </select>
                </div>
                <div>
                  <label className={labelCls}>Status</label>
                  <select value={formData.status} onChange={set("status")} className={inputCls}>
                    {statusOptions.map((s) => <option key={s.value} value={s.value}>{s.label}</option>)}
                  </select>
                </div>
              </div>
              {formData.role !== "admin" && (
                <div>
                  <label className={labelCls}>Loja</label>
                  <select value={formData.establishment_id} onChange={set("establishment_id")} className={inputCls}>
                    <option value="">Sem loja (sem acesso ao site do restaurante)</option>
                    {establishments.map((e) => <option key={e.id} value={String(e.id)}>{e.name}</option>)}
                  </select>
                </div>
              )}
              <div className="flex justify-end gap-2 pt-4 border-t border-gray-100">
                <button type="button" onClick={closeModal} className="px-5 py-2.5 rounded-xl text-sm font-medium text-gray-700 bg-white border border-gray-200 hover:bg-gray-50">Cancelar</button>
                <button type="submit" disabled={saving} className="flex items-center gap-2 px-5 py-2.5 rounded-xl text-sm font-medium text-white transition-all bg-gradient-to-br from-fuu-red to-fuu-red-dark disabled:opacity-60">
                  {saving ? "Salvando..." : editing ? "Salvar" : "Criar"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
