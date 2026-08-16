import React, { useState, useEffect, useCallback } from "react";
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

  const fetchWallet = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);

      const [walletData, extractData, health] = await Promise.all([
        getWallet().catch(() => null),
        getExtract(20, "").catch(() => ({ data: [] })),
        getPaymentHealth().catch(() => ({ status: "offline" })),
      ]);

      if (walletData) {
        setWallet(walletData);
      }

      setTransactions(extractData?.data || []);
      setCursor(extractData?.next_cursor || "");
      setHasMore(!!extractData?.next_cursor);
      // /health do monolito responde "up" ou "degraded" (Redis fora) —
      // ambos significam que a API de pagamentos está acessível.
      setPaymentOnline(
        health?.status === "up" || health?.status === "degraded"
      );
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
  }, []);

  useEffect(() => {
    fetchWallet();
  }, [fetchWallet]);

  const loadMore = async () => {
    if (!cursor) return;
    try {
      const more = await getExtract(20, cursor);
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
      await requestWithdraw({
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
      <div className="max-w-4xl mx-auto p-4 space-y-6">
        <div className="flex items-center gap-2">
          <div className="skeleton h-3 w-3 rounded-full" />
          <div className="skeleton h-4 w-44" />
        </div>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          {[...Array(3)].map((_, i) => (
            <div key={i} className="card rounded-xl p-6">
              <div className="skeleton h-4 w-28 mb-2" />
              <div className="skeleton h-9 w-24" />
            </div>
          ))}
        </div>
        <div className="grid grid-cols-2 gap-4">
          <div className="card rounded-xl p-4">
            <div className="skeleton h-4 w-20 mb-2" />
            <div className="skeleton h-6 w-28" />
          </div>
          <div className="card rounded-xl p-4">
            <div className="skeleton h-4 w-20 mb-2" />
            <div className="skeleton h-6 w-28" />
          </div>
        </div>
        <div className="card rounded-xl">
          <div className="p-4 border-b border-gray-200">
            <div className="skeleton h-5 w-32" />
          </div>
          <div className="p-6 space-y-2">
            {[...Array(4)].map((_, i) => (
              <div key={i} className="skeleton h-10 w-full" />
            ))}
          </div>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center min-h-[400px]">
        <FaExclamationTriangle className="text-4xl text-yellow-500 mb-4" />
        <p className="text-gray-700 mb-2">Não foi possível carregar a carteira</p>
        <p className="text-gray-500 text-sm mb-4">{error}</p>
        <button
          onClick={fetchWallet}
          className="btn btn-primary"
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
        <div className="card rounded-xl p-6">
          <div className="flex items-center gap-2 mb-3">
            <FaWallet className="text-green-500" />
            <span className="text-gray-500 text-sm">Saldo Disponível</span>
          </div>
          <p className="text-3xl font-bold text-green-500">
            {formatCurrency(wallet?.available)}
          </p>
          <p className="text-gray-400 text-xs mt-1">Pronto para saque</p>
        </div>

        <div className="card rounded-xl p-6">
          <div className="flex items-center gap-2 mb-2">
            <FaSpinner className="text-yellow-500" />
            <span className="text-gray-500 text-sm">Saldo Pendente</span>
          </div>
          <p className="text-3xl font-bold text-yellow-500">
            {formatCurrency(wallet?.pending)}
          </p>
          <p className="text-gray-400 text-xs mt-1">
            Aguardando aprovação do sistema
          </p>
        </div>

        <div className="card rounded-xl p-6">
          <div className="flex items-center gap-2 mb-2">
            <FaLock className="text-red-400" />
            <span className="text-gray-500 text-sm">Saldo Bloqueado</span>
          </div>
          <p className="text-3xl font-bold text-red-400">
            {formatCurrency(wallet?.blocked)}
          </p>
          <p className="text-gray-400 text-xs mt-1">
            Retido por disputa ou estorno
          </p>
        </div>
      </div>

      {/* Totais */}
      <div className="grid grid-cols-2 gap-4">
        <div className="card rounded-xl p-4">
          <span className="text-gray-500 text-sm">Total ganho</span>
          <p className="text-xl font-semibold text-gray-900">
            {formatCurrency(wallet?.total_earned)}
          </p>
        </div>
        <div className="card rounded-xl p-4">
          <span className="text-gray-500 text-sm">Total sacado</span>
          <p className="text-xl font-semibold text-gray-900">
            {formatCurrency(wallet?.total_withdrawn)}
          </p>
        </div>
      </div>

      {/* Botão de saque */}
      {wallet?.available > 0 && (
        <button
          onClick={() => setShowWithdraw(true)}
          className="btn btn-primary w-full justify-center py-3"
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
          <div className="bg-white rounded-xl p-6 w-full max-w-md border border-gray-200 shadow-xl">
            <h3 className="text-xl font-bold text-gray-900 mb-4">
              Solicitar Saque
            </h3>

            <div className="space-y-4">
              <div>
                <label className="text-gray-500 text-sm block mb-2">
                  Valor (mínimo R$ 10,00)
                </label>
                <input
                  type="number"
                  value={withdrawAmount}
                  onChange={(e) => setWithdrawAmount(e.target.value)}
                  placeholder="0.00"
                  min="10"
                  step="0.01"
                  className="input"
                />
                <p className="text-gray-400 text-xs mt-1">
                  Disponível: {formatCurrency(wallet?.available)}
                </p>
              </div>

              <div>
                <label className="text-gray-500 text-sm block mb-2">
                  Método
                </label>
                <div className="flex gap-2">
                  <button
                    onClick={() => setWithdrawMethod("PIX")}
                    className={`flex-1 py-2 rounded-lg flex items-center justify-center gap-2 transition ${
                      withdrawMethod === "PIX"
                        ? "bg-[#EA1D2C] text-white"
                        : "bg-gray-100 text-gray-500"
                    }`}
                  >
                    <FaQrcode /> PIX
                  </button>
                  <button
                    onClick={() => setWithdrawMethod("TED")}
                    className={`flex-1 py-2 rounded-lg flex items-center justify-center gap-2 transition ${
                      withdrawMethod === "TED"
                        ? "bg-[#EA1D2C] text-white"
                        : "bg-gray-100 text-gray-500"
                    }`}
                  >
                    <FaUniversity /> TED
                  </button>
                </div>
              </div>

              <div>
                <label className="text-gray-500 text-sm block mb-2">
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
                  className="input"
                />
              </div>
            </div>

            <div className="flex gap-2 mt-6">
              <button
                onClick={() => setShowWithdraw(false)}
                className="flex-1 btn btn-ghost"
              >
                Cancelar
              </button>
              <button
                onClick={handleWithdraw}
                disabled={withdrawing}
                className="flex-1 btn btn-primary justify-center"
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
        <div className="flex items-center justify-between p-4 border-b border-gray-200">
          <h3 className="text-lg font-semibold text-gray-900 flex items-center gap-2">
            <FaHistory /> Extrato
          </h3>
        </div>

        {transactions.length === 0 ? (
          <div className="p-8 text-center text-gray-400">
            Nenhuma transação encontrada
          </div>
        ) : (
          <div className="divide-y divide-gray-200">
            {transactions.map((tx, i) => (
              <div
                key={tx.id || tx._id || i}
                className="flex items-center justify-between p-4 hover:bg-gray-50 transition"
              >
                <div className="flex items-center gap-3">
                  {getTransactionIcon(tx.type)}
                  <div>
                    <p className="text-gray-900 text-sm">
                      {tx.description || tx.type}
                    </p>
                    <p className="text-gray-400 text-xs">
                      {formatDate(tx.created_at)}
                    </p>
                    {tx.payment_ref && (
                      <p className="text-gray-400 text-xs font-mono">
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
                    <p className="text-gray-400 text-xs">
                      Saldo: {formatCurrency(tx.balance)}
                    </p>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}

        {hasMore && (
          <div className="p-4 text-center border-t border-gray-200">
            <button
              onClick={loadMore}
              className="btn btn-ghost"
            >
              Carregar mais
            </button>
          </div>
        )}
      </div>

      {/* Info sobre o fluxo */}
      <div className="card rounded-xl p-4">
        <p className="text-gray-400 text-xs leading-relaxed">
          <strong className="text-gray-600">Como funciona:</strong> Após a
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
