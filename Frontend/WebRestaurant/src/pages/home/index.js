import React, { useEffect, useState } from "react";
import Board from "../../components/Board";
import DashboardCharts from "../../components/DashboardCharts";
import MenuLayout from "../../components/Menu";
import { useAuth } from "../../context/AuthContext";
import Texts from "../../constants/Texts";
import ordersModels from "../../services/orders.models";

const columns = [
  { id: "AWAIT_APPROVE", title: "Em análise", background: "linear-gradient(135deg, #EA1D2C, #FF6B35)" },
  { id: "APPROVED", title: "Em produção", background: "linear-gradient(135deg, #F7A11E, #FBBF24)" },
  { id: "DONE", title: "Pronto p/ entrega", background: "linear-gradient(135deg, #10B981, #34D399)" },
];

const Home = () => {
  const [tasks, setTasks] = useState([]);
  const { getUser, socketMessage, fmode } = useAuth();
  const user = getUser();

  async function init(verifyFmode) {
    if (!user) return;
    try {
      if (verifyFmode && !fmode) return;
      setTasks(await ordersModels.getOrders(getUser().id));
    } catch (e) {
      console.log(e);
    }
  }

  useEffect(() => {
    init();
  }, [socketMessage]);

  useEffect(() => {
    let intervalId;
    if (fmode) {
      intervalId = setInterval(() => init(true), 15000);
    }
    return () => clearInterval(intervalId);
  }, [fmode]);

  const onDragEnd = async (result) => {
    const { destination, source, draggableId } = result;
    if (!destination) return;
    setTasks(
      tasks.map((e) => {
        if (e.id === draggableId) return { ...e, column: destination.droppableId };
        return e;
      })
    );
    await ordersModels.alterStatus(destination.droppableId, draggableId);
  };

  return (
    <MenuLayout>
      <DashboardCharts establishmentId={user?.id} />
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-lg font-bold text-gray-900">{Texts.meus_pedidos}</h2>
      </div>
      <Board tasks={tasks} columns={columns} onDragEnd={onDragEnd} />
    </MenuLayout>
  );
};

export default Home;
