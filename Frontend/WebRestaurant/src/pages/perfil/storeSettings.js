// Regras da página Ajustes (perfil/index.js). Funções puras, testadas em
// __tests__/storeSettings.test.js.

export const DAYS = ["Domingo", "Segunda", "Terça", "Quarta", "Quinta", "Sexta", "Sábado"];
const SHORT = ["Dom", "Seg", "Ter", "Qua", "Qui", "Sex", "Sáb"];

export function defaultHours() {
  return DAYS.map((_, i) => ({
    day_of_week: i,
    is_open: i !== 0,
    open_time: "08:00",
    close_time: "22:00",
    break_start_time: "",
    break_end_time: "",
  }));
}

// Junta o que veio do servidor com a semana padrão: dia não cadastrado fica
// com o padrão, e nunca sobram buracos na grade.
export function mergeHours(loaded) {
  const list = Array.isArray(loaded) ? loaded : [];
  return defaultHours().map((d) => {
    const found = list.find((h) => Number(h.day_of_week) === d.day_of_week);
    return found ? { ...d, ...found, day_of_week: d.day_of_week } : d;
  });
}

// Dia aberto precisa de abertura e fechamento; fechamento antes da abertura
// é aceito (turno que vira a madrugada, ex.: 18:00 às 02:00).
export function validateHours(hours) {
  for (const h of hours) {
    if (h.is_open && (!h.open_time || !h.close_time)) {
      return `Informe abertura e fechamento de ${DAYS[h.day_of_week]}.`;
    }
    if (h.is_open && h.open_time === h.close_time) {
      return `Abertura e fechamento iguais em ${DAYS[h.day_of_week]}.`;
    }
  }
  return null;
}

export function hoursPayload(hours, establishmentId) {
  const id = Number(establishmentId);
  return hours.map((h) => ({
    establishment_id: id,
    day_of_week: h.day_of_week,
    is_open: !!h.is_open,
    open_time: h.open_time || "",
    close_time: h.close_time || "",
    break_start_time: h.break_start_time || "",
    break_end_time: h.break_end_time || "",
  }));
}

// Resumo em texto da grade ("Seg a Sáb, 08:00–22:00"). Grava no campo
// antigo horarioFuncionamento, que era digitado à mão e repetia (às vezes
// contradizia) a grade semanal.
export function summarizeHours(hours) {
  const open = hours.filter((h) => h.is_open);
  if (open.length === 0) return "Fechado";
  const same = open.every((h) => h.open_time === open[0].open_time && h.close_time === open[0].close_time);
  if (!same) return "Horários variados — veja a grade";
  const range = `${open[0].open_time}–${open[0].close_time}`;
  if (open.length === 7) return `Todos os dias, ${range}`;
  const days = open.map((h) => h.day_of_week);
  const consecutive = days.every((d, i) => i === 0 || d === days[i - 1] + 1);
  const label = consecutive && days.length > 2
    ? `${SHORT[days[0]]} a ${SHORT[days[days.length - 1]]}`
    : days.map((d) => SHORT[d]).join(", ");
  return `${label}, ${range}`;
}

// ---------------------------------------------------------------------------
// Endereço → coordenadas (Nominatim/OpenStreetMap, gratuito, sem chave).
// Loja cadastrada pelo site ficava com lat/long 0,0 e o cálculo de distância
// e taxa de entrega não funcionava para ela; os campos eram só leitura.
// ---------------------------------------------------------------------------

export function geocodeUrl(address) {
  const q = encodeURIComponent(`${String(address || "").trim()}, Brasil`);
  return `https://nominatim.openstreetmap.org/search?format=json&limit=1&countrycodes=br&q=${q}`;
}

export function parseGeocode(data) {
  if (!Array.isArray(data) || data.length === 0) return null;
  const lat = parseFloat(data[0].lat);
  const long = parseFloat(data[0].lon);
  if (!Number.isFinite(lat) || !Number.isFinite(long)) return null;
  return { lat, long };
}

export async function geocodeAddress(address, fetchImpl = fetch) {
  if (!String(address || "").trim()) return null;
  try {
    const resp = await fetchImpl(geocodeUrl(address), { headers: { Accept: "application/json" } });
    if (!resp.ok) return null;
    return parseGeocode(await resp.json());
  } catch {
    return null;
  }
}

// Busca coordenadas quando o endereço mudou ou a loja ainda está em 0,0.
export function needsGeocode(savedAddress, establishment) {
  const addr = String(establishment?.location_string || "").trim();
  if (!addr) return false;
  if (addr !== String(savedAddress || "").trim()) return true;
  return !Number(establishment?.lat) || !Number(establishment?.long);
}

// Corpo do PUT /establishments/:id com os tipos que o servidor espera. O
// input number devolve texto ("6") e o Go recusava o JSON inteiro (400) —
// mexer na distância máxima fazia o salvar falhar sempre.
export function establishmentPayload(est, hours) {
  const num = (v) => {
    const n = parseFloat(v);
    return Number.isFinite(n) ? n : 0;
  };
  return {
    name: String(est?.name || "").trim(),
    description: String(est?.description || "").trim(),
    image: est?.image || "",
    primary_color: est?.primary_color || "#DC2626",
    secondary_color: est?.secondary_color || "#F59E0B",
    horarioFuncionamento: hours ? summarizeHours(hours) : est?.horarioFuncionamento || "",
    lat: num(est?.lat),
    long: num(est?.long),
    location_string: String(est?.location_string || "").trim(),
    max_distance_delivery: num(est?.max_distance_delivery),
  };
}
