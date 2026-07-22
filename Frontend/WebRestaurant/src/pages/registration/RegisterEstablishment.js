import React, { useState } from "react";
import { useNavigate } from "react-router-dom";
import { toast } from "react-toastify";
import {
  FiArrowLeft,
  FiUser,
  FiMail,
  FiLock,
  FiMapPin,
  FiPhone,
  FiClock,
  FiSave,
  FiLoader,
  FiStore,
} from "react-icons/fi";
import api from "../../services/api";

const RegisterEstablishment = () => {
  const navigate = useNavigate();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [form, setForm] = useState({
    name: "",
    owner_name: "",
    email: "",
    password: "",
    confirm_password: "",
    phone: "",
    address: "",
    city: "",
    opening_time: "09:00",
    closing_time: "22:00",
  });

  const handleChange = (e) => {
    setForm({ ...form, [e.target.name]: e.target.value });
  };

  const validate = () => {
    if (!form.name.trim()) return "Nome do restaurante é obrigatório";
    if (!form.owner_name.trim()) return "Nome do responsável é obrigatório";
    if (!form.email.trim()) return "Email é obrigatório";
    if (!form.email.includes("@")) return "Email inválido";
    if (form.password.length < 6) return "Senha deve ter pelo menos 6 caracteres";
    if (form.password !== form.confirm_password) return "Senhas não conferem";
    if (!form.phone.trim()) return "Telefone é obrigatório";
    if (!form.address.trim()) return "Endereço é obrigatório";
    if (!form.city.trim()) return "Cidade é obrigatória";
    return null;
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    setError("");

    const validationError = validate();
    if (validationError) {
      setError(validationError);
      return;
    }

    setLoading(true);
    try {
      await api.post("/establishments/register", {
        name: form.name,
        owner_name: form.owner_name,
        email: form.email,
        password: form.password,
        phone: form.phone,
        address: form.address,
        city: form.city,
        opening_time: form.opening_time,
        closing_time: form.closing_time,
      });

      toast.success("Restaurante cadastrado com sucesso!");
      navigate("/");
    } catch (err) {
      const msg =
        err.response?.data?.error || "Erro ao cadastrar restaurante. Tente novamente.";
      setError(msg);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-gray-50">
      {/* Header */}
      <div className="bg-white border-b border-gray-100 sticky top-0 z-10">
        <div className="max-w-2xl mx-auto px-4 py-4 flex items-center gap-3">
          <button
            onClick={() => navigate(-1)}
            className="p-2 rounded-xl hover:bg-gray-100 transition-colors"
          >
            <FiArrowLeft className="h-5 w-5 text-gray-600" />
          </button>
          <div>
            <h1 className="text-lg font-bold text-gray-900">Cadastrar Restaurante</h1>
            <p className="text-xs text-gray-500">Preencha os dados para começar</p>
          </div>
        </div>
      </div>

      {/* Form */}
      <div className="max-w-2xl mx-auto px-4 py-6">
        <form onSubmit={handleSubmit} className="space-y-6">
          {/* Error banner */}
          {error && (
            <div className="p-3 rounded-lg bg-red-50 border border-red-200 text-red-700 text-sm animate-slide-up">
              {error}
            </div>
          )}

          {/* Restaurante */}
          <div className="bg-white rounded-2xl border border-gray-100 shadow-card p-6">
            <div className="flex items-center gap-2 mb-4">
              <div className="p-2.5 rounded-xl bg-red-50">
                <FiStore className="h-5 w-5" style={{ color: "#EA1D2C" }} />
              </div>
              <h2 className="text-sm font-semibold text-gray-900">Dados do Restaurante</h2>
            </div>

            <div className="space-y-4">
              <div>
                <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">
                  Nome do Restaurante *
                </label>
                <div className="relative">
                  <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                    <FiStore className="h-5 w-5 text-gray-400" />
                  </div>
                  <input
                    name="name"
                    type="text"
                    required
                    className="appearance-none block w-full pl-10 pr-4 py-3 border border-gray-200 rounded-xl placeholder-gray-400 text-gray-900 bg-gray-50 focus:bg-white transition-colors"
                    placeholder="Ex: Restaurante Sabor da Terra"
                    value={form.name}
                    onChange={handleChange}
                  />
                </div>
              </div>

              <div>
                <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">
                  Nome do Responsável *
                </label>
                <div className="relative">
                  <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                    <FiUser className="h-5 w-5 text-gray-400" />
                  </div>
                  <input
                    name="owner_name"
                    type="text"
                    required
                    className="appearance-none block w-full pl-10 pr-4 py-3 border border-gray-200 rounded-xl placeholder-gray-400 text-gray-900 bg-gray-50 focus:bg-white transition-colors"
                    placeholder="Seu nome completo"
                    value={form.owner_name}
                    onChange={handleChange}
                  />
                </div>
              </div>
            </div>
          </div>

          {/* Conta */}
          <div className="bg-white rounded-2xl border border-gray-100 shadow-card p-6">
            <div className="flex items-center gap-2 mb-4">
              <div className="p-2.5 rounded-xl bg-red-50">
                <FiMail className="h-5 w-5" style={{ color: "#EA1D2C" }} />
              </div>
              <h2 className="text-sm font-semibold text-gray-900">Conta de Acesso</h2>
            </div>

            <div className="space-y-4">
              <div>
                <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">
                  Email *
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
                    placeholder="restaurante@email.com"
                    value={form.email}
                    onChange={handleChange}
                  />
                </div>
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div>
                  <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">
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
                  <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">
                    Confirmar Senha *
                  </label>
                  <div className="relative">
                    <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                      <FiLock className="h-5 w-5 text-gray-400" />
                    </div>
                    <input
                      name="confirm_password"
                      type="password"
                      required
                      className="appearance-none block w-full pl-10 pr-4 py-3 border border-gray-200 rounded-xl placeholder-gray-400 text-gray-900 bg-gray-50 focus:bg-white transition-colors"
                      placeholder="Repita a senha"
                      value={form.confirm_password}
                      onChange={handleChange}
                    />
                  </div>
                </div>
              </div>
            </div>
          </div>

          {/* Contato e Endereço */}
          <div className="bg-white rounded-2xl border border-gray-100 shadow-card p-6">
            <div className="flex items-center gap-2 mb-4">
              <div className="p-2.5 rounded-xl bg-red-50">
                <FiMapPin className="h-5 w-5" style={{ color: "#EA1D2C" }} />
              </div>
              <h2 className="text-sm font-semibold text-gray-900">Contato e Endereço</h2>
            </div>

            <div className="space-y-4">
              <div>
                <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">
                  Telefone *
                </label>
                <div className="relative">
                  <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                    <FiPhone className="h-5 w-5 text-gray-400" />
                  </div>
                  <input
                    name="phone"
                    type="tel"
                    required
                    className="appearance-none block w-full pl-10 pr-4 py-3 border border-gray-200 rounded-xl placeholder-gray-400 text-gray-900 bg-gray-50 focus:bg-white transition-colors"
                    placeholder="(11) 99999-9999"
                    value={form.phone}
                    onChange={handleChange}
                  />
                </div>
              </div>

              <div>
                <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">
                  Endereço *
                </label>
                <div className="relative">
                  <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                    <FiMapPin className="h-5 w-5 text-gray-400" />
                  </div>
                  <input
                    name="address"
                    type="text"
                    required
                    className="appearance-none block w-full pl-10 pr-4 py-3 border border-gray-200 rounded-xl placeholder-gray-400 text-gray-900 bg-gray-50 focus:bg-white transition-colors"
                    placeholder="Rua, número, bairro"
                    value={form.address}
                    onChange={handleChange}
                  />
                </div>
              </div>

              <div>
                <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">
                  Cidade *
                </label>
                <div className="relative">
                  <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                    <FiMapPin className="h-5 w-5 text-gray-400" />
                  </div>
                  <input
                    name="city"
                    type="text"
                    required
                    className="appearance-none block w-full pl-10 pr-4 py-3 border border-gray-200 rounded-xl placeholder-gray-400 text-gray-900 bg-gray-50 focus:bg-white transition-colors"
                    placeholder="São Paulo - SP"
                    value={form.city}
                    onChange={handleChange}
                  />
                </div>
              </div>
            </div>
          </div>

          {/* Horário de Funcionamento */}
          <div className="bg-white rounded-2xl border border-gray-100 shadow-card p-6">
            <div className="flex items-center gap-2 mb-4">
              <div className="p-2.5 rounded-xl bg-red-50">
                <FiClock className="h-5 w-5" style={{ color: "#EA1D2C" }} />
              </div>
              <h2 className="text-sm font-semibold text-gray-900">Horário de Funcionamento</h2>
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">
                  Abertura
                </label>
                <input
                  name="opening_time"
                  type="time"
                  className="appearance-none block w-full px-4 py-3 border border-gray-200 rounded-xl text-gray-900 bg-gray-50 focus:bg-white transition-colors"
                  value={form.opening_time}
                  onChange={handleChange}
                />
              </div>
              <div>
                <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">
                  Fechamento
                </label>
                <input
                  name="closing_time"
                  type="time"
                  className="appearance-none block w-full px-4 py-3 border border-gray-200 rounded-xl text-gray-900 bg-gray-50 focus:bg-white transition-colors"
                  value={form.closing_time}
                  onChange={handleChange}
                />
              </div>
            </div>
          </div>

          {/* Submit */}
          <button
            type="submit"
            disabled={loading}
            className="w-full flex items-center justify-center gap-2 py-3 px-4 rounded-xl text-white font-semibold text-sm transition-all duration-200 hover:shadow-lg hover:scale-[1.02] active:scale-[0.98] disabled:opacity-50 disabled:cursor-not-allowed"
            style={{ background: "linear-gradient(135deg, #EA1D2C, #C41420)" }}
          >
            {loading ? (
              <>
                <FiLoader className="animate-spin h-5 w-5" /> Processando...
              </>
            ) : (
              <>
                <FiSave className="h-5 w-5" /> Cadastrar Restaurante
              </>
            )}
          </button>
        </form>
      </div>
    </div>
  );
};

export default RegisterEstablishment;
