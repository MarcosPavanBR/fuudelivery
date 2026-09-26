import React, { useState, useEffect } from "react";
import { FiUser, FiShield, FiLoader, FiKey, FiLogOut, FiActivity, FiCheckCircle, FiAlertTriangle, FiRefreshCw } from "react-icons/fi";
import { useAuth } from "../context/AuthContext";
import api from "../services/api";
import { toast } from "react-toastify";
import ProfileSettings from "./ProfileSettings";

// As abas "Notificações", "Aparência" e "Integrações" eram de mentira: o
// salvar esperava 600 ms e dizia "salvo", e as integrações mostravam
// "Conectado" fixo no código. No lugar, "Status do sistema" pergunta ao
// servidor o que está de fato ligado (GET /admin/system-status).
const tabs = [
  { id: "profile", label: "Perfil", icon: FiUser },
  { id: "security", label: "Segurança", icon: FiShield },
  { id: "status", label: "Status do sistema", icon: FiActivity },
];

const emptyPasswords = { currentPassword: "", newPassword: "", confirmPassword: "" };

function SecurityTab({ user }) {
  const [form, setForm] = useState(emptyPasswords);
  const [saving, setSaving] = useState(false);
  const set = (f) => (e) => setForm({ ...form, [f]: e.target.value });

  const submit = async (e) => {
    e.preventDefault();
    if (form.newPassword.length < 6) return toast.error("A nova senha precisa de pelo menos 6 caracteres");
    if (form.newPassword !== form.confirmPassword) return toast.error("As senhas não conferem");
    setSaving(true);
    try {
      await api.put(`/users/${user?.id}/password`, { current_password: form.currentPassword, new_password: form.newPassword });
      toast.success("Senha atualizada!");
      setForm(emptyPasswords);
    } catch (err) {
      toast.error(err?.response?.data?.error || "Erro ao atualizar a senha");
    }
    setSaving(false);
  };

  return (
    <div className="card p-6 animate-fade-in">
      <h2 className="text-xl font-bold text-gray-900 mb-6">Trocar senha</h2>
      <form onSubmit={submit} className="space-y-6 max-w-2xl">
        <div>
          <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">Senha atual <span className="text-red-500">*</span></label>
          <input type="password" required autoComplete="current-password" value={form.currentPassword} onChange={set("currentPassword")} className="input" />
        </div>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-6">
          <div>
            <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">Nova senha <span className="text-red-500">*</span></label>
            <input type="password" required autoComplete="new-password" value={form.newPassword} onChange={set("newPassword")} className="input" placeholder="Mínimo 6 caracteres" />
          </div>
          <div>
            <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">Confirmar senha <span className="text-red-500">*</span></label>
            <input type="password" required autoComplete="new-password" value={form.confirmPassword} onChange={set("confirmPassword")} className="input" />
          </div>
        </div>
        <div className="pt-4 border-t border-gray-100 flex justify-end">
          <button type="submit" disabled={saving} className="btn btn-primary">
            {saving ? <FiLoader className="h-4 w-4 animate-spin" /> : <FiKey className="h-4 w-4" />}
            {saving ? " Atualizando..." : " Atualizar senha"}
          </button>
        </div>
      </form>
    </div>
  );
}

function StatusTab() {
  const [items, setItems] = useState(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  const load = async () => {
    setLoading(true);
    setError("");
    try {
      const { data } = await api.get("/admin/system-status");
      setItems(Array.isArray(data) ? data : []);
    } catch (e) {
      setError(e?.response?.data?.error || "Não foi possível consultar o servidor");
    }
    setLoading(false);
  };

  useEffect(() => { load(); }, []);

  const problems = (items || []).filter((i) => !i.ok).length;

  return (
    <div className="card p-6 animate-fade-in">
      <div className="flex items-center justify-between mb-2 gap-4">
        <h2 className="text-xl font-bold text-gray-900">Status do sistema</h2>
        <button onClick={load} disabled={loading} className="btn btn-ghost text-xs"><FiRefreshCw className={`w-3.5 h-3.5 ${loading ? "animate-spin" : ""}`} /> Verificar de novo</button>
      </div>
      <p className="text-sm text-gray-500 mb-6">
        O que está ligado no servidor agora. {items && (problems ? `${problems} item(ns) precisam de atenção.` : "Tudo certo.")}
      </p>
      {error && <p className="text-sm text-red-600">{error}</p>}
      {!items && !error && <div className="skeleton h-24 w-full" />}
      <div className="space-y-3">
        {(items || []).map((it) => (
          <div key={it.key} className={`flex items-start gap-3 p-4 rounded-xl border ${it.ok ? "bg-gray-50 border-gray-100" : "bg-amber-50 border-amber-200"}`}>
            {it.ok ? <FiCheckCircle className="h-5 w-5 text-green-600 flex-shrink-0 mt-0.5" /> : <FiAlertTriangle className="h-5 w-5 text-amber-600 flex-shrink-0 mt-0.5" />}
            <div className="min-w-0">
              <p className="font-medium text-gray-900">{it.label}</p>
              <p className="text-sm text-gray-600">{it.detail}</p>
              {it.fix && <p className="text-xs text-amber-800 mt-1">Como resolver: {it.fix}</p>}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

export default function Settings() {
  const { user, logout } = useAuth();
  const [activeTab, setActiveTab] = useState("profile");

  return (
    <div className="animate-fade-in">
      {activeTab !== "profile" && (
        <div className="mb-8 px-1">
          <p className="text-sm text-gray-500">Configurações da conta</p>
          <h1 className="text-2xl font-bold text-gray-900 mt-1">Configurações</h1>
        </div>
      )}

      <div className="flex flex-col lg:flex-row gap-8">
        <aside className="lg:w-64 flex-shrink-0">
          <nav className="bg-white rounded-2xl shadow-card border border-gray-100 overflow-hidden">
            {tabs.map((tab) => (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id)}
                className={`relative w-full flex items-center gap-2 px-6 py-4 transition-all duration-200 border-b border-gray-100 last:border-0 ${
                  activeTab === tab.id ? "bg-fuu-red-light text-fuu-red font-semibold" : "text-gray-600 hover:bg-gray-50 hover:text-gray-900"
                }`}
              >
                {activeTab === tab.id && <span className="absolute left-0 top-1/2 -translate-y-1/2 w-1 h-7 bg-fuu-red rounded-r-full" />}
                <tab.icon className="h-5 w-5 flex-shrink-0" />
                {tab.label}
              </button>
            ))}
            <div className="border-t border-gray-100 p-4">
              <button onClick={() => logout()} className="w-full flex items-center gap-2 px-4 py-4 text-red-600 hover:bg-red-50 rounded-xl transition-colors font-medium">
                <FiLogOut className="h-5 w-5" />
                Sair da conta
              </button>
            </div>
          </nav>
        </aside>

        <main className="flex-1 min-w-0">
          {activeTab === "profile" && <ProfileSettings />}
          {activeTab === "security" && <SecurityTab user={user} />}
          {activeTab === "status" && <StatusTab />}
        </main>
      </div>
    </div>
  );
}
