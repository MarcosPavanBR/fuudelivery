import React, { useEffect, useState } from "react";
import { toast } from "react-toastify";
import Board from "../../components/Board";
import DashboardCharts from "../../components/DashboardCharts";
import MenuLayout from "../../components/Menu";
import { useAuth } from "../../context/AuthContext";
import Texts from "../../constants/Texts";
import ordersModels from "../../services/orders.models";

const columns = [
  { id: "AWAIT_APPROVE", title: "Em análise", background: "linear-gradient(135deg, #DC2626, #FF6B35)" },
  { id: "APPROVED", title: "Em produção", background: "linear-gradient(135deg, #F59E0B, #FBBF24)" },
  { id: "DONE", title: "Pronto p/ entrega", background: "linear-gradient(135deg, #10B981, #34D399)" },
];

const Home = () => {
  const [tasks, setTasks] = useState([]);
  const [loadError, setLoadError] = useState(false);
  const { getUser, socketMessage, fmode } = useAuth();
  const user = getUser();

  async function init(verifyFmode) {
    if (!user) return;
    try {
      if (verifyFmode && !fmode) return;
      // Mesma chave do DashboardCharts: establishment.id quando existir —
      // antes o Kanban usava user.id e divergia do dashboard.
      const establishmentId = getUser()?.establishment?.id || getUser().id;
      setTasks(await ordersModels.getOrders(establishmentId));
      setLoadError(false);
    } catch (e) {
      setLoadError(true);
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
    // Reordenação dentro da mesma coluna não muda status — não chama a API.
    if (destination.droppableId === source.droppableId) return;

    const previous = tasks;
    // Update otimista + rollback: se a API falhar, o card volta para a
    // coluna original (antes ficava preso na coluna errada em silêncio).
    setTasks(
      tasks.map((e) => {
        if (e.id === draggableId) return { ...e, column: destination.droppableId };
        return e;
      })
    );
    try {
      await ordersModels.alterStatus(destination.droppableId, draggableId);
    } catch (e) {
      setTasks(previous);
      toast.error("Não foi possível mover o pedido. Tente novamente.");
    }
  };

  return (
    <MenuLayout>
      <DashboardCharts establishmentId={user?.establishment?.id || user?.id} />
      <div className="flex items-center justify-between mb-4">
        <div>
          <p className="text-xs font-semibold uppercase tracking-wider text-gray-500 mb-1">Pedidos</p>
          <h2 className="text-xl font-bold text-gray-900">{Texts.meus_pedidos}</h2>
        </div>
      </div>
      {loadError && (
        <div className="mb-4 flex items-center justify-between rounded-lg border border-red-200 bg-red-50 px-4 py-3">
          <span className="text-sm text-red-700">
            Não foi possível carregar os pedidos. Verifique sua conexão.
          </span>
          <button
            onClick={() => init()}
            className="rounded-md bg-[#DC2626] px-3 py-1.5 text-sm font-semibold text-white hover:bg-[#B91C1C]"
          >
            Tentar novamente
          </button>
        </div>
      )}
      <Board tasks={tasks} columns={columns} onDragEnd={onDragEnd} />
    </MenuLayout>
  );
};

export default Home;
