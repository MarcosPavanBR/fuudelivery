import React, { useState, useEffect, useRef } from "react";
import {
  FiUser,
  FiMail,
  FiPhone,
  FiCamera,
  FiTrash2,
  FiSave,
  FiX,
  FiUpload,
  FiLoader,
} from "react-icons/fi";
import { useAuth } from "../context/AuthContext";
import api from "../services/api";
import { toast } from "react-toastify";

/* ============================================================
   ProfileSettings — Tela de Perfil / Configurações da Conta
   ============================================================
   - Alinhada ao backend do monolito:
       GET  /users/:id            (id do token/AuthContext)
       POST /upload/avatars       (multipart "file" → {url})
       PUT  /users/:id            (JSON: name, email, phone, avatar_url)
   - Grid consistente (2 colunas; telefone ocupa metade da linha)
   - Avatar editável com preview circular (persistido no backend)
   - Máscara de telefone brasileiro (XX) XXXXX-XXXX
   - Estados de loading (skeleton), erro e sucesso
   - Design System Fuu: cards 12px, inputs 8px, salvar = success (verde)
   ============================================================ */

const INITIAL_FORM = {
  nome: "",
  email: "",
  telefone: "",
};

const INITIAL_ERRORS = {
  nome: "",
  email: "",
  telefone: "",
  avatar: "",
  general: "",
};

// Máscara de telefone brasileiro progressiva: (XX) XXXXX-XXXX
function maskPhone(value) {
  const digits = (value || "").replace(/\D/g, "").slice(0, 11);
  if (digits.length === 0) return "";
  if (digits.length <= 2) return `(${digits}`;
  if (digits.length <= 6) return `(${digits.slice(0, 2)}) ${digits.slice(2)}`;
  if (digits.length <= 10)
    return `(${digits.slice(0, 2)}) ${digits.slice(2, 6)}-${digits.slice(6)}`;
  return `(${digits.slice(0, 2)}) ${digits.slice(2, 7)}-${digits.slice(7)}`;
}

const unmaskPhone = (value) => (value || "").replace(/\D/g, "");

