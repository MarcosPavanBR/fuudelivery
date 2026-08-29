import Strings from "../constants/Strings";
import api from "./api";

async function handlerVinculoProdutoAdicional(productID, additionalID) {
  const { data } = await api.post("/additional/product", {
    productID: parseInt(productID),
    additionalID: parseInt(additionalID),
  });
  return true;
}

async function getAdditionals(id) {
  try {
    const { data } = await api.get("/additional/" + id);
    return data;
  } catch (e) {
    console.error(e);
    return [];
  }
}

async function updateAdditional(items, editItem) {
  const body = { ...editItem, Price: parseFloat(editItem.Price ?? 0) };
  const { data } = await api.put(
    "/additional/" + editItem.ID,
    body
  );
  return items.map((e) => (e.ID === editItem.ID ? data : e));
}

async function createAdditional(items, editItem, establishmentId) {
  const body = {
    ...editItem,
    EstablishmentId: establishmentId,
    Price: parseFloat(editItem.Price ?? 0),
  };
  const { data } = await api.post("/additional", body);
  return [data, ...items.filter((e) => e.ID && e.ID !== Strings.id_default)];
}

async function deleteAdditional(items, id) {
  await api.delete("/additional/" + id);
  return [...items.filter((e) => e.ID !== id)];
}

export default {
  handlerVinculoProdutoAdicional,
  getAdditionals,
  deleteAdditional,
  updateAdditional,
  createAdditional,
};
