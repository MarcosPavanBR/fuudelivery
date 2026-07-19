import React, { useState, useEffect } from "react";
import { FiSettings, FiUser, FiBell, FiShield, FiSave, FiLoader, FiEye, FiEyeOff, FiBuilding2, FiGlobe, FiKey, FiCreditCard, FiTrash2, FiPlus } from "react-icons/fi";
import { useAuth } from "../context/AuthContext";
import api from "../services/api";
import { toast } from "react-toastify";

const tabs = [
  { id: "profile", label: "Perfil", icon: FiUser },
  { id: "security", label: "Segurança", icon: FiShield },
  { id: "notifications", label: "Notificações", icon: FiBell },
  { id: "appearance", label: "Aparência", icon: FiSettings },
  { id: "integrations", label: "Integrações", icon: FiCreditCard },
];

export default function Settings() {
  const { user, logout } = useAuth();
  const [activeTab, setActiveTab] = useState("profile");
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [formData, setFormData] = useState({
    name: "",
    email: "",
    phone: "",
    currentPassword: "",
    newPassword: "",
    confirmPassword: "",
    language: "pt-BR",
    timezone: "America/Sao_Paulo",
    emailNotifications: true,
    pushNotifications: true,
    orderUpdates: true,
    marketingEmails: false,
    theme: "system",
    compactMode: false,
    autoRefresh: true,
    refreshInterval: 30,
  });

  useEffect(() => {
    if (user) {
      setFormData(prev => ({ ...prev, name: user.name || "", email: user.email || "" }));
    }
  }, [user]);

  const handleSave = async (e) => {
    e.preventDefault();
    setSaving(true);
    try {
      // Simulate API call
      await new Promise(r => setTimeout(r, 1000));
      toast.success("Configurações salvas com sucesso!");
    } catch (err) {
      toast.error("Erro ao salvar");
    }
    setSaving(false);
  };

  const handleLogout = () => logout();

  return (
    <div className="animate-fade-in">
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Configurações</h1>
          <p className="text-gray-500 mt-1">Gerencie suas preferências e configurações da conta</p>
        </div>
      </div>

      <div className="flex flex-col lg:flex-row gap-8">
        {/* Sidebar */}
        <aside className="lg:w-64 flex-shrink-0">
          <nav className="bg-white rounded-2xl shadow-card border border-gray-100 overflow-hidden">
            {tabs.map((tab) => (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id)}
                className={`w-full flex items-center gap-3 px-6 py-4 transition-all duration-200 border-b border-gray-100 last:border-0 ${
                  activeTab === tab.id
                    ? "bg-fuu-red-light text-fuu-red font-semibold"
                    : "text-gray-600 hover:bg-gray-50 hover:text-gray-900"
                }`}
              >
                <tab.icon className="h-5 w-5 flex-shrink-0" />
                {tab.label}
              </button>
            ))}
            <div className="border-t border-gray-100 p-4">
              <button
                onClick={handleLogout}
                className="w-full flex items-center gap-3 px-4 py-3 text-red-600 hover:bg-red-50 rounded-xl transition-colors font-medium"
              >
                <FiLogOut className="h-5 w-5" />
                Sair da conta
              </button>
            </div>
          </nav>
        </aside>

        {/* Content */}
        <main className="flex-1">
          {activeTab === "profile" && (
            <div className="bg-white rounded-2xl shadow-card border border-gray-100 p-6 animate-fade-in">
              <h2 className="text-xl font-bold text-gray-900 mb-6">Perfil</h2>
              <form onSubmit={handleSave} className="space-y-6 max-w-2xl">
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-6">
                  <div>
                    <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">Nome completo</label>
                    <input name="name" value={formData.name} onChange={e => setFormData({...formData, name: e.target.value})} className="w-full px-4 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white" />
                  </div>
                  <div>
                    <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">E-mail</label>
                    <input name="email" type="email" value={formData.email} onChange={e => setFormData({...formData, email: e.target.value})} className="w-full px-4 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white" />
                  </div>
                  <div>
                    <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">Telefone</label>
                    <input name="phone" value={formData.phone} onChange={e => setFormData({...formData, phone: e.target.value})} className="w-full px-4 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white" />
                  </div>
                </div>
                <div className="pt-4 border-t border-gray-100 flex justify-end gap-3">
                  <button type="button" className="px-5 py-2.5 rounded-xl text-sm font-medium text-gray-700 bg-white border border-gray-200 hover:bg-gray-50">Cancelar</button>
                  <button type="submit" disabled={saving} className="flex items-center gap-2 px-5 py-2.5 rounded-xl text-sm font-medium text-white transition-all" style={{ background: "linear-gradient(135deg, #EA1D2C, #C41420)" }}><FiSave className="h-4 w-4" />{saving ? " Salvando..." : " Salvar alterações"}</button>
                </div>
              </form>
            </div>
          )}

          {activeTab === "security" && (
            <div className="bg-white rounded-2xl shadow-card border border-gray-100 p-6 animate-fade-in">
              <h2 className="text-xl font-bold text-gray-900 mb-6">Segurança</h2>
              <form onSubmit={handleSave} className="space-y-6 max-w-2xl">
                <div>
                  <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">Senha atual</label>
                  <input type="password" name="currentPassword" value={formData.currentPassword} onChange={e => setFormData({...formData, currentPassword: e.target.value})} className="w-full px-4 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white" />
                </div>
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-6">
                  <div>
                    <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">Nova senha</label>
                    <input type="password" name="newPassword" value={formData.newPassword} onChange={e => setFormData({...formData, newPassword: e.target.value})} className="w-full px-4 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white" placeholder="Mínimo 8 caracteres" />
                  </div>
                  <div>
                    <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">Confirmar senha</label>
                    <input type="password" name="confirmPassword" value={formData.confirmPassword} onChange={e => setFormData({...formData, confirmPassword: e.target.value})} className="w-full px-4 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white" />
                  </div>
                </div>
                <div className="pt-4 border-t border-gray-100 flex justify-end">
                  <button type="submit" disabled={saving} className="flex items-center gap-2 px-5 py-2.5 rounded-xl text-sm font-medium text-white" style={{ background: "linear-gradient(135deg, #EA1D2C, #C41420)" }}><FiSave className="h-4 w-4" />{saving ? " Salvando..." : " Atualizar senha"}</button>
                </div>
              </form>
            </div>
          )}

          {activeTab === "notifications" && (
            <div className="bg-white rounded-2xl shadow-card border border-gray-100 p-6 animate-fade-in">
              <h2 className="text-xl font-bold text-gray-900 mb-6">Notificações</h2>
              <form onSubmit={handleSave} className="space-y-4 max-w-xl">
                {[
                  { id: "emailNotifications", label: "Notificações por e-mail", desc: "Receba atualizações por e-mail" },
                  { id: "pushNotifications", label: "Notificações push", desc: "Receba notificações no navegador" },
                  { id: "orderUpdates", label: "Atualizações de pedidos", desc: "Novos pedidos e mudanças de status" },
                  { id: "marketingEmails", label: "E-mails de marketing", desc: "Promoções e novidades" },
                ].map(n => (
                  <label key={n.id} className="flex items-center justify-between p-4 bg-gray-50 rounded-xl hover:bg-gray-100 transition-colors">
                    <div>
                      <p className="font-medium text-gray-900">{n.label}</p>
                      <p className="text-sm text-gray-500">{n.desc}</p>
                    </div>
                    <input type="checkbox" name={n.id} checked={formData[n.id]} onChange={e => setFormData({...formData, [n.id]: e.target.checked})} className="w-5 h-5 rounded border-gray-300 text-fuu-red focus:ring-fuu-red" />
                  </label>
                ))}
                <div className="pt-4 border-t border-gray-100 flex justify-end">
                  <button type="submit" disabled={saving} className="flex items-center gap-2 px-5 py-2.5 rounded-xl text-sm font-medium text-white" style={{ background: "linear-gradient(135deg, #EA1D2C, #C41420)" }}><FiSave className="h-4 w-4" />{saving ? " Salvando..." : " Salvar preferências"}</button>
                </div>
              </form>
            </div>
          )}

          {activeTab === "appearance" && (
            <div className="bg-white rounded-2xl shadow-card border border-gray-100 p-6 animate-fade-in">
              <h2 className="text-xl font-bold text-gray-900 mb-6">Aparência</h2>
              <form onSubmit={handleSave} className="space-y-6 max-w-xl">
                <div>
                  <label className="block text-xs font-semibold text-gray-500 uppercase mb-3">Tema</label>
                  <div className="grid grid-cols-3 gap-3">
                    {["light", "dark", "system"].map(t => (
                      <label key={t} className={`relative cursor-pointer p-4 rounded-xl border-2 transition-all ${formData.theme === t ? "border-fuu-red bg-fuu-red-light" : "border-gray-200 hover:border-gray-300"}`}>
                        <input type="radio" name="theme" value={t} checked={formData.theme === t} onChange={e => setFormData({...formData, theme: e.target.value})} className="sr-only" />
                        <div className="text-center">
                          <p className="font-medium text-gray-900 capitalize">{t}</p>
                          <p className="text-xs text-gray-500 mt-1">{t === "light" ? "Claro" : t === "dark" ? "Escuro" : "Padrão do sistema"}</p>
                        </div>
                      </label>
                    ))}
                  </div>
                </div>
                <div className="space-y-4">
                  {[
                    { id: "compactMode", label: "Modo compacto", desc: "Reduz espaçamento para ver mais conteúdo" },
                    { id: "autoRefresh", label: "Atualização automática", desc: "Atualiza dados automaticamente" },
                  ].map(o => (
                    <label key={o.id} className="flex items-center justify-between p-4 bg-gray-50 rounded-xl hover:bg-gray-100 transition-colors">
                      <div><p className="font-medium text-gray-900">{o.label}</p><p className="text-sm text-gray-500">{o.desc}</p></div>
                      <input type="checkbox" name={o.id} checked={formData[o.id]} onChange={e => setFormData({...formData, [o.id]: e.target.checked})} className="w-5 h-5 rounded border-gray-300 text-fuu-red focus:ring-fuu-red" />
                    </label>
                  ))}
                </div>
                <div>
                  <label className="block text-xs font-semibold text-gray-500 uppercase mb-2">Intervalo de atualização (segundos)</label>
                  <select name="refreshInterval" value={formData.refreshInterval} onChange={e => setFormData({...formData, refreshInterval: parseInt(e.target.value)})} className="w-40 px-4 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white">
                    <option value={15}>15s</option>
                    <option value={30}>30s</option>
                    <option value={60}>60s</option>
                    <option value={120}>2min</option>
                  </select>
                </div>
                <div className="pt-4 border-t border-gray-100 flex justify-end">
                  <button type="submit" disabled={saving} className="flex items-center gap-2 px-5 py-2.5 rounded-xl text-sm font-medium text-white" style={{ background: "linear-gradient(135deg, #EA1D2C, #C41420)" }}><FiSave className="h-4 w-4" />{saving ? " Salvando..." : " Salvar aparência"}</button>
                </div>
              </form>
            </div>
          )}

          {activeTab === "integrations" && (
            <div className="bg-white rounded-2xl shadow-card border border-gray-100 p-6 animate-fade-in">
              <div className="flex items-center justify-between mb-6">
                <h2 className="text-xl font-bold text-gray-900">Integrações</h2>
                <button className="flex items-center gap-2 px-4 py-2 rounded-xl text-white text-sm font-semibold" style={{ background: "linear-gradient(135deg, #EA1D2C, #C41420)" }}><FiPlus className="h-4 w-4" /> Nova integração</button>
              </div>
              <div className="space-y-4">
                {[
                  { name: "Mercado Pago", desc: "Pagamentos via PIX, cartão e boleto", status: "connected", icon: FiCreditCard },
                  { name: "AbacatePay", desc: "Pagamentos via PIX instantâneo", status: "connected", icon: FiCreditCard },
                  { name: "WhatsApp Business API", desc: "Notificações e chat automático", status: "disconnected", icon: FiGlobe },
                  { name: "Google Maps API", desc: "Cálculo de rotas e distâncias", status: "connected", icon: FiGlobe },
                ].map(int => (
                  <div key={int.name} className="flex items-center justify-between p-4 bg-gray-50 rounded-xl">
                    <div className="flex items-center gap-4">
                      <div className="w-12 h-12 rounded-xl bg-fuu-red-light flex items-center justify-center"><int.icon className="h-6 w-6 text-fuu-red" /></div>
                      <div><p className="font-medium text-gray-900">{int.name}</p><p className="text-sm text-gray-500">{int.desc}</p></div>
                    </div>
                    <div className="flex items-center gap-3">
                      <span className={`inline-flex items-center px-3 py-1 rounded-full text-xs font-medium ${int.status === "connected" ? "bg-green-100 text-green-800" : "bg-gray-100 text-gray-600"}`}>
                        {int.status === "connected" ? "Conectado" : "Desconectado"}
                      </span>
                      <button className="p-2 text-gray-400 hover:text-gray-600 hover:bg-gray-200 rounded-lg"><FiSettings className="h-4 w-4" /></button>
                      {int.status === "connected" && <button className="p-2 text-gray-400 hover:text-red-500 hover:bg-red-50 rounded-lg"><FiTrash2 className="h-4 w-4" /></button>}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}
        </main>
      </div>
    </div>
  );
}