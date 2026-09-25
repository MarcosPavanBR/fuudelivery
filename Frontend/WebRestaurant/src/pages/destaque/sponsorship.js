// Regras da tela Destaque (pages/destaque/index.js). Funções puras, testadas
// em __tests__/sponsorship.test.js.

export const STATUS_LABEL = {
  active: "Pago",
  pending_payment: "Aguardando PIX",
  cancelled: "Cancelado",
};

// Os N dias a partir de start ("AAAA-MM-DD"), sem fuso: aritmética em UTC
// só para andar o calendário — a data já é a data comercial do servidor.
export function dayRange(start, days) {
  const [y, m, d] = String(start).split("-").map(Number);
  if (!y || !m || !d || days < 1) return [];
  return Array.from({ length: days }, (_, i) =>
    new Date(Date.UTC(y, m - 1, d + i)).toISOString().slice(0, 10)
  );
}

// Primeiro dia lotado do período escolhido (ou null se cabe).
export function firstFullDay(calendar, start, days) {
  const free = new Map((calendar || []).map((c) => [c.day, c.free]));
  for (const d of dayRange(start, days)) {
    if (free.has(d) && free.get(d) <= 0) return d;
  }
  return null;
}

export function totalFor(pricePerDay, days) {
  return Math.round(Number(pricePerDay || 0) * Number(days || 0) * 100) / 100;
}

// "26/09" para a grade e mensagens.
export function shortDay(day) {
  const [, m, d] = String(day).split("-");
  return d && m ? `${d}/${m}` : String(day || "");
}

export function weekdayShort(day) {
  const [y, m, d] = String(day).split("-").map(Number);
  if (!y) return "";
  return ["dom", "seg", "ter", "qua", "qui", "sex", "sáb"][new Date(Date.UTC(y, m - 1, d)).getUTCDay()];
}

// A loja só cancela o que ainda não começou (o servidor confere de novo).
export function canCancel(booking, today) {
  return booking?.status !== "cancelled" && String(booking?.start_day) > String(today);
}
