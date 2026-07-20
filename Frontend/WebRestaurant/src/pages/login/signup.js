import React, { useState } from "react";
import { useAuth } from "../../context/AuthContext";
import Logo from "../../components/Logo";
import {
  FiMail,
  FiLock,
  FiUser,
  FiArrowRight,
  FiLoader,
  FiArrowLeft,
  FiMapPin,
} from "react-icons/fi";
import api from "../../services/api";

const SignupPage = ({ onBack }) => {
  const { login } = useAuth();
  const [form, setForm] = useState({
    name: "",
    email: "",
    password: "",
    confirmPassword: "",
    establishmentName: "",
  });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const handleChange = (e) => {
    setForm({ ...form, [e.target.name]: e.target.value });
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    setError("");

    if (!form.name || !form.email || !form.password || !form.establishmentName) {
      setError("Preencha todos os campos obrigatórios.");
      return;
    }

    if (form.password !== form.confirmPassword) {
      setError("As senhas não coincidem.");
      return;
    }

    if (form.password.length < 6) {
      setError("A senha deve ter pelo menos 6 caracteres.");
      return;
    }

    setLoading(true);
    try {
      const response = await api.post("/users/register", {
        name: form.name,
        email: form.email,
        password: form.password,
        establishment: {
          name: form.establishmentName,
          description: "",
        },
      });

      const token = response.data.token;
      if (token) {
        localStorage.setItem("JWT_TOKEN", token);
        window.location.href = "/#/";
      }
    } catch (err) {
      const msg =
        err.response?.data?.error ||
        "Erro ao criar conta. Tente novamente.";
      setError(msg);
    }
    setLoading(false);
  };

  return (
    <div className="min-h-screen flex">
      {/* Left Panel - Branding */}
      <div
        className="hidden lg:flex lg:w-1/2 relative overflow-hidden items-center justify-center"
        style={{
          background:
            "linear-gradient(135deg, #EA1D2C 0%, #C41420 40%, #8B0F18 100%)",
        }}
      >
        <div className="absolute inset-0 opacity-10">
          <div
            className="absolute -top-20 -left-20 w-96 h-96 rounded-full"
            style={{
              background: "radial-gradient(circle, #F7A11E 0%, transparent 70%)",
            }}
          />
          <div
            className="absolute -bottom-32 -right-32 w-[500px] h-[500px] rounded-full"
            style={{
              background: "radial-gradient(circle, #FF6B35 0%, transparent 70%)",
            }}
          />
        </div>
        <div className="relative z-10 text-center px-12">
          <Logo size={70} variant="white" />
          <div className="mt-10 text-white">
            <h2
              className="text-3xl font-bold mb-4"
              style={{ lineHeight: 1.2 }}
            >
              Comece a gerenciar
              <br />
              <span style={{ color: "#F7A11E" }}>seu restaurante</span>
            </h2>
            <p className="text-white/70 text-lg max-w-md mx-auto">
              Crie sua conta e comece a receber pedidos online hoje mesmo
            </p>
          </div>
        </div>
      </div>

      {/* Right Panel - Signup Form */}
      <div className="w-full lg:w-1/2 flex items-center justify-center px-4 sm:px-6 py-8 sm:py-12 bg-white">
        <div className="w-full max-w-md animate-fade-in">
          {/* Mobile Logo */}
          <div className="lg:hidden mb-6 flex justify-center">
            <Logo size={50} variant="login" />
          </div>

          <div className="mb-6">
            <button
              onClick={onBack}
              className="flex items-center gap-2 text-sm text-gray-500 hover:text-gray-700 mb-4 transition-colors"
            >
              <FiArrowLeft className="h-4 w-4" />
              Voltar para o login
            </button>
            <h2 className="text-2xl font-bold text-gray-900">
              Criar sua conta
            </h2>
            <p className="text-gray-500 mt-2 text-sm">
              Preencha os dados abaixo para começar
            </p>
          </div>

          {error && (
            <div className="mb-4 p-3 rounded-lg bg-red-50 border border-red-200 text-red-700 text-sm animate-slide-up">
              {error}
            </div>
          )}

          <form className="space-y-4" onSubmit={handleSubmit}>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1.5">
                Seu nome *
              </label>
              <div className="relative">
                <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                  <FiUser className="h-5 w-5 text-gray-400" />
                </div>
                <input
                  name="name"
                  type="text"
                  required
                  className="appearance-none block w-full pl-10 pr-4 py-3 border border-gray-200 rounded-xl placeholder-gray-400 text-gray-900 bg-gray-50 focus:bg-white transition-colors"
                  placeholder="João Silva"
                  value={form.name}
                  onChange={handleChange}
                />
              </div>
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1.5">
                E-mail *
              </label>
              <div className="relative">
                <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                  <FiMail className="h-5 w-5 text-gray-400" />
                </div>
                <input
                  name="email"
                  type="email"
                  required
                  className="appearance-none block w-full pl-10 pr-4 py-3 border border-gray-200 rounded-xl placeholder-gray-400 text-gray-900 bg-gray-50 focus:bg-white transition-colors"
                  placeholder="seu@email.com"
                  value={form.email}
                  onChange={handleChange}
                />
              </div>
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1.5">
                Nome do restaurante *
              </label>
              <div className="relative">
                <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                  <FiMapPin className="h-5 w-5 text-gray-400" />
                </div>
                <input
                  name="establishmentName"
                  type="text"
                  required
                  className="appearance-none block w-full pl-10 pr-4 py-3 border border-gray-200 rounded-xl placeholder-gray-400 text-gray-900 bg-gray-50 focus:bg-white transition-colors"
                  placeholder="Restaurante Exemplo"
                  value={form.establishmentName}
                  onChange={handleChange}
                />
              </div>
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1.5">
                Senha *
              </label>
              <div className="relative">
                <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                  <FiLock className="h-5 w-5 text-gray-400" />
                </div>
                <input
                  name="password"
                  type="password"
                  required
                  className="appearance-none block w-full pl-10 pr-4 py-3 border border-gray-200 rounded-xl placeholder-gray-400 text-gray-900 bg-gray-50 focus:bg-white transition-colors"
                  placeholder="Mínimo 6 caracteres"
                  value={form.password}
                  onChange={handleChange}
                />
              </div>
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1.5">
                Confirmar senha *
              </label>
              <div className="relative">
                <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                  <FiLock className="h-5 w-5 text-gray-400" />
                </div>
                <input
                  name="confirmPassword"
                  type="password"
                  required
                  className="appearance-none block w-full pl-10 pr-4 py-3 border border-gray-200 rounded-xl placeholder-gray-400 text-gray-900 bg-gray-50 focus:bg-white transition-colors"
                  placeholder="Repita a senha"
                  value={form.confirmPassword}
                  onChange={handleChange}
                />
              </div>
            </div>

            <button
              type="submit"
              disabled={loading}
              className="w-full flex items-center justify-center gap-2 py-3 px-4 rounded-xl text-white font-semibold text-sm transition-all duration-200 hover:shadow-lg hover:scale-[1.02] active:scale-[0.98] disabled:opacity-50 disabled:cursor-not-allowed disabled:hover:scale-100 mt-2"
              style={{
                background: "linear-gradient(135deg, #EA1D2C, #C41420)",
              }}
            >
              {loading ? (
                <>
                  <FiLoader className="animate-spin h-5 w-5" />
                  Criando conta...
                </>
              ) : (
                <>
                  Criar conta
                  <FiArrowRight className="h-5 w-5" />
                </>
              )}
            </button>
          </form>

          <div className="mt-6 text-center">
            <p className="text-sm text-gray-500">
              Já tem uma conta?{" "}
              <button
                onClick={onBack}
                className="font-semibold hover:underline"
                style={{ color: "#EA1D2C" }}
              >
                Faça login
              </button>
            </p>
          </div>
        </div>
      </div>
    </div>
  );
};

export default SignupPage;
