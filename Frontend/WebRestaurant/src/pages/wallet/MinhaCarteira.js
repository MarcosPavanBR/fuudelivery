import React, { useState, useEffect, useCallback } from "react";
import { useAuth } from "../../context/AuthContext";
import {
  getWallet,
  getExtract,
  requestWithdraw,
  getPaymentHealth,
} from "../../services/payment.model";
import {
  FaWallet,
  FaArrowUp,
  FaArrowDown,
  FaLock,
  FaMoneyBillWave,
  FaHistory,
  FaSpinner,
  FaExclamationTriangle,
  FaCheckCircle,
  FaFileInvoiceDollar,
  FaUniversity,
  FaQrcode,
} from "react-icons/fa";
import { toast } from "react-toastify";

function formatCurrency(value) {
  if (value == null) return "R$ 0,00";
  return `R$ ${Number(value).toLocaleString("pt-BR", {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  })}`;
}

function formatDate(dateStr) {
  if (!dateStr) return "\u2014";
  return new Date(dateStr).toLocaleString("pt-BR");
}

function getTransactionIcon(type) {
  switch (type) {
    case "CREDIT":
    case "PAYMENT":
      return <FaArrowDown className="text-green-400" />;
    case "DEBIT":
    case "CHARGEBACK":
      return <FaArrowUp className="text-red-400" />;
    case "WITHDRAWAL":
      return <FaMoneyBillWave className="text-yellow-400" />;
    default:
      return <FaFileInvoiceDollar className="text-gray-400" />;
  }
}

