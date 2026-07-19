import api from "./api";

async function deleteProduct(id) {
  try {
    const { data } = await api.delete("/products/delete/" + id);

    return true;
  } catch (e) {
    console.log(e);
    return false;
  }
}

async function getProducts(id) {
  try {
    const { data } = await api.get("/products/" + id);
    return data;
  } catch (e) {
    return [];
  }
}

export default {
  deleteProduct,
  getProducts,
};
