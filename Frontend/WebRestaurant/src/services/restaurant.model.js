import api from "./api";

async function updateEstablishment(id, body) {
  try {
    const { data } = await api.put("/establishments/" + id, {
      establishment: body,
    });
    return true;
  } catch (e) {
    console.error(e);
    return false;
  }
}

export default {
  updateEstablishment,
};
