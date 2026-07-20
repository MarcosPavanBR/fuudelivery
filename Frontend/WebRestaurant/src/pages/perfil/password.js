import React, { useState } from "react";
import { useAuth } from "../../context/AuthContext";
import MenuLayout from "../../components/Menu";
import {
  FiLock,
  FiEye,
  FiEyeOff,
  FiSave,
  FiLoader,
  FiArrowLeft,
} from "react-icons/fi";
import { toast } from "react-toastify";
import Texts from "../../constants/Texts";
import api from "../../services/api";

const ChangePasswordPage = () => {
  const { getUser } = useAuth();
  const user = getUser();
  const [form, setForm] = useState({
    currentPassword: "",
    newPassword: "",
    confirmPassword: "",
  });
  const [showCurrent, setShowCurrent] = useState(false);
  const [showNew, setShowNew] = useState(false);
  const [showConfirm, setShowConfirm] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const handleChange = (e) => {
    setForm({ ...form, [e.target.name]: e.target.value });
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    setError("");

    if (!form.currentPassword || !form.newPassword || !form.confirmPassword) {
      setError("Preencha todos os campos.");
      return;
    }

    if (form.newPassword !== form.confirmPassword) {
      setError("As senhas não coincidem.");
      return;
    }

    if (form.newPassword.length < 6) {
      setError("A nova senha deve ter pelo menos 6 caracteres.");
      return;
    }

    if (form.currentPassword === form.newPassword) {
      setError("A nova senha deve ser diferente da atual.");
      return;
    }

    setLoading(true);
    try {
      await api.put(`/users/${user.id}/password`, {
        current_password: form.currentPassword,
        new_password: form.newPassword,
      });
      toast.success("Senha alterada com sucesso!");
      setForm({ currentPassword: "", newPassword: "", confirmPassword: "" });
    } catch (err) {
      const msg =
        err.response?.data?.error || "Erro ao alterar senha. Tente novamente.";
      setError(msg);
    }
    setLoading(false);
  };

  const inputClass =
    "appearance-none block w-full pl-10 pr-10 py-3 border border-gray-200 rounded-xl placeholder-gray-400 text-gray-900 bg-gray-50 focus:bg-white transition-colors";

  return (
    <MenuLayout>
      <div className="max-w-lg mx-auto">
        <div className="mb-6">
          <a
            href="/#/perfil"
            className="flex items-center gap-2 text-sm text-gray-500 hover:text-gray-700 mb-4 transition-colors"
          >
            <FiArrowLeft className="h-4 w-4" />
            Voltar ao perfil
          </a>
          <h1
            className="text-2xl font-bold"
            style={{ color: "var(--text-primary, #1A1A1A)" }}
          >
            Alterar Senha
          </h1>
          <p
            className="text-sm mt-1"
            style={{ color: "var(--text-secondary, #666)" }}
          >
            Mantenha sua conta segura com uma senha forte
          </p>
        </div>

        <div
          className="rounded-2xl p-6 sm:p-8"
          style={{
            background: "var(--bg-card, #FFFFFF)",
            boxShadow: "var(--shadow-card, 0 1px 3px rgba(0,0,0,0.08))",
          }}
        >
          {error && (
            <div className="mb-4 p-3 rounded-lg bg-red-50 border border-red-200 text-red-700 text-sm animate-slide-up">
              {error}
            </div>
          )}

          <form className="space-y-5" onSubmit={handleSubmit}>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1.5">
                Senha atual
              </label>
              <div className="relative">
                <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                  <FiLock className="h-5 w-5 text-gray-400" />
                </div>
                <input
                  name="currentPassword"
                  type={showCurrent ? "text" : "password"}
                  required
                  className={inputClass}
                  placeholder="Sua senha atual"
                  value={form.currentPassword}
                  onChange={handleChange}
                />
                <button
                  type="button"
                  onClick={() => setShowCurrent(!showCurrent)}
                  className="absolute inset-y-0 right-0 pr-3 flex items-center text-gray-400 hover:text-gray-600"
                >
                  {showCurrent ? (
                    <FiEyeOff className="h-5 w-5" />
                  ) : (
                    <FiEye className="h-5 w-5" />
                  )}
                </button>
              </div>
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1.5">
                Nova senha
              </label>
              <div className="relative">
                <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                  <FiLock className="h-5 w-5 text-gray-400" />
                </div>
                <input
                  name="newPassword"
                  type={showNew ? "text" : "password"}
                  required
                  className={inputClass}
                  placeholder="Mínimo 6 caracteres"
                  value={form.newPassword}
                  onChange={handleChange}
                />
                <button
                  type="button"
                  onClick={() => setShowNew(!showNew)}
                  className="absolute inset-y-0 right-0 pr-3 flex items-center text-gray-400 hover:text-gray-600"
                >
                  {showNew ? (
                    <FiEyeOff className="h-5 w-5" />
                  ) : (
                    <FiEye className="h-5 w-5" />
                  )}
                </button>
              </div>
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1.5">
                Confirmar nova senha
              </label>
              <div className="relative">
                <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                  <FiLock className="h-5 w-5 text-gray-400" />
                </div>
                <input
                  name="confirmPassword"
                  type={showConfirm ? "text" : "password"}
                  required
                  className={inputClass}
                  placeholder="Repita a nova senha"
                  value={form.confirmPassword}
                  onChange={handleChange}
                />
                <button
                  type="button"
                  onClick={() => setShowConfirm(!showConfirm)}
                  className="absolute inset-y-0 right-0 pr-3 flex items-center text-gray-400 hover:text-gray-600"
                >
                  {showConfirm ? (
                    <FiEyeOff className="h-5 w-5" />
                  ) : (
                    <FiEye className="h-5 w-5" />
                  )}
                </button>
              </div>
            </div>

            <button
              type="submit"
              disabled={loading}
              className="w-full flex items-center justify-center gap-2 py-3 px-4 rounded-xl text-white font-semibold text-sm transition-all duration-200 hover:shadow-lg hover:scale-[1.02] active:scale-[0.98] disabled:opacity-50 disabled:cursor-not-allowed disabled:hover:scale-100"
              style={{
                background: "linear-gradient(135deg, #EA1D2C, #C41420)",
              }}
            >
              {loading ? (
                <>
                  <FiLoader className="animate-spin h-5 w-5" />
                  Salvando...
                </>
              ) : (
                <>
                  <FiSave className="h-5 w-5" />
                  Alterar Senha
                </>
              )}
            </button>
          </form>
        </div>
      </div>
    </MenuLayout>
  );
};

export default ChangePasswordPage;
