import api from "./api";

// Destaque patrocinado por dia (servidor: payment_api/app/handlers/sponsor.go).
export async function getOffer(days) {
  const { data } = await api.get("/sponsorship/offer", { params: { days } });
  return data;
}

// Propaga o erro: a tela lê status (402 sem saldo, 409 dia lotado) e o
// next_available do corpo.
export async function book({ startDay, days, payWith }) {
  const { data } = await api.post("/sponsorship/bookings", {
    start_day: startDay,
    days,
    pay_with: payWith,
  });
  return data;
}

export async function cancelBooking(id) {
  const { data } = await api.post(`/sponsorship/bookings/${id}/cancel`);
  return data;
}
