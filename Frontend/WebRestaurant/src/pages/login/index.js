import React, { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "../../context/AuthContext";
import Texts from "../../constants/Texts";
import SignupPage from "./signup";
import Logo from "../../components/Logo";
import { FiMail, FiLock, FiArrowRight, FiLoader } from "react-icons/fi";

const LoginPage = () => {
  const navigate = useNavigate();
  const { login } = useAuth();
  const [email, setEmail] = useState("");
  const [cadastro, setCadastro] = useState(false);
  const [password, setPassword] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const handleSubmit = async (e) => {
    e.preventDefault();
    setLoading(true);
    setError("");
    try {
      await login(email, password);
      navigate("/");
    } catch (error) {
      setError("Credenciais inválidas. Verifique seu e-mail e senha.");
      console.error("Erro de login:", error);
    }
    setLoading(false);
  };

  if (cadastro) return <SignupPage onBack={() => setCadastro(false)} />;

  return (
    <div className="min-h-screen flex">
      {/* Left Panel - Branding */}
      <div
        className="hidden lg:flex lg:w-1/2 relative overflow-hidden items-center justify-center"
        style={{
          background: "linear-gradient(135deg, #EA1D2C 0%, #C41420 40%, #8B0F18 100%)",
        }}
      >
        <div className="absolute inset-0 opacity-10">
          <div
            className="absolute -top-20 -left-20 w-96 h-96 rounded-full"
            style={{ background: "radial-gradient(circle, #F7A11E 0%, transparent 70%)" }}
          />
          <div
            className="absolute -bottom-32 -right-32 w-[500px] h-[500px] rounded-full"
            style={{ background: "radial-gradient(circle, #FF6B35 0%, transparent 70%)" }}
          />
        </div>
        <div className="relative z-10 text-center px-12">
          <Logo size={70} variant="white" />
          <div className="mt-10 text-white">
            <h2
              className="text-3xl font-bold mb-4"
              style={{ lineHeight: 1.2 }}
            >
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
                <div className="text-2xl font-bold" style={{ color: "#F7A11E" }}>
                  {stat.num}
                </div>
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
            <Logo size={50} variant="login" />
          </div>

          <div className="lg:hidden mb-8">
            <Logo size={40} variant="mark" />
          </div>

          <div className="mb-8">
            <h2 className="text-2xl font-bold text-gray-900">
              {Texts.text_login}
            </h2>
            <p className="text-gray-500 mt-2 text-sm">
              Entre com suas credenciais para acessar o painel
            </p>
          </div>

          {error && (
            <div className="mb-4 p-3 rounded-lg bg-red-50 border border-red-200 text-red-700 text-sm animate-slide-up">
              {error}
            </div>
          )}

          <form className="space-y-5" onSubmit={handleSubmit}>
            <div>
              <label
                htmlFor="email-address"
                className="block text-sm font-medium text-gray-700 mb-1.5"
              >
                {Texts.email_end}
              </label>
              <div className="relative">
                <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                  <FiMail className="h-5 w-5 text-gray-400" />
                </div>
                <input
                  id="email-address"
                  name="email"
                  type="email"
                  autoComplete="email"
                  required
                  className="appearance-none block w-full pl-10 pr-4 py-3 border border-gray-200 rounded-xl placeholder-gray-400 text-gray-900 bg-gray-50 focus:bg-white transition-colors"
                  placeholder="seu@email.com"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                />
              </div>
            </div>

            <div>
              <label
                htmlFor="password"
                className="block text-sm font-medium text-gray-700 mb-1.5"
              >
                {Texts.password}
              </label>
              <div className="relative">
                <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                  <FiLock className="h-5 w-5 text-gray-400" />
                </div>
                <input
                  id="password"
                  name="password"
                  type="password"
                  autoComplete="current-password"
                  required
                  className="appearance-none block w-full pl-10 pr-4 py-3 border border-gray-200 rounded-xl placeholder-gray-400 text-gray-900 bg-gray-50 focus:bg-white transition-colors"
                  placeholder="Sua senha"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                />
              </div>
            </div>

            <div className="flex items-center justify-between">
              <label className="flex items-center gap-2 cursor-pointer">
                <input
                  type="checkbox"
                  className="w-4 h-4 rounded border-gray-300 text-fuu-red focus:ring-fuu-red"
                />
                <span className="text-sm text-gray-600">Lembrar-me</span>
              </label>
              <a
                href="#"
                className="text-sm font-medium hover:underline"
                style={{ color: "#EA1D2C" }}
              >
                {Texts.esqueceu_senha}
              </a>
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
                  Entrando...
                </>
              ) : (
                <>
                  {Texts.entrar}
                  <FiArrowRight className="h-5 w-5" />
                </>
              )}
            </button>
          </form>

          <div className="mt-8 text-center">
            <p className="text-sm text-gray-500">
              Não tem uma conta?{" "}
              <button
                onClick={() => setCadastro(true)}
                className="font-semibold hover:underline"
                style={{ color: "#EA1D2C" }}
              >
                Cadastre-se
              </button>
            </p>
          </div>
        </div>
      </div>
    </div>
  );
};

export default LoginPage;
