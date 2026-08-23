import api from "./api";

async function getOrders(id) {
  try {
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
  } catch (e) {
    return [];
  }
}

async function alterStatus(droppableId, draggableId) {
  try {
    const { data } = await api.put("/orders/status", {
      id: draggableId,
      status: droppableId,
    });
    return true;
  } catch (e) {
    console.error(e);
    return false;
  }
}

export default {
  getOrders,
  alterStatus,
};
