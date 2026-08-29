import api from "./api";

export async function saveDelivery({ establishmentId, fixedTaxa, perKm }) {
  try {
    const { data } = await api.post("/delivery", {
      establishmentId,
      fixedTaxa: parseFloat(fixedTaxa),
      perKm: parseFloat(perKm),
    });
    return data;
  } catch (e) {
    console.error("[delivery] saveDelivery error:", e?.response?.status, e?.response?.data || e.message);
    throw e;
  }
}

export async function getDelivery(establishmentId) {
  const { data } = await api.get(`/delivery/value/${establishmentId}`);
  return data;
}

export default {
  getDelivery,
  saveDelivery,
};
