import Strings from "../constants/Strings";
import api from "./api";

async function getCategories(id) {
  try {
    const { data } = await api.get("/categories/" + id);

    return data;
  } catch (e) {
    console.error(e);
    return [];
  }
}

async function createCategory(items, editItem, establishmentId) {
  const body = {
    ...editItem,
    ID: null,
    EstablishmentId: establishmentId,
  };
  const { data } = await api.post("categories/create", body);
  return [
    { ID: data.Id, ...data },
    ...items.filter((e) => e.ID !== Strings.id_default),
  ];
}

async function updateCategory(items, editItem, establishmentId) {
  const body = {
    ...editItem,
    EstablishmentId: establishmentId,
  };
  const { data } = await api.put(
    "/categories/" + editItem.ID,
    body
  );
  return items.map((e) => (e.ID === editItem.ID ? data : e));
}

async function handlerVinculoProdutoCategoria(productID, categoryId) {
  await api.post("/categories/product", {
    productID: parseInt(productID),
    categoryId: parseInt(categoryId),
  });
  return true;
}

async function deleteCategory(items, id) {
  await api.delete("/categories/" + id);
  return [...items.filter((e) => e.ID !== id)];
}

export default {
  getCategories,
  deleteCategory,
  updateCategory,
  handlerVinculoProdutoCategoria,
  createCategory,
};
