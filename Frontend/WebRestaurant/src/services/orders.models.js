import api from "./api";

async function getOrders(id) {
  // Propaga o erro: engolir aqui fazia o Kanban mostrar "Nenhum pedido"
  // quando a API estava fora (falha silenciosa virava dado).
  const { data } = await api.get("/orders/" + id);

  return data
    .filter((e) => {
      // Exclui pedidos finalizados pelo entregador (já saiu do Kanban)
      if (e.deliveryman && e.deliveryman.status === "FINISHED") {
        return false;
      }
      return true;
    })
    .map((e) => {
      return {
        id: e._id,
        column: e.status,
        data: {
          ...e,
        },
      };
    });
}

async function alterStatus(droppableId, draggableId) {
  try {
    const { data } = await api.put("/orders/status", {
      id: draggableId,
      status: droppableId,
    });
    return true;
  } catch (e) {
    return false;
  }
}

export default {
  getOrders,
  alterStatus,
};
