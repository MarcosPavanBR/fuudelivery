const APP_MODE_OPTIONS = {
  unique: 1,
  multi: 2,
};

const APP_MODE = APP_MODE_OPTIONS.multi;

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

const PAYMENT_TYPE = [
  { type: "credit", icon: "credit-score", label: "Cartão de Crédito" },
  { type: "debit", icon: "credit-card", label: "Cartão de Débito" },
  { type: "money", icon: "money", label: "Dinheiro" },
  { type: "pix", icon: "pix", label: "PIX" },
];

export {
  ESTABLISHMENT_ID,
  ESTABLISHMENT,
  APP_MODE,
  APP_MODE_OPTIONS,
  PAYMENT_TYPE,
};
