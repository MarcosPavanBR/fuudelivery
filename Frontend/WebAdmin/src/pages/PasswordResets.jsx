import React, { useState } from "react";
import {
  FiKey,
  FiUser,
  FiTruck,
  FiShield,
  FiCopy,
  FiCheck,
  FiAlertCircle,
} from "react-icons/fi";
import api from "../services/api";
import { toast } from "react-toastify";

const userTypeOptions = [
  { value: "client", label: "Cliente", icon: FiUser, color: "text-blue-600" },
  {
    value: "delivery_man",
    label: "Entregador",
    icon: FiTruck,
    color: "text-green-600",
  },
  { value: "user", label: "Restaurante/Admin", icon: FiShield, color: "text-purple-600" },
];

export default function PasswordResets() {
  const [userType, setUserType] = useState("client");
  const [identifier, setIdentifier] = useState("");
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState(null);
  const [copied, setCopied] = useState(false);

  const handleGenerate = async (e) => {
    e.preventDefault();
    if (!identifier.trim()) {
      toast.error("Informe o telefone ou email do usuário");
      return;
    }

    setLoading(true);
    setResult(null);
    setCopied(false);

    try {
      const { data } = await api.post("/admin/password-reset/code", {
        user_type: userType,
        identifier: identifier.trim(),
      });
      setResult(data);
      toast.success("Código gerado com sucesso!");
    } catch (err) {
      const msg =
        err.response?.data?.error || "Erro ao gerar código. Tente novamente.";
      toast.error(msg);
    } finally {
      setLoading(false);
    }
  };

  const handleCopyCode = () => {
    if (result?.code) {
      navigator.clipboard.writeText(result.code);
      setCopied(true);
      toast.success("Código copiado!");
      setTimeout(() => setCopied(false), 2000);
    }
  };

  const handleNewCode = () => {
    setResult(null);
    setIdentifier("");
    setCopied(false);
  };

  return (
    <div className="max-w-2xl mx-auto">
      <div className="mb-8">
        <div className="flex items-center gap-3 mb-2">
          <div
            className="w-10 h-10 rounded-xl flex items-center justify-center"
            style={{ background: "linear-gradient(135deg, #DC2626, #B91C1C)" }}
          >
            <FiKey className="h-5 w-5 text-white" />
          </div>
          <div>
            <h1 className="text-2xl font-bold text-gray-900">
              Reset de Senha
            </h1>
            <p className="text-sm text-gray-500">
              Gerar código de uso único para o usuário
            </p>
          </div>
        </div>
      </div>

      {/* Instruções */}
      <div className="bg-amber-50 border border-amber-200 rounded-xl p-4 mb-6">
        <div className="flex gap-3">
          <FiAlertCircle className="h-5 w-5 text-amber-600 flex-shrink-0 mt-0.5" />
          <div className="text-sm text-amber-800">
            <p className="font-semibold mb-1">Como funciona:</p>
            <ol className="list-decimal list-inside space-y-1">
              <li>Selecione o tipo de conta e informe o telefone/email</li>
              <li>Clique em "Gerar Código"</li>
              <li>Informe o código ao usuário por <strong>telefone ou WhatsApp</strong></li>
              <li>O usuário acessa <code className="bg-amber-100 px-1 rounded">/resetar-senha</code> no site e digita o código</li>
            </ol>
            <p className="mt-2 text-xs text-amber-600">
              O código expira em <strong>15 minutos</strong> e aceita no máximo <strong>5 tentativas</strong>.
            </p>
          </div>
        </div>
      </div>

      {!result ? (
        /* Formulário de geração */
        <form onSubmit={handleGenerate} className="space-y-6">
          {/* Tipo de conta */}
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-3">
              Tipo de conta
            </label>
            <div className="grid grid-cols-3 gap-3">
              {userTypeOptions.map((opt) => {
                const Icon = opt.icon;
                return (
                  <button
                    key={opt.value}
                    type="button"
                    onClick={() => setUserType(opt.value)}
                    className={`flex flex-col items-center gap-2 p-4 rounded-xl border-2 transition-all ${
                      userType === opt.value
                        ? "border-red-500 bg-red-50 shadow-sm"
                        : "border-gray-200 bg-white hover:border-gray-300"
                    }`}
                  >
                    <Icon
                      className={`h-6 w-6 ${
                        userType === opt.value ? opt.color : "text-gray-400"
                      }`}
                    />
                    <span
                      className={`text-sm font-medium ${
                        userType === opt.value
                          ? "text-gray-900"
                          : "text-gray-600"
                      }`}
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
              placeholder={
                userType === "user"
                  ? "Email ou telefone do restaurante"
                  : "Telefone do usuário"
              }
              className="w-full px-4 py-3 bg-gray-50 border border-gray-200 rounded-xl text-gray-900 placeholder-gray-400 focus:bg-white focus:ring-2 focus:ring-red-500 focus:border-transparent transition-all"
              autoFocus
            />
          </div>

          {/* Botão */}
          <button
            type="submit"
            disabled={loading || !identifier.trim()}
            className="w-full py-3 px-4 rounded-xl font-semibold text-white transition-all disabled:opacity-50 disabled:cursor-not-allowed"
            style={{
              background: loading
                ? "#9CA3AF"
                : "linear-gradient(135deg, #DC2626, #B91C1C)",
            }}
          >
            {loading ? (
              <span className="flex items-center justify-center gap-2">
                <svg
                  className="animate-spin h-5 w-5"
                  viewBox="0 0 24 24"
                  fill="none"
                >
                  <circle
                    className="opacity-25"
                    cx="12"
                    cy="12"
                    r="10"
                    stroke="currentColor"
                    strokeWidth="4"
                  />
                  <path
                    className="opacity-75"
                    fill="currentColor"
                    d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
                  />
                </svg>
                Gerando...
              </span>
            ) : (
              <span className="flex items-center justify-center gap-2">
                <FiKey className="h-5 w-5" />
                Gerar Código
              </span>
            )}
          </button>
        </form>
      ) : (
        /* Resultado */
        <div className="space-y-6">
          <div className="bg-white border-2 border-green-200 rounded-xl p-6 text-center">
            <div className="w-14 h-14 rounded-full bg-green-100 flex items-center justify-center mx-auto mb-4">
              <FiCheck className="h-7 w-7 text-green-600" />
            </div>
            <h2 className="text-lg font-semibold text-gray-900 mb-1">
              Código Gerado
            </h2>
            <p className="text-sm text-gray-500 mb-4">
              Informe este código ao usuário por telefone ou WhatsApp
            </p>

            {/* Código */}
            <div className="bg-gray-900 rounded-xl px-6 py-4 mb-4">
              <p className="text-3xl font-mono font-bold text-white tracking-[0.3em]">
                {result.code}
              </p>
            </div>

            <button
              onClick={handleCopyCode}
              className="inline-flex items-center gap-2 px-4 py-2 bg-gray-100 hover:bg-gray-200 rounded-lg text-sm font-medium text-gray-700 transition-colors"
            >
              {copied ? (
                <>
                  <FiCheck className="h-4 w-4 text-green-600" /> Copiado!
                </>
              ) : (
                <>
                  <FiCopy className="h-4 w-4" /> Copiar código
                </>
              )}
            </button>
          </div>

          {/* Info */}
          <div className="bg-gray-50 rounded-xl p-4 text-sm text-gray-600 space-y-1">
            <p>
              <strong>Tipo:</strong>{" "}
              {userTypeOptions.find((o) => o.value === userType)?.label}
            </p>
            <p>
              <strong>Identificador:</strong> {result.identifier || identifier}
            </p>
            <p>
              <strong>Expira em:</strong>{" "}
              {new Date(result.expires_at).toLocaleString("pt-BR")}
            </p>
            <p>
              <strong>Tentativas permitidas:</strong> {result.attempts_allowed}
            </p>
          </div>

          {/* Botão novo código */}
          <button
            onClick={handleNewCode}
            className="w-full py-3 px-4 rounded-xl font-semibold border-2 border-gray-200 text-gray-700 hover:bg-gray-50 transition-all"
          >
            Gerar Novo Código
          </button>
        </div>
      )}
    </div>
  );
}
