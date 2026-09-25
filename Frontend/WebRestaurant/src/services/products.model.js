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

// Pausa (esgotado) ou reativa o item. Propaga erro para a tela desfazer.
async function setAvailability(id, available) {
  const { data } = await api.put(`/products/${id}/availability`, { available });
  return data;
}

// Produto sem o campo (servidor antigo) conta como à venda.
export const isAvailable = (product) => product?.Available !== false;

export default {
  deleteProduct,
  getProducts,
  setAvailability,
};
