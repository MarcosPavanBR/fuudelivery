import api from "./api";

async function deleteProduct(id) {
  try {
    const { data } = await api.delete("/products/delete/" + id);

    return true;
  } catch (e) {
    console.error(e);
    return false;
  }
}

async function getProducts(id) {
  // Propaga erro: engolir fazia o cardápio aparecer vazio como se não
  // houvesse produtos (falha de rede virava dado).
  const { data } = await api.get("/products/" + id);
  return data;
}

export default {
  deleteProduct,
  getProducts,
};
