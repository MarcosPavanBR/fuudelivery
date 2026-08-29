import api from "./api";

async function updateEstablishment(id, body) {
  const { data } = await api.put("/establishments/" + id, {
    establishment: body,
  });
  return data;
}

export default {
  updateEstablishment,
};
