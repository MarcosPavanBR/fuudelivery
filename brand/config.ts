export const APP_MODE_OPTIONS = {
  unique: 1,
  multi: 2,
};

export const APP_MODE = APP_MODE_OPTIONS.multi;

export const ESTABLISHMENT_ID = 1;
export const ESTABLISHMENT = {
  id: ESTABLISHMENT_ID,
  name: "FUUDELIVERY",
  image: "/brand/logos/logo-icon.svg",
  horarioFuncionamento: "22h",
  lat: -21.778131,
  long: -43.367493,
  max_distance_delivery: 10,
};

export const PAYMENT_TYPE = [
  { type: "credit", icon: "credit-score", label: "Cartão de Crédito" },
  { type: "debit", icon: "credit-card", label: "Cartão de Débito" },
  { type: "money", icon: "money", label: "Dinheiro" },
  { type: "pix", icon: "pix", label: "PIX" },
];

export const DELIVERY_STATUS = {
  AWAIT_APPROVE: { label: "Aguardando Aprovação", color: "#F59E0B" },
  APPROVED: { label: "Preparando seu pedido", color: "#3B82F6" },
  IN_ROUTE_COLECT: { label: "À caminho da coleta", color: "#8B5CF6" },
  AWAIT_COLECT: { label: "Aguardando coleta", color: "#DC2626" },
  DONE: { label: "Em rota de entrega", color: "#10B981" },
  IN_ROUTE_DELIVERY: { label: "Em rota de entrega", color: "#10B981" },
  FINISH: { label: "Entregue", color: "#6B7280" },
  FINISHED: { label: "Entregue", color: "#6B7280" },
  CANCELLED: { label: "Cancelado", color: "#EF4444" },
};
