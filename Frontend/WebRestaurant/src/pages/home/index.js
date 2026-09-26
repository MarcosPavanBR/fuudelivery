import React, { useEffect, useRef, useState } from "react";
import { toast } from "react-toastify";
import Board from "../../components/Board";
import DashboardCharts from "../../components/DashboardCharts";
import MenuLayout from "../../components/Menu";
import { useAuth } from "../../context/AuthContext";
import Texts from "../../constants/Texts";
import ordersModels from "../../services/orders.models";
import { canMove, displayColumn, newPendingIds } from "../../components/orderCard";
import { alertNewOrders, isSoundOn, setSoundOn, unlockAudio } from "../../services/newOrderAlert";
import { FiBell, FiBellOff } from "react-icons/fi";
import { establishmentIdOf } from "../../helpers/session";

// Precisa espelhar exatamente as transições que o backend aceita
// (validTransitions em Backend/orders_api/app/handlers/orders.go) — sem a
// coluna PREPARING, arrastar de "Aceito" pra "Pronto" pedia APPROVED->DONE
// direto, que o backend sempre rejeitava: o restaurante ficava sem
// nenhuma forma de marcar um pedido aceito como pronto.
const columns = [
  // Âmbar, não vermelho: vermelho lê como erro; aqui é "precisa de atenção".
  { id: "AWAIT_APPROVE", title: "Em análise", background: "linear-gradient(135deg, #EA580C, #F59E0B)" },
  { id: "APPROVED", title: "Aceito", background: "linear-gradient(135deg, #2563EB, #3B82F6)" },
  { id: "PREPARING", title: "Em preparo", background: "linear-gradient(135deg, #0EA5E9, #38BDF8)" },
  { id: "DONE", title: "Pronto", background: "linear-gradient(135deg, #10B981, #34D399)" },
  // O entregador leva o pedido a IN_ROUTE_DELIVERY ao sair com ele; sem esta
  // coluna o card sumia do quadro até a entrega.
  { id: "IN_ROUTE_DELIVERY", title: "A caminho", background: "linear-gradient(135deg, #7C3AED, #A78BFA)" },
];

const Home = () => {
  const [tasks, setTasks] = useState([]);
  const [loadError, setLoadError] = useState(false);
  const [now, setNow] = useState(Date.now());
  const [soundOn, setSoundOnState] = useState(isSoundOn());
  // Ids já vistos: só pedido NOVO em análise dispara o alerta (null = ainda
  // não carregou, a primeira carga não apita).
  const seenIds = useRef(null);
  const { getUser, socketMessage, fmode } = useAuth();
  const user = getUser();

  async function init(verifyFmode) {
    if (!user) return;
    try {
      if (verifyFmode && !fmode) return;
      // Mesma chave do DashboardCharts: o establishment_id da sessão —
      // antes o Kanban usava user.id e divergia do dashboard.
      const establishmentId = establishmentIdOf(getUser());
      const orders = await ordersModels.getOrders(establishmentId);
      alertNewOrders(newPendingIds(seenIds.current, orders).length);
      seenIds.current = new Set(orders.map((o) => o.id));
      setTasks(orders);
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

  // "há X min" dos cards anda sozinho.
  useEffect(() => {
    const t = setInterval(() => setNow(Date.now()), 30000);
    return () => clearInterval(t);
  }, []);

  // O navegador só toca som depois de uma interação com a página.
  useEffect(() => {
    const unlock = () => unlockAudio();
    window.addEventListener("pointerdown", unlock);
    window.addEventListener("keydown", unlock);
    return () => {
      window.removeEventListener("pointerdown", unlock);
      window.removeEventListener("keydown", unlock);
    };
  }, []);

  const toggleSound = () => {
    unlockAudio();
    setSoundOn(!soundOn);
    setSoundOnState(!soundOn);
  };

  const DONE_MESSAGES = {
    DENIED: "Pedido recusado.",
    CANCELLED: "Pedido cancelado.",
    FINISHED: "Pedido entregue.",
  };

  // Muda o status (botão do card ou arrastar). Otimista, com volta se o
  // servidor recusar. Recusado/cancelado/entregue saem do quadro.
  const changeStatus = async (orderId, to) => {
    const previous = tasks;
    setTasks(
      tasks
        .map((e) => (e.id === orderId ? { ...e, column: displayColumn(to), data: { ...e.data, status: to } } : e))
        .filter((e) => !(e.id === orderId && DONE_MESSAGES[to]))
    );
    const ok = await ordersModels.alterStatus(to, orderId);
    if (!ok) {
      setTasks(previous);
      toast.error("Não foi possível atualizar o pedido. Tente novamente.");
      return;
    }
    if (DONE_MESSAGES[to]) toast.info(DONE_MESSAGES[to]);
  };

  const onDragEnd = async (result) => {
    const { destination, source, draggableId } = result;
    if (!destination) return;
    // Reordenação dentro da mesma coluna não muda status — não chama a API.
    if (destination.droppableId === source.droppableId) return;

    const task = tasks.find((e) => e.id === draggableId);
    const from = task?.data?.status;
    if (!canMove(from, destination.droppableId)) {
      toast.warn("Esse pedido não pode ir direto para essa coluna.");
      return;
    }
    await changeStatus(draggableId, destination.droppableId);
  };

  return (
    <MenuLayout>
      <div className="flex items-center justify-between mb-4">
        <div>
          <p className="text-xs font-semibold uppercase tracking-wider text-gray-500 mb-1">Pedidos</p>
          <h2 className="text-xl font-bold text-gray-900">{Texts.meus_pedidos}</h2>
        </div>
        <button
          type="button"
          onClick={toggleSound}
          className={`inline-flex items-center gap-2 rounded-lg border px-3 py-2 text-sm font-medium transition-colors ${
            soundOn ? "border-gray-200 bg-white text-gray-700 hover:bg-gray-50" : "border-amber-200 bg-amber-50 text-amber-800"
          }`}
          title="Som de pedido novo"
        >
          {soundOn ? <FiBell className="h-4 w-4" /> : <FiBellOff className="h-4 w-4" />}
          {soundOn ? "Som ligado" : "Som desligado"}
        </button>
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
      <Board tasks={tasks} columns={columns} onDragEnd={onDragEnd} onAction={changeStatus} now={now} />
      {/* Os pedidos são o trabalho principal da tela: vêm primeiro; os
          números da semana ficam abaixo. */}
      <div className="mt-8">
        <DashboardCharts establishmentId={establishmentIdOf(user)} />
      </div>
    </MenuLayout>
  );
};

export default Home;