export default function ProfileSettings() {
  const { user } = useAuth();
  const [form, setForm] = useState(INITIAL_FORM);
  const [original, setOriginal] = useState(INITIAL_FORM);
  const [errors, setErrors] = useState(INITIAL_ERRORS);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [avatar, setAvatar] = useState(
    () => localStorage.getItem("fuu_admin_avatar") || user?.avatar_url || ""
  );
  const fileInputRef = useRef(null);

  /* ---------- Carrega dados do usuário do backend (fonte da verdade) ---------- */
  useEffect(() => {
    const loadProfile = async () => {
      try {
        if (user?.id) {
          const res = await api.get(`/users/${user.id}`);
          const data = res.data || {};
          const mapped = {
            nome: data.name || data.nome || "",
            email: data.email || "",
            telefone: maskPhone(data.phone || data.telefone || ""),
          };
          setForm(mapped);
          setOriginal(mapped);
          if (data.avatar_url) {
            setAvatar(data.avatar_url);
            localStorage.setItem("fuu_admin_avatar", data.avatar_url);
          }
        } else {
          // Sem sessão — formulário vazio (raro: painel sempre tem sessão)
          const empty = { nome: "", email: "", telefone: "" };
          setForm(empty);
          setOriginal(empty);
        }
      } catch (e) {
        console.error("Erro ao carregar perfil:", e);
        const fallback = {
          nome: user?.name || "",
          email: user?.email || "",
          telefone: "",
        };
        setForm(fallback);
        setOriginal(fallback);
      }
      setLoading(false);
    };
    loadProfile();
  }, [user?.id]); // eslint-disable-line react-hooks/exhaustive-deps

  /* ---------- Handlers ---------- */
  const handleChange = (e) => {
    const { name, value } = e.target;
    setForm((prev) => ({
      ...prev,
      [name]: name === "telefone" ? maskPhone(value) : value,
    }));
    if (errors[name]) setErrors((prev) => ({ ...prev, [name]: "" }));
    if (errors.general) setErrors((prev) => ({ ...prev, general: "" }));
  };

  const handleAvatarClick = () => fileInputRef.current?.click();

  const handleFileChange = async (e) => {
    const file = e.target.files?.[0];
    if (!file) return;
    if (!file.type.startsWith("image/")) {
      setErrors((prev) => ({ ...prev, avatar: "Apenas imagens (JPG, PNG, GIF) são permitidas." }));
      return;
    }
    if (file.size > 2 * 1024 * 1024) {
      setErrors((prev) => ({ ...prev, avatar: "A imagem deve ter no máximo 2MB." }));
      return;
    }
    setErrors((prev) => ({ ...prev, avatar: "" }));

    // Preview instantâneo
    const url = URL.createObjectURL(file);
    setAvatar(url);

    // Upload real: POST /upload/avatars → {url} → PUT /users/:id {avatar_url}
    try {
      const fd = new FormData();
      fd.append("file", file);
      const { data } = await api.post("/upload/avatars", fd, {
        headers: { "Content-Type": "multipart/form-data" },
      });
      if (!data?.url) throw new Error("Upload sem URL");
      await api.put(`/users/${user?.id}`, { avatar_url: data.url });
      setAvatar(data.url);
      localStorage.setItem("fuu_admin_avatar", data.url);
      toast.success("Foto atualizada!");
    } catch (err) {
      console.error("Erro no upload do avatar:", err);
      setAvatar((prev) =>
        prev === url ? localStorage.getItem("fuu_admin_avatar") || user?.avatar_url || "" : prev
      );
      toast.error(err?.response?.data?.error || "Erro ao enviar foto");
    } finally {
      if (fileInputRef.current) fileInputRef.current.value = "";
    }
  };

  const handleRemoveAvatar = async () => {
    setAvatar("");
    localStorage.removeItem("fuu_admin_avatar");
    try {
      await api.put(`/users/${user?.id}`, { avatar_url: "" });
      toast.success("Foto removida");
    } catch (err) {
      toast.error(err?.response?.data?.error || "Erro ao remover foto");
    }
    if (fileInputRef.current) fileInputRef.current.value = "";
  };

  /* ---------- Validação ---------- */
  const validate = () => {
    const next = { ...INITIAL_ERRORS };
    let isValid = true;

    if (!form.nome.trim()) {
      next.nome = "Nome completo é obrigatório.";
      isValid = false;
    } else if (form.nome.trim().length < 3) {
      next.nome = "Nome deve ter pelo menos 3 caracteres.";
      isValid = false;
    }

    if (!form.email.trim()) {
      next.email = "E-mail é obrigatório.";
      isValid = false;
    } else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(form.email)) {
      next.email = "Digite um e-mail válido.";
      isValid = false;
    }

    const phoneDigits = unmaskPhone(form.telefone);
    if (phoneDigits && phoneDigits.length < 10) {
      next.telefone = "Telefone incompleto.";
      isValid = false;
    }

    setErrors(next);
    return isValid;
  };

  /* ---------- Submit ---------- */
  const handleSubmit = async (e) => {
    e.preventDefault();
    if (!validate()) return;

    setSaving(true);
    try {
      await api.put(`/users/${user?.id}`, {
        name: form.nome.trim(),
        email: form.email.trim(),
        phone: unmaskPhone(form.telefone),
      });

      const updated = { ...form, telefone: maskPhone(form.telefone) };
      setOriginal(updated);
      toast.success("Perfil atualizado com sucesso!");
    } catch (err) {
      console.error("Erro ao salvar:", err);
      toast.error(err?.response?.data?.error || "Erro ao salvar alterações. Tente novamente.");
    } finally {
      setSaving(false);
    }
  };

  const handleCancel = () => {
    setForm(original);
    setErrors(INITIAL_ERRORS);
  };

  const hasChanges =
    form.nome !== original.nome ||
    form.email !== original.email ||
    form.telefone !== original.telefone;

  /* ============================================================
     RENDER — Loading / Skeleton
     ============================================================ */
  if (loading) {
    return (
      <div className="animate-fade-in space-y-8 min-w-0 max-w-3xl">
        <div className="space-y-2">
          <div className="skeleton h-8 w-48" />
          <div className="skeleton h-4 w-64" />
        </div>
        <div className="card p-8 space-y-6">
          <div className="flex items-center gap-6 pb-8 border-b border-gray-100">
            <div className="skeleton h-20 w-20 rounded-full" />
            <div className="space-y-2">
              <div className="skeleton h-4 w-32" />
              <div className="skeleton h-3 w-48" />
            </div>
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-6">
            <div className="space-y-2">
              <div className="skeleton h-3 w-24" />
              <div className="skeleton h-10 w-full rounded-lg" />
            </div>
            <div className="space-y-2">
              <div className="skeleton h-3 w-24" />
              <div className="skeleton h-10 w-full rounded-lg" />
            </div>
          </div>
          <div className="space-y-2 sm:max-w-[calc(50%-12px)]">
            <div className="skeleton h-3 w-24" />
            <div className="skeleton h-10 w-full rounded-lg" />
          </div>
        </div>
      </div>
    );
  }

  /* ============================================================
     RENDER — Formulário
     ============================================================ */
  return (
    <div className="animate-fade-in space-y-8 min-w-0 max-w-3xl">
      {/* Header da página */}
      <div>
        <p className="text-xs font-semibold uppercase tracking-wider text-gray-500 mb-1">
          Configurações da conta
        </p>
        <h1 className="text-2xl sm:text-[28px] font-bold text-gray-900 tracking-tight">
          Perfil
        </h1>
      </div>

      {/* Mensagem de erro geral */}
      {errors.general && (
        <div className="flex items-center gap-4 px-4 py-4 rounded-xl bg-red-50 border border-red-200 text-red-700 text-sm font-medium animate-slide-up">
          <span className="w-5 h-5 rounded-full bg-red-500 text-white flex items-center justify-center text-xs font-bold">!</span>
          {errors.general}
        </div>
      )}

      <form onSubmit={handleSubmit} className="card p-8 space-y-8">
        {/* ---------- Seção Avatar ---------- */}
        <div className="flex items-center gap-6 pb-8 border-b border-gray-100">
          <div className="relative flex-shrink-0">
            <div
              className="w-20 h-20 rounded-full flex items-center justify-center text-white text-2xl font-bold overflow-hidden"
              style={{
                background: avatar
                  ? "transparent"
                  : "linear-gradient(135deg, #EA1D2C, #FF6B35)",
                boxShadow: "0 4px 16px rgba(234, 29, 44, 0.25)",
              }}
            >
              {avatar ? (
                <img src={avatar} alt="Avatar" className="w-full h-full object-cover" />
              ) : (
                form.nome?.charAt(0)?.toUpperCase() || "A"
              )}
            </div>
            <button
              type="button"
              onClick={handleAvatarClick}
              className="absolute bottom-0 right-0 w-7 h-7 rounded-full bg-fuu-red text-white flex items-center justify-center border-[3px] border-white cursor-pointer hover:bg-fuu-red-dark transition-colors"
              style={{ boxShadow: "0 2px 8px rgba(234, 29, 44, 0.3)" }}
              title="Alterar foto"
            >
              <FiCamera className="w-3.5 h-3.5" />
            </button>
            <input
              ref={fileInputRef}
              type="file"
              accept="image/jpeg,image/png,image/gif"
              onChange={handleFileChange}
              className="hidden"
            />
          </div>

          <div className="min-w-0">
            <h3 className="text-base font-semibold text-gray-900">Foto de perfil</h3>
            <p className="text-sm text-gray-500 mt-1">JPG, PNG ou GIF. Máximo 2MB.</p>
            {errors.avatar && <p className="text-xs text-red-600 mt-1">{errors.avatar}</p>}
            <div className="flex items-center gap-2 mt-4">
              <button type="button" onClick={handleAvatarClick} className="btn btn-ghost text-xs">
                <FiUpload className="w-3.5 h-3.5" />
                Alterar foto
              </button>
              {avatar && (
                <button
                  type="button"
                  onClick={handleRemoveAvatar}
                  className="btn btn-ghost text-xs text-red-600 hover:bg-red-50 border-transparent"
                >
                  <FiTrash2 className="w-3.5 h-3.5" />
                  Remover
                </button>
              )}
            </div>
          </div>
        </div>

        {/* ---------- Grid do Formulário ---------- */}
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-6">
          {/* Nome Completo */}
          <div className="space-y-2">
            <label htmlFor="nome" className="block text-xs font-semibold text-gray-500 uppercase mb-2 tracking-wide">
              Nome completo <span className="text-fuu-red">*</span>
            </label>
            <div className="relative">
              <FiUser className="absolute left-3.5 top-1/2 -translate-y-1/2 text-gray-400 w-4 h-4" />
              <input
                id="nome"
                name="nome"
                type="text"
                value={form.nome}
                onChange={handleChange}
                placeholder="Seu nome completo"
                className={`input pl-10 ${errors.nome ? "is-invalid" : ""}`}
                autoComplete="name"
              />
            </div>
            {errors.nome && <p className="text-xs text-red-600 font-medium">{errors.nome}</p>}
          </div>

          {/* E-mail */}
          <div className="space-y-2">
            <label htmlFor="email" className="block text-xs font-semibold text-gray-500 uppercase mb-2 tracking-wide">
              E-mail <span className="text-fuu-red">*</span>
            </label>
            <div className="relative">
              <FiMail className="absolute left-3.5 top-1/2 -translate-y-1/2 text-gray-400 w-4 h-4" />
              <input
                id="email"
                name="email"
                type="email"
                value={form.email}
                onChange={handleChange}
                placeholder="seu@email.com"
                className={`input pl-10 ${errors.email ? "is-invalid" : ""}`}
                autoComplete="email"
              />
            </div>
            {errors.email && <p className="text-xs text-red-600 font-medium">{errors.email}</p>}
          </div>
        </div>

        {/* Telefone — alinhado à esquerda, metade da linha (grid de 2 colunas) */}
        <div className="sm:max-w-[calc(50%-12px)]">
          <label htmlFor="telefone" className="block text-xs font-semibold text-gray-500 uppercase mb-2 tracking-wide">
            Telefone
          </label>
          <div className="relative">
            <FiPhone className="absolute left-3.5 top-1/2 -translate-y-1/2 text-gray-400 w-4 h-4" />
            <input
              id="telefone"
              name="telefone"
              type="tel"
              value={form.telefone}
              onChange={handleChange}
              placeholder="(00) 00000-0000"
              className={`input pl-10 ${errors.telefone ? "is-invalid" : ""}`}
              autoComplete="tel"
            />
          </div>
          {errors.telefone ? (
            <p className="text-xs text-red-600 font-medium mt-1">{errors.telefone}</p>
          ) : (
            <p className="text-xs text-gray-400 mt-1">Formato: (XX) XXXXX-XXXX</p>
          )}
        </div>

        {/* ---------- Ações ---------- */}
        <div className="flex flex-col-reverse sm:flex-row items-stretch sm:items-center justify-end gap-4 pt-8 border-t border-gray-100">
          <button
            type="button"
            onClick={handleCancel}
            disabled={!hasChanges || saving}
            className="btn btn-ghost disabled:opacity-50 disabled:cursor-not-allowed"
          >
            <FiX className="w-4 h-4" />
            Cancelar
          </button>

          <button
            type="submit"
            disabled={!hasChanges || saving}
            className="btn btn-primary disabled:opacity-60 disabled:cursor-not-allowed"
          >
            {saving ? (
              <>
                <FiLoader className="w-4 h-4 animate-spin" />
                Salvando...
              </>
            ) : (
              <>
                <FiSave className="w-4 h-4" />
                Salvar alterações
              </>
            )}
          </button>
        </div>
      </form>
    </div>
  );
}
