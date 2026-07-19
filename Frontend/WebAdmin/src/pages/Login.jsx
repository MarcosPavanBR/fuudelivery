import React, { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "../context/AuthContext";
import { FiMail, FiLock, FiArrowRight, FiLoader, FiEye, FiEyeOff } from "react-icons/fi";

export default function Login() {
  const navigate = useNavigate();
  const { login } = useAuth();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const handleSubmit = async (e) => {
    e.preventDefault();
    setLoading(true);
    setError("");
    try {
      await login(email, password);
      navigate("/");
    } catch (err) {
      setError("Credenciais inválidas. Verifique seu e-mail e senha.");
      console.error("Login error:", err);
    }
    setLoading(false);
  };

  return (
    <div className="min-h-screen flex">
      {/* Left Panel - Branding */}
      <div className="hidden lg:flex lg:w-1/2 relative overflow-hidden items-center justify-center"
        style={{ background: "linear-gradient(135deg, #EA1D2C 0%, #C41420 40%, #8B0F18 100%)" }}>
        <div className="absolute inset-0 opacity-10">
          <div className="absolute -top-20 -left-20 w-96 h-96 rounded-full" style={{ background: "radial-gradient(circle, #F7A11E 0%, transparent 70%)" }} />
          <div className="absolute -bottom-32 -right-32 w-[500px] h-[500px] rounded-full" style={{ background: "radial-gradient(circle, #FF6B35 0%, transparent 70%)" }} />
        </div>
        <div className="relative z-10 text-center px-12">
          <svg width={80} height={80} viewBox="0 0 48 48" fill="none" xmlns="http://www.w3.org/2000/svg">
            <defs>
              <linearGradient id="loginGrad" x1="0%" y1="0%" x2="100%" y2="100%">
                <stop offset="0%" stopColor="#EA1D2C" />
                <stop offset="50%" stopColor="#FF4444" />
                <stop offset="100%" stopColor="#F7A11E" />
              </linearGradient>
              <filter id="loginShadow" x="-20%" y="-20%" width="140%" height="140%">
                <feDropShadow dx="0" dy="6" stdDeviation="10" floodColor="#EA1D2C" floodOpacity="0.35" />
              </filter>
            </defs>
            <rect width="48" height="48" rx="14" fill="url(#loginGrad)" filter="url(#loginShadow)" />
            <path d="M14 14h12c4.4 0 8 3.6 8 8s-3.6 8-8 8h-4v8h-8V14zm8 12h4c2.2 0 4-1.8 4-4s-1.8-4-4-4h-4v8z" fill="white" />
            <circle cx="38" cy="12" r="4" fill="#F7A11E" />
            <path d="M35 10l3 2 3-2" stroke="white" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" opacity="0.9" />
          </svg>
          <div className="mt-10 text-white">
            <h2 className="text-3xl font-bold mb-4" style={{ lineHeight: 1.2 }}>
              Gerencie seu restaurante
              <br />
              <span style={{ color: "#F7A11E" }}>com inteligência</span>
            </h2>
            <p className="text-white/70 text-lg max-w-md mx-auto">
              Acesse pedidos em tempo real, gerencie seu cardápio e acompanhe suas vendas
            </p>
          </div>
          <div className="mt-12 flex gap-6 justify-center">
            {[
              { num: "100+", label: "Restaurantes" },
              { num: "50k+", label: "Pedidos/mês" },
              { num: "4.9", label: "Avaliação" },
            ].map((stat) => (
              <div key={stat.label} className="text-center">
                <div className="text-2xl font-bold" style={{ color: "#F7A11E" }}>{stat.num}</div>
                <div className="text-white/60 text-sm mt-1">{stat.label}</div>
              </div>
            ))}
          </div>
        </div>
      </div>

      {/* Right Panel - Login Form */}
      <div className="w-full lg:w-1/2 flex items-center justify-center px-6 py-12 bg-white">
        <div className="w-full max-w-md animate-fade-in">
          {/* Mobile Logo */}
          <div className="lg:hidden mb-8 flex justify-center">
            <svg width={60} height={60} viewBox="0 0 48 48" fill="none" xmlns="http://www.w3.org/2000/svg">
              <defs>
                <linearGradient id="loginGrad2" x1="0%" y1="0%" x2="100%" y2="100%">
                  <stop offset="0%" stopColor="#EA1D2C" />
                  <stop offset="50%" stopColor="#FF4444" />
                  <stop offset="100%" stopColor="#F7A11E" />
                </linearGradient>
                <filter id="loginShadow2" x="-20%" y="-20%" width="140%" height="140%">
                  <feDropShadow dx="0" dy="6" stdDeviation="10" floodColor="#EA1D2C" floodOpacity="0.35" />
                </filter>
              </defs>
              <rect width="48" height="48" rx="14" fill="url(#loginGrad2)" filter="url(#loginShadow2)" />
              <path d="M14 14h12c4.4 0 8 3.6 8 8s-3.6 8-8 8h-4v8h-8V14zm8 12h4c2.2 0 4-1.8 4-4s-1.8-4-4-4h-4v8z" fill="white" />
              <circle cx="38" cy="12" r="4" fill="#F7A11E" />
              <path d="M35 10l3 2 3-2" stroke="white" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" opacity="0.9" />
            </svg>
          </div>

          <div className="mb-8">
            <h2 className="text-2xl font-bold text-gray-900">Entrar na conta</h2>
            <p className="text-gray-500 mt-2 text-sm">Acesse o painel administrativo do FuuDelivery</p>
          </div>

          {error && (
            <div className="mb-4 p-3 rounded-lg bg-red-50 border border-red-200 text-red-700 text-sm animate-slide-up">
              {error}
            </div>
          )}

          <form onSubmit={handleSubmit} className="space-y-5">
            <div>
              <label htmlFor="email" className="block text-sm font-medium text-gray-700 mb-1.5">E-mail</label>
              <div className="relative">
                <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                  <FiMail className="h-5 w-5 text-gray-400" />
                </div>
                <input
                  id="email"
                  name="email"
                  type="email"
                  autoComplete="email"
                  required
                  className="block w-full pl-10 pr-4 py-3 border border-gray-200 rounded-xl text-sm bg-gray-50 placeholder-gray-400 text-gray-900 focus:bg-white transition-colors"
                  placeholder="seu@email.com"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                />
              </div>
            </div>

            <div>
              <label htmlFor="password" className="block text-sm font-medium text-gray-700 mb-1.5">Senha</label>
              <div className="relative">
                <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                  <FiLock className="h-5 w-5 text-gray-400" />
                </div>
                <input
                  id="password"
                  name="password"
                  type={showPassword ? "text" : "password"}
                  autoComplete="current-password"
                  required
                  className="block w-full pl-10 pr-12 py-3 border border-gray-200 rounded-xl text-sm bg-gray-50 placeholder-gray-400 text-gray-900 focus:bg-white transition-colors"
                  placeholder="Sua senha"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                />
                <button
                  type="button"
                  onClick={() => setShowPassword(!showPassword)}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600"
                >
                  {showPassword ? <FiEyeOff className="h-5 w-5" /> : <FiEye className="h-5 w-5" />}
                </button>
              </div>
            </div>

            <div className="flex items-center justify-between">
              <label className="flex items-center gap-2 cursor-pointer">
                <input type="checkbox" className="w-4 h-4 rounded border-gray-300 text-fuu-red focus:ring-fuu-red" />
                <span className="text-sm text-gray-600">Lembrar-me</span>
              </label>
              <a href="#" className="text-sm font-medium hover:underline" style={{ color: "#EA1D2C" }}>
                Esqueceu a senha?
              </a>
            </div>

            <button
              type="submit"
              disabled={loading}
              className="w-full flex items-center justify-center gap-2 py-3 px-4 rounded-xl text-white font-semibold text-sm transition-all duration-200 hover:shadow-lg hover:scale-[1.02] active:scale-[0.98] disabled:opacity-50 disabled:cursor-not-allowed disabled:hover:scale-100"
              style={{ background: "linear-gradient(135deg, #EA1D2C, #C41420)" }}
            >
              {loading ? (
                <>
                  <FiLoader className="animate-spin h-5 w-5" />
                  Entrando...
                </>
              ) : (
                <>
                  Entrar
                  <FiArrowRight className="h-5 w-5" />
                </>
              )}
            </button>
          </form>

          <div className="mt-8 text-center">
            <p className="text-sm text-gray-500">
              Não tem uma conta?{" "}
              <button className="font-semibold hover:underline" style={{ color: "#EA1D2C" }}>
                Cadastre-se
              </button>
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}