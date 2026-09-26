import React, { useCallback, useEffect, useState } from "react";
import { toast } from "react-toastify";
import { FiLoader, FiStar, FiTrendingUp, FiCalendar, FiX } from "react-icons/fi";
import MenuLayout from "../../components/Menu";
import helper from "../../helpers/helper";
import { book, cancelBooking, getOffer } from "../../services/sponsorship.model";
import { STATUS_LABEL, canCancel, dayRange, firstFullDay, shortDay, totalFor, weekdayShort } from "./sponsorship";

// Destaque patrocinado: a loja compra dias no topo do app do cliente.
function Destaque() {
  const [offer, setOffer] = useState(null);
  const [loading, setLoading] = useState(true);
  const [days, setDays] = useState(1);
  const [start, setStart] = useState("");
  const [payWith, setPayWith] = useState("wallet");
  const [sending, setSending] = useState(false);
  const [showPix, setShowPix] = useState(null);

  const load = useCallback(async (d = days) => {
    try {
      const data = await getOffer(d);
      setOffer(data);
      setStart((s) => s || data.next_available || data.today);
    } catch (e) {
      toast.error("Não foi possível carregar o destaque.");
    }
    setLoading(false);
  }, [days]);

  useEffect(() => {
    load(days);
  }, [days]); // eslint-disable-line react-hooks/exhaustive-deps

  // Com PIX aguardando pagamento, atualiza sozinho: o destaque liga quando o
  // webhook do gateway confirma, sem a loja precisar recarregar a página.
  const waitingPix = (offer?.bookings || []).some((b) => b.status === "pending_payment" && b.pix_copy_paste);
  useEffect(() => {
    if (!waitingPix) return undefined;
    const t = setInterval(() => load(days), 5000);
    return () => clearInterval(t);
  }, [waitingPix, days]); // eslint-disable-line react-hooks/exhaustive-deps

  if (loading || !offer) {
    return (
      <MenuLayout>
        <div className="flex items-center justify-center h-32">
          <FiLoader className="animate-spin h-6 w-6" style={{ color: "#DC2626" }} />
        </div>
      </MenuLayout>
    );
  }

  const total = totalFor(offer.price_per_day, days);
  const chosen = start ? dayRange(start, days) : [];
  const full = start ? firstFullDay(offer.calendar, start, days) : null;
  const walletShort = payWith === "wallet" && offer.wallet_balance < total;

  const submit = async () => {
    setSending(true);
    try {
      const data = await book({ startDay: start, days, payWith });
      toast.success(data.message || "Destaque reservado!");
      if (data.booking?.pix_copy_paste) setShowPix(data.booking.id);
      setStart("");
      await load(days);
    } catch (e) {
      const body = e?.response?.data || {};
      toast.error(body.error || "Não foi possível reservar.");
      if (body.next_available) setStart(body.next_available);
      await load(days);
    }
    setSending(false);
  };

  const cancel = async (b) => {
    if (!window.confirm(`Cancelar o destaque de ${shortDay(b.start_day)} a ${shortDay(b.end_day)}?`)) return;
    try {
      await cancelBooking(b.id);
      toast.success(b.status === "active" ? "Cancelado. O valor voltou para a sua carteira." : "Reserva cancelada.");
      await load(days);
    } catch (e) {
      toast.error(e?.response?.data?.error || "Não foi possível cancelar.");
    }
  };

  return (
    <MenuLayout>
      <div className="space-y-6 animate-fade-in">
        <div className="card p-6">
          <div className="flex items-center gap-2 mb-2">
            <div className="p-2 rounded-lg bg-red-50">
              <FiTrendingUp className="h-5 w-5" style={{ color: "#DC2626" }} />
            </div>
            <h3 className="text-lg font-bold text-gray-900 dark:text-white">Destaque no app</h3>
          </div>
          <p className="text-sm text-gray-600 dark:text-gray-300">
            Sua loja aparece no topo da lista do app do cliente, com o selo <b>Patrocinado</b>, por{" "}
            <b>{helper.formatCurrency(offer.price_per_day)} por dia</b>. São só {offer.slots_per_day} vagas por
            dia: quem reserva primeiro garante. Entre as lojas do dia a ordem gira a cada hora, e todas ficam o
            mesmo tempo em 1º lugar. O destaque só aparece enquanto a loja está aberta.
          </p>
        </div>

        <div className="card p-6">
          <div className="flex items-center gap-2 mb-4">
            <FiCalendar className="h-5 w-5 text-gray-500" />
            <h4 className="font-semibold text-gray-900 dark:text-white">Vagas nos próximos 30 dias</h4>
          </div>
          <div className="grid grid-cols-5 sm:grid-cols-7 lg:grid-cols-10 gap-2">
            {offer.calendar.map((c) => {
              const isFull = c.free <= 0;
              const selected = !isFull && chosen.includes(c.day);
              return (
                <button
                  key={c.day}
                  type="button"
                  disabled={isFull}
                  onClick={() => setStart(c.day)}
                  title={isFull ? "Sem vagas" : `${c.free} vaga(s)`}
                  className={`rounded-lg border px-1 py-2 text-center text-xs transition-colors ${
                    isFull
                      ? "bg-gray-100 text-gray-400 border-gray-100 cursor-not-allowed line-through"
                      : selected
                      ? "bg-[#DC2626] text-white border-[#DC2626]"
                      : "bg-white text-gray-700 border-gray-200 hover:border-red-300"
                  }`}
                >
                  <div className="font-semibold">{shortDay(c.day)}</div>
                  <div className={selected ? "text-red-100" : "text-gray-400"}>{weekdayShort(c.day)}</div>
                  <div className={selected ? "text-white" : isFull ? "" : "text-green-700"}>
                    {isFull ? "lotado" : `${c.free} livre${c.free > 1 ? "s" : ""}`}
                  </div>
                </button>
              );
            })}
          </div>

          <div className="mt-6 grid grid-cols-1 sm:grid-cols-3 gap-4">
            <div>
              <label className="block text-xs font-semibold text-gray-500 uppercase mb-2">Início</label>
              <input className="input" readOnly value={start ? `${shortDay(start)} (${weekdayShort(start)})` : "Escolha um dia"} />
            </div>
            <div>
              <label className="block text-xs font-semibold text-gray-500 uppercase mb-2">Quantos dias</label>
              <select className="input" value={days} onChange={(e) => setDays(Number(e.target.value))}>
                {[1, 2, 3, 5, 7, 10, 15, 30].filter((n) => n <= offer.max_days).map((n) => (
                  <option key={n} value={n}>{n} dia{n > 1 ? "s" : ""}</option>
                ))}
              </select>
            </div>
            <div>
              <label className="block text-xs font-semibold text-gray-500 uppercase mb-2">Pagamento</label>
              <select className="input" value={payWith} onChange={(e) => setPayWith(e.target.value)}>
                <option value="wallet">Saldo da carteira ({helper.formatCurrency(offer.wallet_balance)})</option>
                <option value="pix">PIX (confirmado pelo suporte)</option>
              </select>
            </div>
          </div>

          {full && (
            <p className="mt-3 text-sm text-amber-700">
              {shortDay(full)} já está lotado.{" "}
              {offer.next_available && (
                <button type="button" className="underline font-semibold" onClick={() => setStart(offer.next_available)}>
                  Próxima data com {days} dia(s) livres: {shortDay(offer.next_available)}
                </button>
              )}
            </p>
          )}
          {walletShort && (
            <p className="mt-3 text-sm text-amber-700">Saldo insuficiente na carteira. Escolha PIX ou menos dias.</p>
          )}
          {payWith === "pix" && (
            <p className="mt-3 text-xs text-gray-500">
              A vaga fica guardada por 24 h. Depois de reservar aparece o QR Code do PIX; o destaque liga sozinho
              quando o pagamento cair. Se cancelar antes de começar, o valor volta para a sua carteira.
            </p>
          )}

          <div className="mt-6 flex flex-wrap items-center justify-between gap-3">
            <p className="text-lg font-bold text-gray-900 dark:text-white">
              Total: {helper.formatCurrency(total)}
            </p>
            <button
              type="button"
              className="btn btn-primary"
              disabled={sending || !start || !!full || walletShort}
              onClick={submit}
            >
              {sending ? <FiLoader className="h-5 w-5 animate-spin" /> : <FiStar className="h-5 w-5" />}
              Reservar destaque
            </button>
          </div>
        </div>

        <div className="card p-6">
          <h4 className="font-semibold text-gray-900 dark:text-white mb-3">Seus destaques</h4>
          {offer.bookings.length === 0 ? (
            <p className="text-sm text-gray-500">Nenhum destaque reservado.</p>
          ) : (
            <ul className="divide-y divide-gray-100">
              {offer.bookings.map((b) => (
                <li key={b.id} className="flex flex-wrap items-center justify-between gap-2 py-3">
                  <div>
                    <p className="font-medium text-gray-900 dark:text-white">
                      {shortDay(b.start_day)} a {shortDay(b.end_day)} · {b.days} dia{b.days > 1 ? "s" : ""}
                    </p>
                    <p className="text-xs text-gray-500">
                      {helper.formatCurrency(b.total)} · {b.pay_with === "wallet" ? "carteira" : "PIX"}
                    </p>
                  </div>
                  <div className="flex items-center gap-2">
                    <span
                      className={`rounded-full px-2.5 py-1 text-xs font-semibold ${
                        b.status === "active" ? "bg-green-50 text-green-700" : "bg-amber-50 text-amber-700"
                      }`}
                    >
                      {STATUS_LABEL[b.status] || b.status}
                    </span>
                    {b.status === "pending_payment" && b.pix_copy_paste && (
                      <button type="button" className="btn btn-primary" onClick={() => setShowPix(showPix === b.id ? null : b.id)}>
                        {showPix === b.id ? "Ocultar PIX" : "Pagar PIX"}
                      </button>
                    )}
                    {canCancel(b, offer.today) && (
                      <button type="button" className="btn btn-ghost" onClick={() => cancel(b)}>
                        <FiX className="h-4 w-4" /> Cancelar
                      </button>
                    )}
                  </div>
                  {showPix === b.id && b.pix_copy_paste && (
                    <div className="w-full mt-3 flex flex-col sm:flex-row items-center gap-4 rounded-lg bg-gray-50 p-4">
                      {b.pix_qr_base64 && (
                        <img
                          src={`data:image/png;base64,${b.pix_qr_base64}`}
                          alt="QR Code PIX"
                          className="h-40 w-40 rounded bg-white p-2"
                        />
                      )}
                      <div className="flex-1 w-full">
                        <p className="text-sm font-semibold text-gray-900">
                          PIX de {helper.formatCurrency(b.total)} — o destaque liga sozinho quando o pagamento cair.
                        </p>
                        <textarea readOnly className="input mt-2 h-20 text-xs font-mono" value={b.pix_copy_paste} />
                        <button
                          type="button"
                          className="btn btn-ghost mt-2"
                          onClick={() => {
                            navigator.clipboard?.writeText(b.pix_copy_paste).then(
                              () => toast.success("Código PIX copiado."),
                              () => toast.error("Não foi possível copiar.")
                            );
                          }}
                        >
                          Copiar código
                        </button>
                      </div>
                    </div>
                  )}
                </li>
              ))}
            </ul>
          )}
        </div>
      </div>
    </MenuLayout>
  );
}

export default Destaque;
