const APP_MODE_OPTIONS = {
  unique: 1,
  multi: 2,
};

const APP_MODE = APP_MODE_OPTIONS.multi;

// Estilo de mapa gratuito (OpenFreeMap — tiles do OpenStreetMap, sem API key).
export const MAP_STYLE_URL = "https://tiles.openfreemap.org/styles/liberty";

const ESTABLISHMENT_ID = 1;
const ESTABLISHMENT = {
  id: ESTABLISHMENT_ID,
  name: "FUUDELIVERY",
  image: "",
  horarioFuncionamento: "23h",
  lat: -23.550520,
  long: -46.633308,
  max_distance_delivery: 15,
};

// Cartão de crédito/débito desativado temporariamente (2026-09-07): nenhum
// gateway que suporta cartão (Pagar.me, Asaas, Mercado Pago — ver
// pkg/gateway/router.go) tem credencial configurada em produção. O
// AbacatePay tem credencial mas só suporta PIX, então toda cobrança de
// cartão hoje esgota a fila de fallback e falha. PIX vira o default (era
// credit) por ser o único canal digital realmente funcionando agora.
// Reative assim que PAGARME_API_KEY, ASAAS_API_KEY ou
// MERCADOPAGO_ACCESS_TOKEN existir de verdade no Render.
const PAYMENT_TYPE = [
  { type: "pix", icon: "pix", label: "PIX" },
  { type: "money", icon: "money", label: "Dinheiro" },
  // { type: "credit", icon: "credit-score", label: "Cartão de Crédito" },
  // { type: "debit", icon: "credit-card", label: "Cartão de Débito" },
];

export {
  ESTABLISHMENT_ID,
  ESTABLISHMENT,
  APP_MODE,
  APP_MODE_OPTIONS,
  PAYMENT_TYPE,
};
