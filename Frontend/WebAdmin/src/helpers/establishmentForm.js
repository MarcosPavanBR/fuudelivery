// Formulário de estabelecimento do painel admin ↔ modelo do servidor
// (Backend/auth_api/app/models/establishment.go). A tela antiga lia e mandava
// campos que não existem (email, city, status, delivery_fee…): a tabela
// aparecia vazia e o PUT, que espera { establishment: {...} } com o registro
// inteiro, gravava nome/descrição/foto/cores em branco.

const num = (v) => {
  const n = parseFloat(v);
  return Number.isFinite(n) ? n : 0;
};

export const emptyForm = {
  name: "",
  description: "",
  location_string: "",
  lat: "",
  long: "",
  max_distance_delivery: 10,
  zone_id: "",
};

export function formFromEstablishment(est) {
  return {
    name: est?.name || "",
    description: est?.description || "",
    location_string: est?.location_string || "",
    lat: est?.lat || "",
    long: est?.long || "",
    max_distance_delivery: est?.max_distance_delivery || 10,
    zone_id: est?.zone_id ? String(est.zone_id) : "",
  };
}

// PUT /establishments/:id — parte do registro atual para não apagar o que o
// formulário não mostra (foto, cores, horário).
export function updatePayload(est, form) {
  const zone = parseInt(form.zone_id, 10);
  return {
    establishment: {
      name: String(form.name || "").trim(),
      description: String(form.description || "").trim(),
      image: est?.image || "",
      primary_color: est?.primary_color || "",
      secondary_color: est?.secondary_color || "",
      horarioFuncionamento: est?.horarioFuncionamento || "",
      location_string: String(form.location_string || "").trim(),
      lat: num(form.lat),
      long: num(form.long),
      max_distance_delivery: num(form.max_distance_delivery),
      ...(Number.isFinite(zone) && zone > 0 ? { zone_id: zone } : {}),
    },
  };
}

// POST /establishments (CreateEstablishment).
export function createPayload(form) {
  const zone = parseInt(form.zone_id, 10);
  return {
    name: String(form.name || "").trim(),
    description: String(form.description || "").trim(),
    address: String(form.location_string || "").trim(),
    latitude: num(form.lat),
    longitude: num(form.long),
    max_distance_delivery: num(form.max_distance_delivery),
    ...(Number.isFinite(zone) && zone > 0 ? { zone_id: zone } : {}),
  };
}

export function matchesSearch(est, term) {
  const t = String(term || "").trim().toLowerCase();
  if (!t) return true;
  return [est.name, est.owner_name, est.owner_email, est.owner_phone, est.location_string, String(est.id)]
    .some((v) => String(v || "").toLowerCase().includes(t));
}

export function matchesStatus(est, status) {
  if (status === "disabled") return !!est.disabled_at;
  if (status === "open") return !!est.accepting_orders;
  if (status === "closed") return !est.accepting_orders && !est.disabled_at;
  return true;
}

export function formatPhone(p) {
  const d = String(p || "").replace(/\D/g, "").replace(/^55(?=\d{10,11}$)/, "");
  if (d.length === 11) return `(${d.slice(0, 2)}) ${d.slice(2, 7)}-${d.slice(7)}`;
  if (d.length === 10) return `(${d.slice(0, 2)}) ${d.slice(2, 6)}-${d.slice(6)}`;
  return p || "";
}

export async function geocodeAddress(address, fetchImpl = fetch) {
  const a = String(address || "").trim();
  if (!a) return null;
  try {
    const q = encodeURIComponent(`${a}, Brasil`);
    const resp = await fetchImpl(`https://nominatim.openstreetmap.org/search?format=json&limit=1&countrycodes=br&q=${q}`, {
      headers: { Accept: "application/json" },
    });
    if (!resp.ok) return null;
    const data = await resp.json();
    if (!Array.isArray(data) || !data.length) return null;
    const lat = parseFloat(data[0].lat);
    const long = parseFloat(data[0].lon);
    return Number.isFinite(lat) && Number.isFinite(long) ? { lat, long } : null;
  } catch {
    return null;
  }
}
