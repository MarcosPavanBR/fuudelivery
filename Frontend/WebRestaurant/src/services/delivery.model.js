import api from "./api";

export async function saveDelivery({ establishmentId, fixedTaxa, perKm }) {
  try {
    const { data } = await api.post("/delivery", {
      establishmentId,
      fixedTaxa: parseFloat(fixedTaxa),
      perKm: parseFloat(perKm),
    });
    return true;
  } catch (e) {
    console.error(e);
    return false;
  }
}

export async function getDelivery(establishmentId) {
  try {
    const { data } = await api.get(
      `/delivery/value/${establishmentId}`
    );
    return data;
  } catch (e) {
    // 404 = loja ainda não configurou as taxas: não é erro.
    if (e?.response?.status !== 404) console.error(e);
    return { fixedTaxa: 0, perKm: 0 };
  }
}

export default {
  getDelivery,
  saveDelivery,
};
