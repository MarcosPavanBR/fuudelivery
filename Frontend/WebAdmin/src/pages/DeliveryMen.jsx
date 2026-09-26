import React, { useState, useEffect } from "react";
import { FiPlus, FiSearch, FiFilter, FiEdit, FiTrash2, FiActivity, FiX, FiPhone } from "react-icons/fi";
import api from "../services/api";
import { toast } from "react-toastify";

// Status do motor de despacho (models.DeliveryMan.Status). A tela usava
// "online"/"on_delivery", que o servidor não conhece, e campos de veículo,
// CNH e CPF que não existem no cadastro — eram descartados sem aviso.
const statusOptions = [
  { value: "available", label: "Disponível", color: "bg-green-100 text-green-800" },
  { value: "busy", label: "Em entrega", color: "bg-blue-100 text-blue-800" },
  { value: "offline", label: "Offline", color: "bg-gray-100 text-gray-800" },
];

const inputCls = "w-full px-4 py-2.5 bg-gray-50 border border-gray-200 rounded-lg text-sm focus:bg-white";
const labelCls = "block text-xs font-semibold text-gray-500 uppercase mb-2";
const emptyForm = { name: "", email: "", phone: "", password: "", status: "offline", zone_id: "", max_orders: 3 };

export default function DeliveryMen() {
  const [drivers, setDrivers] = useState([]);
  const [zones, setZones] = useState([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState(null);
  const [saving, setSaving] = useState(false);
  const [formData, setFormData] = useState(emptyForm);

  useEffect(() => {
    loadDrivers();
    api.get("/zones/all").then(({ data }) => setZones(Array.isArray(data) ? data : [])).catch(() => setZones([]));
  }, []);

  const loadDrivers = async () => {
    try {
      const { data } = await api.get("/delivery-man");
      setDrivers(Array.isArray(data) ? data : []);
    } catch (e) { console.error(e); toast.error("Erro ao carregar entregadores"); }
    setLoading(false);
  };

  const term = search.trim().toLowerCase();
  const filtered = drivers.filter((d) => {
    const matchesSearch = !term || [d.name, d.email, d.phone].some((v) => String(v || "").toLowerCase().includes(term));
    const matchesStatus = !statusFilter || d.status === statusFilter;
    return matchesSearch && matchesStatus;
  });
  const zoneName = (id) => zones.find((z) => z.id === id)?.name;
  const set = (field) => (e) => setFormData({ ...formData, [field]: e.target.value });

  const handleSubmit = async (e) => {
    e.preventDefault();
    const zone = parseInt(formData.zone_id, 10);
    const maxOrders = parseInt(formData.max_orders, 10);
    const payload = {
      name: formData.name.trim(),
      email: formData.email.trim(),
      phone: formData.phone.trim(),
      status: formData.status,
      ...(zone > 0 ? { zone_id: zone } : {}),
      ...(maxOrders > 0 ? { max_orders: maxOrders } : {}),
      ...(formData.password ? { password: formData.password } : {}),
    };
    setSaving(true);
    try {
      if (editing) {
        await api.put(`/delivery-man/${editing.id}`, payload);
        toast.success(formData.password ? "Entregador atualizado e senha redefinida" : "Entregador atualizado");
      } else {
        await api.post("/delivery-man", payload);
        toast.success("Entregador cadastrado. Ele entra no app com este e-mail e senha.");
      }
      setModalOpen(false);
      setEditing(null);
      loadDrivers();
    } catch (err) {
      toast.error(err.response?.data?.error || "Erro ao salvar");
      console.error(err);
    }
    setSaving(false);
  };

  const handleDelete = async (d) => {
    if (!confirm(`Excluir o entregador ${d.name}? Ele perde o acesso ao app.`)) return;
    try { await api.delete(`/delivery-man/${d.id}`); toast.success("Entregador excluído"); loadDrivers(); }
    catch (e) { toast.error(e.response?.data?.error || "Erro ao excluir"); }
  };

  const openEdit = (d) => {
    setEditing(d);
    setFormData({
      name: d.name || "",
      email: d.email || "",
      phone: d.phone || "",
      password: "",
      status: statusOptions.some((s) => s.value === d.status) ? d.status : "offline",
      zone_id: d.zone_id ? String(d.zone_id) : "",
      max_orders: d.max_orders || 3,
    });
    setModalOpen(true);
  };
  const openNew = () => { setEditing(null); setFormData(emptyForm); setModalOpen(true); };
  const closeModal = () => { setModalOpen(false); setEditing(null); };

  if (loading) return <div className="flex items-center justify-center h-64"><FiActivity className="animate-spin h-8 w-8 text-fuu-red" /></div>;

  const available = drivers.filter((d) => d.status === "available").length;
  const busy = drivers.filter((d) => d.status === "busy").length;

  return (
    <div className="animate-fade-in space-y-6">
      <div className="flex items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Entregadores</h1>
          <p className="text-gray-500 mt-1">{drivers.length} cadastrados · {available} disponíveis · {busy} em entrega</p>
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
            <select value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)} className="w-full sm:w-44 pl-10 pr-10 py-2.5 bg-gray-50 border border-gray-200 rounded-lg text-sm focus:bg-white appearance-none">
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
                <th className="px-6 py-2 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Entregador</th>
                <th className="px-6 py-2 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Contato</th>
                <th className="px-6 py-2 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Região</th>
                <th className="px-6 py-2 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Pedidos simultâneos</th>
                <th className="px-6 py-2 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Status</th>
                <th className="px-6 py-2 text-right text-xs font-semibold text-gray-500 uppercase tracking-wider">Ações</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {filtered.length === 0 ? (
                <tr><td colSpan={6} className="px-6 py-12 text-center text-gray-500">{drivers.length ? "Nenhum entregador no filtro" : "Nenhum entregador cadastrado ainda"}</td></tr>
              ) : filtered.map((d) => {
                const st = statusOptions.find((s) => s.value === d.status);
                return (
                  <tr key={d.id} className="hover:bg-gray-50">
                    <td className="px-6 py-4">
                      <div className="flex items-center gap-2">
                        <div className="w-8 h-8 rounded-full bg-fuu-red-light flex items-center justify-center flex-shrink-0">
                          <span className="text-xs font-bold text-fuu-red">{(d.name || "E").charAt(0).toUpperCase()}</span>
                        </div>
                        <div><p className="font-medium text-gray-900">{d.name}</p><p className="text-xs text-gray-400">#{d.id}</p></div>
                      </div>
                    </td>
                    <td className="px-6 py-4">
                      <p className="text-sm text-gray-900">{d.email}</p>
                      {d.phone && <a href={`tel:${d.phone}`} className="text-xs text-gray-500 hover:text-fuu-red inline-flex items-center gap-1"><FiPhone className="h-3 w-3" />{d.phone}</a>}
                    </td>
                    <td className="px-6 py-4 text-sm text-gray-600">{zoneName(d.zone_id) || "—"}</td>
                    <td className="px-6 py-4 text-sm text-gray-600">{d.max_orders || "—"}</td>
                    <td className="px-6 py-4"><span className={`inline-flex items-center px-2.5 py-1 rounded-full text-xs font-medium ${st?.color || "bg-gray-100 text-gray-800"}`}>{st?.label || d.status || "—"}</span></td>
                    <td className="px-6 py-4">
                      <div className="flex items-center gap-2 justify-end">
                        <button onClick={() => openEdit(d)} title="Editar" className="p-2 text-gray-400 hover:text-fuu-red hover:bg-fuu-red-light rounded-lg"><FiEdit className="h-4 w-4" /></button>
                        <button onClick={() => handleDelete(d)} title="Excluir" className="p-2 text-gray-400 hover:text-red-500 hover:bg-red-50 rounded-lg"><FiTrash2 className="h-4 w-4" /></button>
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
              <h2 className="text-lg font-bold text-gray-900">{editing ? "Editar" : "Novo"} entregador</h2>
              <button onClick={closeModal} className="p-2 rounded-xl hover:bg-gray-100"><FiX className="h-5 w-5 text-gray-500" /></button>
            </div>
            <form onSubmit={handleSubmit} className="p-6 space-y-4 overflow-y-auto max-h-[75vh]">
              <div><label className={labelCls}>Nome *</label><input required value={formData.name} onChange={set("name")} className={inputCls} /></div>
              <div><label className={labelCls}>E-mail *</label><input required type="email" value={formData.email} onChange={set("email")} className={inputCls} /></div>
              <div><label className={labelCls}>Telefone</label><input type="tel" value={formData.phone} onChange={set("phone")} placeholder="(11) 99999-9999" className={inputCls} /></div>
              <div>
                <label className={labelCls}>{editing ? "Nova senha (deixe em branco para manter)" : "Senha de acesso ao app *"}</label>
                <input type="password" autoComplete="new-password" required={!editing} minLength={6} value={formData.password} onChange={set("password")} placeholder="Mínimo 6 caracteres" className={inputCls} />
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className={labelCls}>Status</label>
                  <select value={formData.status} onChange={set("status")} className={inputCls}>
                    {statusOptions.map((s) => <option key={s.value} value={s.value}>{s.label}</option>)}
                  </select>
                </div>
                <div>
                  <label className={labelCls}>Pedidos ao mesmo tempo</label>
                  <input type="number" min="1" max="10" value={formData.max_orders} onChange={set("max_orders")} className={inputCls} />
                </div>
              </div>
              <div>
                <label className={labelCls}>Região</label>
                <select value={formData.zone_id} onChange={set("zone_id")} className={inputCls}>
                  <option value="">Sem região</option>
                  {zones.map((z) => <option key={z.id} value={String(z.id)}>{z.name}</option>)}
                </select>
              </div>
              <div className="flex justify-end gap-2 pt-4 border-t border-gray-100">
                <button type="button" onClick={closeModal} className="px-5 py-2.5 rounded-xl text-sm font-medium text-gray-700 bg-white border border-gray-200 hover:bg-gray-50">Cancelar</button>
                <button type="submit" disabled={saving} className="flex items-center gap-2 px-5 py-2.5 rounded-xl text-sm font-medium text-white transition-all bg-gradient-to-br from-fuu-red to-fuu-red-dark disabled:opacity-60">
                  {saving ? "Salvando..." : editing ? "Salvar" : "Cadastrar"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
