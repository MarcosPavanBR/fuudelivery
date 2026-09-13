import React, { useEffect, useState, useCallback } from "react";
import { FiLoader, FiCheckCircle, FiAlertTriangle, FiExternalLink } from "react-icons/fi";
import { toast } from "react-toastify";
import { getMercadoPagoStatus, connectMercadoPago } from "../services/payment.model";

// ConectarMercadoPago — card do perfil onde o restaurante liga a PRÓPRIA conta
// Mercado Pago. A partir daí a fatia das vendas dele cai direto na conta dele;
// o dinheiro não passa pela plataforma.
//
// Estados: carregando · não conectado · conectado · expirado (reconectar).
// O botão inicia o OAuth (redireciona ao MP); na volta, o backend manda de
// volta para cá com ?mercadopago=ok|erro, que vira um toast.
function ConectarMercadoPago() {
  const [status, setStatus] = useState(null);
  const [loading, setLoading] = useState(true);
  const [connecting, setConnecting] = useState(false);

  const carregar = useCallback(async () => {
    setLoading(true);
    try {
      setStatus(await getMercadoPagoStatus());
    } catch (e) {
      // 503 = OAuth não configurado no servidor ainda; não é erro do lojista.
      setStatus({ connected: false, unavailable: e?.response?.status === 503 });
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    carregar();
    // Toast do retorno do callback (?mercadopago=ok|erro), limpando a query.
    const params = new URLSearchParams(window.location.search);
    const r = params.get("mercadopago");
    if (r === "ok") toast.success("Conta Mercado Pago conectada.");
    else if (r === "erro") toast.error("Não foi possível conectar a conta. Tente de novo.");
    if (r) {
      params.delete("mercadopago");
      const qs = params.toString();
      window.history.replaceState({}, "", window.location.pathname + (qs ? "?" + qs : ""));
    }
  }, [carregar]);

  const conectar = async () => {
    setConnecting(true);
    try {
      const { authorize_url } = await connectMercadoPago();
      if (!authorize_url) throw new Error("sem authorize_url");
      window.location.href = authorize_url; // sai da página para o MP
    } catch (e) {
      setConnecting(false);
      if (e?.response?.status === 503) {
        toast.error("Pagamento por conta própria ainda não habilitado no servidor.");
      } else {
        toast.error("Não foi possível iniciar a conexão. Tente de novo.");
      }
    }
  };

  const conectado = status?.connected;
  const expirado = status?.expired;

  return (
    <div className="card p-6">
      <div className="mb-4 flex items-center gap-2">
        <h3 className="text-lg font-bold text-gray-900 dark:text-white">
          Conta de recebimento
        </h3>
      </div>

      <p className="mb-4 text-sm text-gray-600 dark:text-gray-300">
        Conecte a conta Mercado Pago do seu restaurante. A partir daí, o valor
        das suas vendas cai <strong>direto na sua conta</strong> — só a comissão
        da plataforma é descontada na hora. Sem uma conta conectada, os pedidos
        não podem ser cobrados.
      </p>

      {loading ? (
        <div className="flex items-center gap-2 text-gray-500">
          <FiLoader className="h-5 w-5 animate-spin" /> Verificando…
        </div>
      ) : conectado ? (
        <div className="flex flex-col gap-3">
          <div className="flex items-center gap-2 text-green-600 dark:text-green-400">
            <FiCheckCircle className="h-5 w-5" />
            <span className="font-medium">
              Conta conectada{status?.mp_user_id ? ` (ID ${status.mp_user_id})` : ""}.
            </span>
          </div>
          <button
            type="button"
            onClick={conectar}
            disabled={connecting}
            className="btn btn-ghost self-start"
          >
            {connecting ? <FiLoader className="h-5 w-5 animate-spin" /> : <FiExternalLink className="h-5 w-5" />}
            Reconectar
          </button>
        </div>
      ) : (
        <div className="flex flex-col gap-3">
          {expirado && (
            <div className="flex items-center gap-2 text-amber-600 dark:text-amber-400">
              <FiAlertTriangle className="h-5 w-5" />
              <span>A conexão expirou. Reconecte para continuar recebendo.</span>
            </div>
          )}
          {status?.unavailable && (
            <div className="flex items-center gap-2 text-amber-600 dark:text-amber-400">
              <FiAlertTriangle className="h-5 w-5" />
              <span>Recurso ainda não habilitado no servidor.</span>
            </div>
          )}
          <button
            type="button"
            onClick={conectar}
            disabled={connecting || status?.unavailable}
            className="btn btn-primary self-start"
          >
            {connecting ? <FiLoader className="h-5 w-5 animate-spin" /> : <FiExternalLink className="h-5 w-5" />}
            {expirado ? "Reconectar Mercado Pago" : "Conectar Mercado Pago"}
          </button>
        </div>
      )}
    </div>
  );
}

export default ConectarMercadoPago;