export default function MinhaCarteira() {
  const { user } = useAuth();
  const [wallet, setWallet] = useState(null);
  const [transactions, setTransactions] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [showWithdraw, setShowWithdraw] = useState(false);
  const [withdrawAmount, setWithdrawAmount] = useState("");
  const [withdrawMethod, setWithdrawMethod] = useState("PIX");
  const [withdrawDest, setWithdrawDest] = useState("");
  const [withdrawing, setWithdrawing] = useState(false);
  const [paymentOnline, setPaymentOnline] = useState(null);
  const [cursor, setCursor] = useState("");
  const [hasMore, setHasMore] = useState(false);

  const establishmentId = user?.establishmentId || user?._id || user?.id || "";

  const fetchWallet = useCallback(async () => {
    if (!establishmentId) {
      setError("ID do estabelecimento não encontrado. Faça login novamente.");
      setLoading(false);
      return;
    }

    try {
      setLoading(true);
      setError(null);

      const [walletData, extractData, health] = await Promise.all([
        getWallet(establishmentId).catch(() => null),
        getExtract(establishmentId, 20, "").catch(() => ({ data: [] })),
        getPaymentHealth().catch(() => ({ status: "offline" })),
      ]);

      if (walletData) {
        setWallet(walletData);
      }

      setTransactions(extractData?.data || []);
      setCursor(extractData?.next_cursor || "");
      setHasMore(!!extractData?.next_cursor);
      setPaymentOnline(health?.status === "healthy");
    } catch (err) {
      console.error("Erro ao carregar carteira:", err);
      setError(
        err?.response?.data?.message ||
          err?.message ||
          "Erro ao conectar com o servidor de pagamentos"
      );
    } finally {
      setLoading(false);
    }
  }, [establishmentId]);

  useEffect(() => {
    fetchWallet();
  }, [fetchWallet]);

  const loadMore = async () => {
    if (!cursor) return;
    try {
      const more = await getExtract(establishmentId, 20, cursor);
      setTransactions((prev) => [...prev, ...(more?.data || [])]);
      setCursor(more?.next_cursor || "");
      setHasMore(!!more?.next_cursor);
    } catch (err) {
      toast.error("Erro ao carregar mais transações");
    }
  };

  const handleWithdraw = async () => {
    const amount = parseFloat(withdrawAmount);

    if (!amount || amount <= 0) {
      toast.error("Informe um valor válido");
      return;
    }

    if (wallet && amount > wallet.available) {
      toast.error("Saldo insuficiente para este saque");
      return;
    }

    if (amount < 10) {
      toast.error("Valor mínimo para saque: R$ 10,00");
      return;
    }

    if (!withdrawDest || withdrawDest.length < 10) {
      toast.error("Informe uma chave PIX ou dados bancários válidos");
      return;
    }

    try {
      setWithdrawing(true);
      await requestWithdraw(establishmentId, {
        amount,
        destination: withdrawDest,
        method: withdrawMethod,
      });
      toast.success(
        `Saque de R$ ${amount.toFixed(2)} solicitado com sucesso!`
      );
      setShowWithdraw(false);
      setWithdrawAmount("");
      setWithdrawDest("");
      await fetchWallet();
    } catch (err) {
      toast.error(
        err?.response?.data?.error || "Erro ao solicitar saque"
      );
    } finally {
      setWithdrawing(false);
    }
  };

  if (loading) {
    return (
      <div className="flex flex-col items-center justify-center min-h-[400px]">
        <FaSpinner className="animate-spin text-4xl text-red-500 mb-4" />
        <p className="text-gray-400">Carregando carteira...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center min-h-[400px]">
        <FaExclamationTriangle className="text-4xl text-yellow-500 mb-4" />
        <p className="text-gray-300 mb-2">Não foi possível carregar a carteira</p>
        <p className="text-gray-500 text-sm mb-4">{error}</p>
        <button
          onClick={fetchWallet}
          className="px-4 py-2 bg-red-600 hover:bg-red-700 rounded-lg text-white transition"
        >
          Tentar novamente
        </button>
      </div>
    );
  }

  return (
    <div className="max-w-4xl mx-auto p-4 space-y-6">
      {/* Status do servidor de pagamentos */}
      <div className="flex items-center gap-2 text-sm">
        <div
          className={`w-2 h-2 rounded-full ${
            paymentOnline ? "bg-green-400" : "bg-red-400"
          }`}
        />
        <span className="text-gray-400">
          Servidor de pagamentos:{" "}
          {paymentOnline ? "Online" : "Offline"}
        </span>
      </div>

      {/* Cards de saldo */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div className="bg-gray-800 rounded-xl p-5 border border-gray-700">
          <div className="flex items-center gap-2 mb-3">
            <FaWallet className="text-green-400" />
            <span className="text-gray-400 text-sm">Saldo Disponível</span>
          </div>
          <p className="text-3xl font-bold text-green-400">
            {formatCurrency(wallet?.available)}
          </p>
          <p className="text-gray-500 text-xs mt-1">Pronto para saque</p>
        </div>

        <div className="bg-gray-800 rounded-xl p-5 border border-gray-700">
          <div className="flex items-center gap-2 mb-3">
            <FaSpinner className="text-yellow-400" />
            <span className="text-gray-400 text-sm">Saldo Pendente</span>
          </div>
          <p className="text-3xl font-bold text-yellow-400">
            {formatCurrency(wallet?.pending)}
          </p>
          <p className="text-gray-500 text-xs mt-1">
            Aguardando aprovação do sistema
          </p>
        </div>

        <div className="bg-gray-800 rounded-xl p-5 border border-gray-700">
          <div className="flex items-center gap-2 mb-3">
            <FaLock className="text-red-400" />
            <span className="text-gray-400 text-sm">Saldo Bloqueado</span>
          </div>
          <p className="text-3xl font-bold text-red-400">
            {formatCurrency(wallet?.blocked)}
          </p>
          <p className="text-gray-500 text-xs mt-1">
            Retido por disputa ou estorno
          </p>
        </div>
      </div>

      {/* Totais */}
      <div className="grid grid-cols-2 gap-4">
        <div className="bg-gray-800/50 rounded-xl p-4 border border-gray-700/50">
          <span className="text-gray-400 text-sm">Total ganho</span>
          <p className="text-xl font-semibold text-white">
            {formatCurrency(wallet?.total_earned)}
          </p>
        </div>
        <div className="bg-gray-800/50 rounded-xl p-4 border border-gray-700/50">
          <span className="text-gray-400 text-sm">Total sacado</span>
          <p className="text-xl font-semibold text-white">
            {formatCurrency(wallet?.total_withdrawn)}
          </p>
        </div>
      </div>

      {/* Botão de saque */}
      {wallet?.available > 0 && (
        <button
          onClick={() => setShowWithdraw(true)}
          className="w-full py-3 bg-green-600 hover:bg-green-700 rounded-xl text-white font-semibold transition flex items-center justify-center gap-2"
        >
          <FaMoneyBillWave /> Solicitar Saque
        </button>
      )}

      {/* Modal de saque */}
      {showWithdraw && (
        <div
          className="fixed inset-0 bg-black/70 flex items-center justify-center z-50 p-4"
          onClick={(e) => e.target === e.currentTarget && setShowWithdraw(false)}
        >
          <div className="bg-gray-800 rounded-2xl p-6 w-full max-w-md border border-gray-700">
            <h3 className="text-xl font-bold text-white mb-4">
              Solicitar Saque
            </h3>

            <div className="space-y-4">
              <div>
                <label className="text-gray-400 text-sm block mb-1">
                  Valor (mínimo R$ 10,00)
                </label>
                <input
                  type="number"
                  value={withdrawAmount}
                  onChange={(e) => setWithdrawAmount(e.target.value)}
                  placeholder="0.00"
                  min="10"
                  step="0.01"
                  className="w-full px-4 py-3 bg-gray-700 rounded-lg text-white border border-gray-600 focus:border-green-500 focus:outline-none"
                />
                <p className="text-gray-500 text-xs mt-1">
                  Disponível: {formatCurrency(wallet?.available)}
                </p>
              </div>

              <div>
                <label className="text-gray-400 text-sm block mb-1">
                  Método
                </label>
                <div className="flex gap-2">
                  <button
                    onClick={() => setWithdrawMethod("PIX")}
                    className={`flex-1 py-2 rounded-lg flex items-center justify-center gap-2 transition ${
                      withdrawMethod === "PIX"
                        ? "bg-green-600 text-white"
                        : "bg-gray-700 text-gray-400"
                    }`}
                  >
                    <FaQrcode /> PIX
                  </button>
                  <button
                    onClick={() => setWithdrawMethod("TED")}
                    className={`flex-1 py-2 rounded-lg flex items-center justify-center gap-2 transition ${
                      withdrawMethod === "TED"
                        ? "bg-green-600 text-white"
                        : "bg-gray-700 text-gray-400"
                    }`}
                  >
                    <FaUniversity /> TED
                  </button>
                </div>
              </div>

              <div>
                <label className="text-gray-400 text-sm block mb-1">
                  {withdrawMethod === "PIX"
                    ? "Chave PIX"
                    : "Dados bancários (Ag/CC)"}
                </label>
                <input
                  type="text"
                  value={withdrawDest}
                  onChange={(e) => setWithdrawDest(e.target.value)}
                  placeholder={
                    withdrawMethod === "PIX"
                      ? "CPF, email, telefone ou chave aleatória"
                      : "0000/00000-0"
                  }
                  className="w-full px-4 py-3 bg-gray-700 rounded-lg text-white border border-gray-600 focus:border-green-500 focus:outline-none"
                />
              </div>
            </div>

            <div className="flex gap-3 mt-6">
              <button
                onClick={() => setShowWithdraw(false)}
                className="flex-1 py-2 bg-gray-700 hover:bg-gray-600 rounded-lg text-gray-300 transition"
              >
                Cancelar
              </button>
              <button
                onClick={handleWithdraw}
                disabled={withdrawing}
                className="flex-1 py-2 bg-green-600 hover:bg-green-700 rounded-lg text-white font-semibold transition flex items-center justify-center gap-2"
              >
                {withdrawing ? (
                  <>
                    <FaSpinner className="animate-spin" /> Processando...
                  </>
                ) : (
                  <>
                    <FaCheckCircle /> Confirmar Saque
                  </>
                )}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Extrato */}
      <div className="bg-gray-800 rounded-xl border border-gray-700">
        <div className="flex items-center justify-between p-4 border-b border-gray-700">
          <h3 className="text-lg font-semibold text-white flex items-center gap-2">
            <FaHistory /> Extrato
          </h3>
        </div>

        {transactions.length === 0 ? (
          <div className="p-8 text-center text-gray-500">
            Nenhuma transação encontrada
          </div>
        ) : (
          <div className="divide-y divide-gray-700">
            {transactions.map((tx, i) => (
              <div
                key={tx.id || tx._id || i}
                className="flex items-center justify-between p-4 hover:bg-gray-700/30 transition"
              >
                <div className="flex items-center gap-3">
                  {getTransactionIcon(tx.type)}
                  <div>
                    <p className="text-white text-sm">
                      {tx.description || tx.type}
                    </p>
                    <p className="text-gray-500 text-xs">
                      {formatDate(tx.created_at)}
                    </p>
                    {tx.payment_ref && (
                      <p className="text-gray-600 text-xs font-mono">
                        {tx.payment_ref}
                      </p>
                    )}
                  </div>
                </div>
                <div className="text-right">
                  <p
                    className={`font-semibold ${
                      tx.type === "CREDIT" || tx.type === "PAYMENT"
                        ? "text-green-400"
                        : tx.type === "WITHDRAWAL"
                        ? "text-yellow-400"
                        : "text-red-400"
                    }`}
                  >
                    {tx.type === "CREDIT" || tx.type === "PAYMENT"
                      ? "+"
                      : "-"}{" "}
                    {formatCurrency(Math.abs(tx.amount))}
                  </p>
                  {tx.balance != null && (
                    <p className="text-gray-600 text-xs">
                      Saldo: {formatCurrency(tx.balance)}
                    </p>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}

        {hasMore && (
          <div className="p-4 text-center border-t border-gray-700">
            <button
              onClick={loadMore}
              className="px-6 py-2 bg-gray-700 hover:bg-gray-600 rounded-lg text-gray-300 text-sm transition"
            >
              Carregar mais
            </button>
          </div>
        )}
      </div>

      {/* Info sobre o fluxo */}
      <div className="bg-gray-800/30 rounded-xl p-4 border border-gray-700/30">
        <p className="text-gray-500 text-xs leading-relaxed">
          <strong className="text-gray-400">Como funciona:</strong> Após a
          entrega ser confirmada, o pagamento fica pendente por 48h (janela
          anti-fraude). Pagamentos de baixo risco são aprovados automaticamente
          pelo sistema. Pagamentos de alto valor ou alto risco passam por análise
          de compliance. Após aprovação, o valor é creditado na sua carteira e
          fica disponível para saque.
        </p>
      </div>
    </div>
  );
}
