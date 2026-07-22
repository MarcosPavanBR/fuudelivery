import React, { useState, useEffect } from "react";
import { toast } from "react-toastify";
import api from "../../services/api";
import { useAuth } from "../../context/AuthContext";

export default function WalletPage() {
  const { getUser } = useAuth();
  const user = getUser();
  const [walletId, setWalletId] = useState("");
  const [loading, setLoading] = useState(false);
  const [creating, setCreating] = useState(false);
  const [formData, setFormData] = useState({
    name: "",
    cpf_cnpj: "",
    email: "",
    phone: "",
    person_type: "JURIDICA",
  });

  useEffect(() => {
    if (user) {
      setFormData((prev) => ({
        ...prev,
        name: user.establishment_name || "",
        email: user.email || "",
        phone: user.phone || "",
      }));
      if (user.payment_wallet_id) {
        setWalletId(user.payment_wallet_id);
      }
    }
  }, [user]);

  const handleChange = (e) => {
    setFormData({ ...formData, [e.target.name]: e.target.value });
  };

  const handleCreateWallet = async () => {
    if (!formData.name || !formData.cpf_cnpj || !formData.email) {
      toast.error("Preencha nome, CNPJ/CPF e email");
      return;
    }

    setCreating(true);
    try {
      const res = await api.post("/asaas/wallet/create", formData);
      const newWalletId = res.data.wallet_id;
      setWalletId(newWalletId);

      await api.put(
        `/establishments/${user.establishment_id || user.id}/wallet`,
        { payment_wallet_id: newWalletId }
      );

      toast.success("Conta de recebimento criada com sucesso!");
    } catch (err) {
      toast.error(err?.response?.data?.error || "Erro ao criar conta");
    } finally {
      setCreating(false);
    }
  };

  return (
    <div className="max-w-2xl mx-auto">
      <h1 className="text-2xl font-bold mb-6" style={{ color: "var(--text-primary)" }}>
        Conta de Recebimento
      </h1>

      <div
        className="rounded-xl p-6 mb-6"
        style={{
          background: "var(--bg-card, #FFFFFF)",
          border: "1px solid var(--border-color, #E5E7EB)",
        }}
      >
        <p className="text-sm mb-4" style={{ color: "var(--text-secondary)" }}>
          Crie sua conta de recebimento no Asaas para receber pagamentos de pedidos
          diretamente na sua conta bancária. O split de pagamento é feito automaticamente
          na origem — você recebe apenas a sua fatia.
        </p>

        {walletId ? (
          <div className="p-4 rounded-lg bg-green-50 border border-green-200">
            <div className="flex items-center gap-2 mb-2">
              <div className="w-3 h-3 rounded-full bg-green-500" />
              <span className="font-semibold text-green-800">Conta ativa</span>
            </div>
            <p className="text-sm text-green-700">
              Wallet ID: <code className="bg-green-100 px-1 rounded">{walletId}</code>
            </p>
            <p className="text-xs text-green-600 mt-2">
              Pagamentos de pedidos serão enviados diretamente para sua conta bancária.
            </p>
          </div>
        ) : (
          <>
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium mb-1" style={{ color: "var(--text-primary)" }}>
                  Razão Social / Nome
                </label>
                <input
                  type="text"
                  name="name"
                  value={formData.name}
                  onChange={handleChange}
                  className="w-full px-3 py-2 rounded-lg border"
                  style={{
                    background: "var(--bg-card)",
                    borderColor: "var(--border-color)",
                    color: "var(--text-primary)",
                  }}
                  placeholder="Razão social do restaurante"
                />
              </div>

              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm font-medium mb-1" style={{ color: "var(--text-primary)" }}>
                    Tipo
                  </label>
                  <select
                    name="person_type"
                    value={formData.person_type}
                    onChange={handleChange}
                    className="w-full px-3 py-2 rounded-lg border"
                    style={{
                      background: "var(--bg-card)",
                      borderColor: "var(--border-color)",
                      color: "var(--text-primary)",
                    }}
                  >
                    <option value="JURIDICA">PJ (CNPJ)</option>
                    <option value="FISICA">PF (CPF)</option>
                  </select>
                </div>
                <div>
                  <label className="block text-sm font-medium mb-1" style={{ color: "var(--text-primary)" }}>
                    {formData.person_type === "JURIDICA" ? "CNPJ" : "CPF"}
                  </label>
                  <input
                    type="text"
                    name="cpf_cnpj"
                    value={formData.cpf_cnpj}
                    onChange={handleChange}
                    className="w-full px-3 py-2 rounded-lg border"
                    style={{
                      background: "var(--bg-card)",
                      borderColor: "var(--border-color)",
                      color: "var(--text-primary)",
                    }}
                    placeholder={formData.person_type === "JURIDICA" ? "00.000.000/0001-00" : "000.000.000-00"}
                  />
                </div>
              </div>

              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm font-medium mb-1" style={{ color: "var(--text-primary)" }}>
                    Email
                  </label>
                  <input
                    type="email"
                    name="email"
                    value={formData.email}
                    onChange={handleChange}
                    className="w-full px-3 py-2 rounded-lg border"
                    style={{
                      background: "var(--bg-card)",
                      borderColor: "var(--border-color)",
                      color: "var(--text-primary)",
                    }}
                    placeholder="email@restaurante.com"
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium mb-1" style={{ color: "var(--text-primary)" }}>
                    Telefone
                  </label>
                  <input
                    type="text"
                    name="phone"
                    value={formData.phone}
                    onChange={handleChange}
                    className="w-full px-3 py-2 rounded-lg border"
                    style={{
                      background: "var(--bg-card)",
                      borderColor: "var(--border-color)",
                      color: "var(--text-primary)",
                    }}
                    placeholder="(11) 99999-9999"
                  />
                </div>
              </div>

              <button
                onClick={handleCreateWallet}
                disabled={creating}
                className="w-full py-3 rounded-lg font-semibold text-white transition-colors"
                style={{
                  background: creating ? "#9CA3AF" : "linear-gradient(135deg, #EA1D2C, #C41420)",
                }}
              >
                {creating ? "Criando conta..." : "Criar Conta de Recebimento"}
              </button>
            </div>

            <p className="text-xs mt-4" style={{ color: "var(--text-secondary)" }}>
              Ao criar a conta, você concorda com os termos de uso do Asaas.
              A conta é gratuita e sem mensalidade.
            </p>
          </>
        )}
      </div>

      <div
        className="rounded-xl p-6"
        style={{
          background: "var(--bg-card, #FFFFFF)",
          border: "1px solid var(--border-color, #E5E7EB)",
        }}
      >
        <h2 className="font-semibold mb-3" style={{ color: "var(--text-primary)" }}>
          Como funciona?
        </h2>
        <ul className="text-sm space-y-2" style={{ color: "var(--text-secondary)" }}>
          <li>1. Crie sua conta de recebimento (gratuita, sem mensalidade)</li>
          <li>2. Quando um cliente paga o pedido, o valor é dividido automaticamente</li>
          <li>3. Sua fatia (85%) vai direto para sua conta bancária</li>
          <li>4. A plataforma (5%) e o entregador recebem suas partes automaticamente</li>
          <li>5. Sem necessidade de PIX manual ou transferência manual</li>
        </ul>
      </div>
    </div>
  );
}
