import React, { useState } from "react";
import { toast } from "react-toastify";
import Logo from "../../components/Logo";
import {
  FiKey,
  FiUser,
  FiTruck,
  FiShield,
  FiLock,
  FiCheck,
  FiLoader,
  FiArrowLeft,
} from "react-icons/fi";

const API_BASE_URL =
  import.meta.env.REACT_APP_API_URL ||
  import.meta.env.VITE_API_URL ||
  "https://fuudelivery-api-8y6l.onrender.com";

const userTypeOptions = [
  { value: "client", label: "Cliente", icon: FiUser },
  { value: "delivery_man", label: "Entregador", icon: FiTruck },
  { value: "user", label: "Restaurante", icon: FiShield },
];

const ResetarSenhaPage = () => {
  const [step, setStep] = useState("form"); // form | success
  const [userType, setUserType] = useState("client");
  const [identifier, setIdentifier] = useState("");
  const [code, setCode] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const handleSubmit = async (e) => {
    e.preventDefault();
    setError("");

    if (!identifier.trim()) {
      setError("Informe seu telefone ou email");
      return;
    }
    if (!code.trim()) {
      setError("Informe o código recebido do suporte");
      return;
    }
    if (newPassword.length < 6) {
      setError("A nova senha deve ter pelo menos 6 caracteres");
      return;
    }
    if (newPassword !== confirmPassword) {
      setError("As senhas não coincidem");
      return;
    }

    setLoading(true);
    try {
      const res = await fetch(`${API_BASE_URL}/auth/reset-password`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          user_type: userType,
          identifier: identifier.trim(),
          code: code.trim(),
          new_password: newPassword,
        }),
      });

      const data = await res.json();

      if (!res.ok) {
        setError(data.error || "Erro ao redefinir senha");
        return;
      }

      toast.success("Senha redefinida com sucesso!");
      setStep("success");
    } catch (err) {
      setError("Erro de conexão. Tente novamente.");
    } finally {
      setLoading(false);
    }
  };

  if (step === "success") {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-50 px-4">
        <div className="w-full max-w-md text-center">
          <div className="bg-white rounded-2xl shadow-lg p-8">
            <div className="w-16 h-16 rounded-full bg-green-100 flex items-center justify-center mx-auto mb-6">
              <FiCheck className="h-8 w-8 text-green-600" />
            </div>
            <h1 className="text-2xl font-bold text-gray-900 mb-2">
              Senha Redefinida!
            </h1>
            <p className="text-gray-500 mb-6">
              Sua senha foi alterada com sucesso. Agora você pode fazer login
              com a nova senha.
            </p>
            <a
              href="/"
              className="inline-flex items-center gap-2 px-6 py-3 rounded-xl font-semibold text-white transition-all"
              style={{
                background: "linear-gradient(135deg, #DC2626, #B91C1C)",
              }}
            >
              <FiArrowLeft className="h-5 w-5" />
              Ir para o Login
            </a>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen flex">
      {/* Left Panel - Branding */}
      <div
        className="hidden lg:flex lg:w-1/2 relative overflow-hidden items-center justify-center"
        style={{
          background:
            "linear-gradient(135deg, #DC2626 0%, #B91C1C 40%, #7F1D1D 100%)",
        }}
      >
        <div className="absolute inset-0 opacity-10">
          <div
            className="absolute -top-20 -left-20 w-96 h-96 rounded-full"
            style={{
              background: "radial-gradient(circle, #F59E0B 0%, transparent 70%)",
            }}
          />
        </div>
        <div className="relative z-10 text-center px-12">
          <Logo size={70} variant="white" />
          <div className="mt-10 text-white">
            <h2 className="text-3xl font-bold mb-4" style={{ lineHeight: 1.2 }}>
              Redefinir sua senha
            </h2>
            <p className="text-white/80 text-lg">
              Use o código fornecido pelo suporte para criar uma nova senha.
            </p>
          </div>
        </div>
      </div>

      {/* Right Panel - Form */}
      <div className="w-full lg:w-1/2 flex items-center justify-center px-6 py-12 bg-white">
        <div className="w-full max-w-md">
          {/* Logo mobile */}
          <div className="lg:hidden flex justify-center mb-8">
            <Logo size={50} />
          </div>

          <div className="mb-8">
            <div className="flex items-center gap-3 mb-2">
              <div
                className="w-10 h-10 rounded-xl flex items-center justify-center"
                style={{
                  background: "linear-gradient(135deg, #DC2626, #B91C1C)",
                }}
              >
                <FiKey className="h-5 w-5 text-white" />
              </div>
              <h1 className="text-2xl font-bold text-gray-900">
                Esqueci minha senha
              </h1>
            </div>
            <p className="text-sm text-gray-500 mt-1">
              Entre em contato com o suporte para receber um código de
              redefinição.
            </p>
          </div>

          {/* Error */}
          {error && (
            <div className="bg-red-50 border border-red-200 rounded-xl p-4 mb-6">
              <p className="text-sm text-red-700">{error}</p>
            </div>
          )}

          <form onSubmit={handleSubmit} className="space-y-5">
            {/* Tipo de conta */}
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Tipo de conta
              </label>
              <div className="grid grid-cols-3 gap-2">
                {userTypeOptions.map((opt) => {
                  const Icon = opt.icon;
                  return (
                    <button
                      key={opt.value}
                      type="button"
                      onClick={() => setUserType(opt.value)}
                      className={`flex flex-col items-center gap-1.5 p-3 rounded-xl border-2 transition-all text-sm ${
                        userType === opt.value
                          ? "border-red-500 bg-red-50"
                          : "border-gray-200 hover:border-gray-300"
                      }`}
                    >
                      <Icon
                        className={`h-5 w-5 ${
                          userType === opt.value
                            ? "text-red-600"
                            : "text-gray-400"
                        }`}
                      />
                      <span
                        className={
                          userType === opt.value
                            ? "font-medium text-gray-900"
                            : "text-gray-600"
                        }
                      >
                        {opt.label}
                      </span>
                    </button>
                  );
                })}
              </div>
            </div>

            {/* Identificador */}
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Telefone ou email
              </label>
              <input
                type="text"
                value={identifier}
                onChange={(e) => setIdentifier(e.target.value)}
                placeholder="Seu telefone ou email cadastrado"
                className="w-full px-4 py-3 bg-gray-50 border border-gray-200 rounded-xl text-gray-900 placeholder-gray-400 focus:bg-white focus:ring-2 focus:ring-red-500 focus:border-transparent transition-all"
              />
            </div>

            {/* Código */}
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Código de redefinição
              </label>
              <input
                type="text"
                value={code}
                onChange={(e) => setCode(e.target.value.toUpperCase())}
                placeholder="Ex: ABCD1234"
                maxLength={8}
                className="w-full px-4 py-3 bg-gray-50 border border-gray-200 rounded-xl text-gray-900 placeholder-gray-400 font-mono text-lg tracking-widest text-center focus:bg-white focus:ring-2 focus:ring-red-500 focus:border-transparent transition-all uppercase"
              />
              <p className="text-xs text-gray-400 mt-1">
                Código de 8 caracteres informado pelo suporte
              </p>
            </div>

            {/* Nova senha */}
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Nova senha
              </label>
              <div className="relative">
                <FiLock className="absolute left-3 top-1/2 -translate-y-1/2 h-5 w-5 text-gray-400" />
                <input
                  type="password"
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                  placeholder="Mínimo 6 caracteres"
                  className="w-full pl-10 pr-4 py-3 bg-gray-50 border border-gray-200 rounded-xl text-gray-900 placeholder-gray-400 focus:bg-white focus:ring-2 focus:ring-red-500 focus:border-transparent transition-all"
                />
              </div>
            </div>

            {/* Confirmar senha */}
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Confirmar nova senha
              </label>
              <div className="relative">
                <FiLock className="absolute left-3 top-1/2 -translate-y-1/2 h-5 w-5 text-gray-400" />
                <input
                  type="password"
                  value={confirmPassword}
                  onChange={(e) => setConfirmPassword(e.target.value)}
                  placeholder="Digite a senha novamente"
                  className="w-full pl-10 pr-4 py-3 bg-gray-50 border border-gray-200 rounded-xl text-gray-900 placeholder-gray-400 focus:bg-white focus:ring-2 focus:ring-red-500 focus:border-transparent transition-all"
                />
              </div>
            </div>

            {/* Botão */}
            <button
              type="submit"
              disabled={loading}
              className="w-full py-3 px-4 rounded-xl font-semibold text-white transition-all disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
              style={{
                background: loading
                  ? "#9CA3AF"
                  : "linear-gradient(135deg, #DC2626, #B91C1C)",
              }}
            >
              {loading ? (
                <>
                  <FiLoader className="animate-spin h-5 w-5" />
                  Redefinindo...
                </>
              ) : (
                <>
                  <FiCheck className="h-5 w-5" />
                  Redefinir Senha
                </>
              )}
            </button>
          </form>

          {/* Link volta */}
          <div className="mt-6 text-center">
            <a
              href="/"
              className="text-sm text-gray-500 hover:text-red-600 transition-colors inline-flex items-center gap-1"
            >
              <FiArrowLeft className="h-4 w-4" />
              Voltar para o login
            </a>
          </div>
        </div>
      </div>
    </div>
  );
};

export default ResetarSenhaPage;
