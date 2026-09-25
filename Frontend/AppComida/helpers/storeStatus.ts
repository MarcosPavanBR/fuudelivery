// Textos da vitrine a partir do que GET /establishments devolve
// (Backend/auth_api/app/models/listing.go): is_open, opens_at, opens_day,
// rating, reviews_count, is_sponsored. Testado em __tests__/storeStatus.test.js.

const WEEKDAYS = ["dom", "seg", "ter", "qua", "qui", "sex", "sáb"];

// Loja de servidor antigo (sem o campo) conta como aberta.
export function isStoreOpen(store: any): boolean {
  return store?.is_open !== false;
}

// null quando aberta; senão "Fechado · abre às 18:00" / "amanhã" / "seg".
export function closedLabel(store: any, now: Date = new Date()): string | null {
  if (isStoreOpen(store)) return null;
  const at = store?.opens_at;
  if (!at) return "Fechado";
  const day = Number(store?.opens_day || 0);
  if (day === 0) return `Fechado · abre às ${at}`;
  if (day === 1) return `Fechado · abre amanhã às ${at}`;
  return `Fechado · abre ${WEEKDAYS[(now.getDay() + day) % 7]} às ${at}`;
}

// "★ 4,6 (23)" ou "Novo" para loja sem avaliação.
export function ratingLabel(store: any): string {
  const count = Number(store?.reviews_count || 0);
  const rating = Number(store?.rating || 0);
  if (!count || !rating) return "Novo";
  return `★ ${rating.toFixed(1).replace(".", ",")} (${count})`;
}
