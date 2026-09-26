import React, { useState, useEffect } from "react";
import { FiPlus, FiSearch, FiEdit, FiTrash2, FiMapPin, FiFilter, FiActivity, FiX, FiPower, FiPhone, FiMail, FiSlash } from "react-icons/fi";
import api from "../services/api";
import { toast } from "react-toastify";
import {
  emptyForm,
  formFromEstablishment,
  updatePayload,
  createPayload,
  matchesSearch,
  matchesStatus,
  formatPhone,
  geocodeAddress,
} from "../helpers/establishmentForm";

const inputCls = "w-full px-4 py-2.5 bg-gray-50 border border-gray-200 rounded-lg text-sm focus:bg-white";
const labelCls = "block text-xs font-semibold text-gray-500 uppercase mb-2";

export default function Establishments() {
  const [establishments, setEstablishments] = useState([]);
  const [zones, setZones] = useState([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState(null);
  const [saving, setSaving] = useState(false);
  const [locating, setLocating] = useState(false);
  const [formData, setFormData] = useState(emptyForm);

  useEffect(() => {
    loadEstablishments();
    api.get("/zones/all").then(({ data }) => setZones(Array.isArray(data) ? data : [])).catch(() => setZones([]));
  }, []);

  const loadEstablishments = async () => {
    try {
      const { data } = await api.get("/admin/establishments");
      setEstablishments(Array.isArray(data) ? data : []);
    } catch (e) {
      console.error(e);
      toast.error("Erro ao carregar estabelecimentos");
    }
    setLoading(false);
  };

  const filtered = establishments.filter((e) => matchesSearch(e, search) && matchesStatus(e, statusFilter));
  const zoneName = (id) => zones.find((z) => z.id === id)?.name;
  const set = (field) => (e) => setFormData({ ...formData, [field]: e.target.value });

  const closeModal = () => {
    setModalOpen(false);
    setEditing(null);
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    setSaving(true);
    try {
      let form = formData;
      // Sem coordenadas a loja não calcula frete nem aparece por distância.
      if (form.location_string.trim() && (!parseFloat(form.lat) || !parseFloat(form.long))) {
        const pos = await geocodeAddress(form.location_string);
        if (pos) form = { ...form, lat: pos.lat, long: pos.long };
      }
      if (editing) {
        await api.put(`/establishments/${editing.id}`, updatePayload(editing, form));
        toast.success("Estabelecimento atualizado");
      } else {
        await api.post("/establishments", createPayload(form));
        toast.success("Estabelecimento criado. Vincule um usuário a ele em Usuários.");
      }
      closeModal();
      loadEstablishments();
    } catch (err) {
      toast.error(err.response?.data?.error || "Erro ao salvar");
      console.error(err);
    }
    setSaving(false);
  };

  const handleToggle = async (est) => {
    const verb = est.accepting_orders ? "Fechar" : "Abrir";
    if (!confirm(`${verb} "${est.name}" para pedidos?`)) return;
    try {
      await api.put(`/establishments/status/handler/${est.id}`);
      toast.success(est.accepting_orders ? "Loja fechada" : "Loja aberta");
      loadEstablishments();
    } catch (e) {
      toast.error(e.response?.data?.error || "Erro ao mudar o status");
    }
  };

  const setDisabled = async (est, disabled) => {
    const msg = disabled
      ? `Desativar "${est.name}"? Ela sai do app, para de receber pedidos e não consegue se abrir. Pedidos e repasses antigos ficam guardados.`
      : `Reativar "${est.name}"? A loja volta a poder se abrir para pedidos.`;
    if (!confirm(msg)) return;
    try {
      await api.put(`/establishments/${est.id}/disabled`, { disabled });
      toast.success(disabled ? "Loja desativada" : "Loja reativada");
      loadEstablishments();
    } catch (e) {
      toast.error(e.response?.data?.error || "Erro ao mudar a loja");
    }
  };

  // Excluir só vale para loja sem pedidos; com histórico o servidor responde
  // 409 e a saída é desativar.
  const handleDelete = async (est) => {
    if (!confirm(`Excluir "${est.name}" de vez? O cardápio e os horários saem junto e os usuários dela ficam sem loja. Não dá para desfazer.`)) return;
    try {
      await api.delete(`/establishments/${est.id}`);
      toast.success("Loja excluída");
      loadEstablishments();
    } catch (e) {
      if (e.response?.status === 409) {
        if (!est.disabled_at && confirm(`${e.response.data.error}\n\nDesativar agora?`)) {
          try {
            await api.put(`/establishments/${est.id}/disabled`, { disabled: true });
            toast.success("Loja desativada");
            loadEstablishments();
          } catch (err) {
            toast.error(err.response?.data?.error || "Erro ao desativar");
          }
        } else if (est.disabled_at) {
          toast.info("Esta loja tem histórico e já está desativada.");
        }
        return;
      }
      toast.error(e.response?.data?.error || "Erro ao excluir");
    }
  };

  const handleLocate = async () => {
    setLocating(true);
    const pos = await geocodeAddress(formData.location_string);
    setLocating(false);
    if (!pos) return toast.error("Endereço não encontrado. Confira rua, número e cidade.");
    setFormData({ ...formData, lat: pos.lat, long: pos.long });
    toast.success("Coordenadas preenchidas");
  };

  const openEdit = (est) => {
    setEditing(est);
    setFormData(formFromEstablishment(est));
    setModalOpen(true);
  };

  const openNew = () => {
    setEditing(null);
    setFormData(emptyForm);
    setModalOpen(true);
  };

  if (loading) return <div className="flex items-center justify-center h-64"><FiActivity className="animate-spin h-8 w-8 text-fuu-red" /></div>;

  const openCount = establishments.filter((e) => e.accepting_orders).length;

  return (
    <div className="animate-fade-in space-y-6">
      <div className="flex items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Estabelecimentos</h1>
          <p className="text-gray-500 mt-1">
            {establishments.length} cadastrados · {openCount} abertos agora
            {filtered.length !== establishments.length && ` · ${filtered.length} no filtro`}
          </p>
        </div>
        <button onClick={openNew} className="flex items-center gap-2 px-4 py-2 rounded-xl text-white font-semibold text-sm bg-gradient-to-br from-fuu-red to-fuu-red-dark">
          <FiPlus className="h-4 w-4" />
          Novo
        </button>
      </div>

      {/* Filters */}
      <div className="bg-white rounded-xl shadow-card border border-gray-100 p-4">
        <div className="flex flex-col sm:flex-row gap-4">
          <div className="flex-1 relative">
            <FiSearch className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" />
            <input
              type="text"
              placeholder="Buscar por nome, responsável, e-mail, endereço..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="w-full pl-10 pr-4 py-2.5 bg-gray-50 border border-gray-200 rounded-lg text-sm focus:bg-white"
            />
          </div>
          <div className="relative">
            <FiFilter className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" />
            <select value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)} className="w-full sm:w-48 pl-10 pr-10 py-2.5 bg-gray-50 border border-gray-200 rounded-lg text-sm focus:bg-white appearance-none">
              <option value="">Todos</option>
              <option value="open">Abertos</option>
              <option value="closed">Fechados</option>
              <option value="disabled">Desativadas</option>
            </select>
          </div>
        </div>
      </div>

      {/* Table */}
      <div className="bg-white rounded-xl shadow-card border border-gray-100 overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-6 py-2 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Estabelecimento</th>
                <th className="px-6 py-2 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Responsável</th>
                <th className="px-6 py-2 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Endereço</th>
                <th className="px-6 py-2 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Status</th>
                <th className="px-6 py-2 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Entrega</th>
                <th className="px-6 py-2 text-right text-xs font-semibold text-gray-500 uppercase tracking-wider">Ações</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {filtered.length === 0 ? (
                <tr><td colSpan={6} className="px-6 py-12 text-center text-gray-500">Nenhum estabelecimento encontrado</td></tr>
              ) : (
                filtered.map((est) => (
                  <tr key={est.id} className="hover:bg-gray-50 transition-colors">
                    <td className="px-6 py-4">
                      <div className="flex items-center gap-3">
                        {est.image ? (
                          <img src={est.image} alt="" className="h-10 w-10 rounded-lg object-cover border border-gray-100 flex-shrink-0" />
                        ) : (
                          <div className="h-10 w-10 rounded-lg bg-fuu-red-light text-fuu-red flex items-center justify-center font-bold flex-shrink-0">
                            {(est.name || "?").charAt(0).toUpperCase()}
                          </div>
                        )}
                        <div className="min-w-0">
                          <p className="font-medium text-gray-900 truncate">{est.name}</p>
                          <p className="text-xs text-gray-500">#{est.id}{zoneName(est.zone_id) ? ` · ${zoneName(est.zone_id)}` : ""}</p>
                        </div>
                      </div>
                    </td>
                    <td className="px-6 py-4">
                      {est.owner_email ? (
                        <>
                          <p className="text-sm text-gray-900">{est.owner_name}</p>
                          <p className="text-xs text-gray-500 flex items-center gap-1"><FiMail className="h-3 w-3" />{est.owner_email}</p>
                          {est.owner_phone && (
                            <a href={`tel:${est.owner_phone}`} className="text-xs text-gray-500 flex items-center gap-1 hover:text-fuu-red"><FiPhone className="h-3 w-3" />{formatPhone(est.owner_phone)}</a>
                          )}
                        </>
                      ) : (
                        <span className="text-xs text-amber-600">Sem usuário vinculado</span>
                      )}
                    </td>
                    <td className="px-6 py-4 text-sm text-gray-600 max-w-xs">
                      {est.location_string ? (
                        <span className="flex items-start gap-1">
                          <FiMapPin className="h-3.5 w-3.5 mt-0.5 flex-shrink-0 text-gray-400" />
                          <span className="line-clamp-2">{est.location_string}</span>
                        </span>
                      ) : (
                        <span className="text-xs text-gray-400">Não informado</span>
                      )}
                      {est.location_string && (!est.lat || !est.long) && <p className="text-xs text-amber-600 mt-1">Sem coordenadas</p>}
                    </td>
                    <td className="px-6 py-4">
                      {est.disabled_at ? (
                        <span className="inline-flex items-center px-2.5 py-1 rounded-full text-xs font-medium bg-red-100 text-red-800">Desativada</span>
                      ) : (
                        <span className={`inline-flex items-center px-2.5 py-1 rounded-full text-xs font-medium ${est.accepting_orders ? "bg-green-100 text-green-800" : "bg-gray-100 text-gray-700"}`}>
                          {est.accepting_orders ? "Aberto" : "Fechado"}
                        </span>
                      )}
                    </td>
                    <td className="px-6 py-4 text-sm text-gray-600 whitespace-nowrap">
                      até {Number(est.max_distance_delivery || 0).toLocaleString("pt-BR")} km
                    </td>
                    <td className="px-6 py-4">
                      <div className="flex items-center gap-1 justify-end">
                        {est.disabled_at ? (
                          <button onClick={() => setDisabled(est, false)} className="px-2.5 py-1.5 rounded-lg text-xs font-medium text-green-700 hover:bg-green-50" title="Reativar loja">Reativar</button>
                        ) : (
                          <>
                            <button onClick={() => handleToggle(est)} className={`p-2 rounded-lg transition-colors ${est.accepting_orders ? "text-green-600 hover:bg-green-50" : "text-gray-400 hover:bg-gray-100"}`} title={est.accepting_orders ? "Fechar para pedidos" : "Abrir para pedidos"}><FiPower className="h-4 w-4" /></button>
                            <button onClick={() => setDisabled(est, true)} className="p-2 text-gray-400 hover:text-amber-600 hover:bg-amber-50 rounded-lg transition-colors" title="Desativar loja"><FiSlash className="h-4 w-4" /></button>
                          </>
                        )}
                        <button onClick={() => openEdit(est)} className="p-2 text-gray-400 hover:text-fuu-red hover:bg-fuu-red-light rounded-lg transition-colors" title="Editar"><FiEdit className="h-4 w-4" /></button>
                        <button onClick={() => handleDelete(est)} className="p-2 text-gray-400 hover:text-red-500 hover:bg-red-50 rounded-lg transition-colors" title="Excluir"><FiTrash2 className="h-4 w-4" /></button>
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
          <div className="bg-white rounded-xl w-full max-w-2xl max-h-[90vh] overflow-hidden shadow-modal animate-slide-up">
            <div className="flex items-center justify-between px-6 py-4 border-b border-gray-100">
              <h2 className="text-lg font-bold text-gray-900">{editing ? "Editar" : "Novo"} estabelecimento</h2>
              <button onClick={closeModal} className="p-2 rounded-xl hover:bg-gray-100"><FiX className="h-5 w-5 text-gray-500" /></button>
            </div>
            <form onSubmit={handleSubmit} className="p-6 overflow-y-auto max-h-[70vh] space-y-4">
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div className="sm:col-span-2"><label className={labelCls}>Nome *</label><input required value={formData.name} onChange={set("name")} className={inputCls} /></div>
                <div className="sm:col-span-2"><label className={labelCls}>Descrição</label><textarea rows={2} value={formData.description} onChange={set("description")} className={inputCls} /></div>
                <div className="sm:col-span-2">
                  <label className={labelCls}>Endereço</label>
                  <div className="flex gap-2">
                    <input value={formData.location_string} onChange={set("location_string")} placeholder="Rua, número, bairro, cidade - UF" className={inputCls} />
                    <button type="button" onClick={handleLocate} disabled={locating || !formData.location_string.trim()} className="flex items-center gap-1 px-3 rounded-lg text-sm border border-gray-200 hover:bg-gray-50 disabled:opacity-50 whitespace-nowrap">
                      <FiMapPin className="h-4 w-4" />{locating ? "Buscando..." : "Localizar"}
                    </button>
                  </div>
                </div>
                <div><label className={labelCls}>Latitude</label><input type="number" step="any" value={formData.lat} onChange={set("lat")} className={inputCls} /></div>
                <div><label className={labelCls}>Longitude</label><input type="number" step="any" value={formData.long} onChange={set("long")} className={inputCls} /></div>
                <div><label className={labelCls}>Raio de entrega (km)</label><input type="number" step="0.5" min="0" value={formData.max_distance_delivery} onChange={set("max_distance_delivery")} className={inputCls} /></div>
                <div>
                  <label className={labelCls}>Região (repasse)</label>
                  <select value={formData.zone_id} onChange={set("zone_id")} className={inputCls}>
                    <option value="">Padrão (5% plataforma / 85% loja)</option>
                    {zones.map((z) => <option key={z.id} value={String(z.id)}>{z.name}</option>)}
                  </select>
                </div>
              </div>
              {editing && <p className="text-xs text-gray-500">Logo, cores e horários são ajustados pela própria loja no site do restaurante.</p>}
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
