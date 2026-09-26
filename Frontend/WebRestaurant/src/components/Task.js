import React, { useState } from "react";
import { Draggable } from "@hello-pangea/dnd";
import helper from "../helpers/helper";
import Texts from "../constants/Texts";
import {
  actionsFor,
  elapsedLabel,
  isLate,
  itemName,
  itemNote,
  formatPhone,
  orderTotal,
  paymentType,
  selectedAdditionals,
  shortOrderId,
} from "./orderCard";
import { FiChevronDown, FiChevronUp, FiClock, FiPhone, FiCalendar } from "react-icons/fi";

const formatTime = (iso) => {
  const t = Date.parse(iso);
  if (Number.isNaN(t)) return "";
  return new Date(t).toLocaleTimeString("pt-BR", { hour: "2-digit", minute: "2-digit" });
};

const Task = ({ task, index, onAction, now }) => {
  const [showItems, setShowItems] = useState(task.data?.status === "AWAIT_APPROVE");
  const [confirming, setConfirming] = useState(null);
  const [busy, setBusy] = useState(false);

  const data = task.data || {};
  const payment = paymentType(data);
  const paymentLabel = Texts[payment] ?? (payment || "—");
  const late = isLate(data, now);
  const actions = actionsFor(data);
  const cart = data.cart || [];
  const itemsCount = cart.reduce((s, c) => s + (c.quantity || 0), 0);
  const courier = data.deliveryman && Number(data.deliveryman.id) > 0 ? data.deliveryman : null;

  const run = async (action) => {
    if (action.confirm && confirming !== action.to) {
      setConfirming(action.to);
      return;
    }
    setConfirming(null);
    setBusy(true);
    await onAction?.(task.id, action.to);
    setBusy(false);
  };

  return (
    <Draggable id={task.id} draggableId={task.id} index={index} type="TASK">
      {(provided, snapshot) => (
        <div
          ref={provided.innerRef}
          {...provided.draggableProps}
          {...provided.dragHandleProps}
          className={`bg-white rounded-xl p-4 border transition-all duration-200 ${
            late ? "border-red-300 ring-2 ring-red-100" : "border-gray-100"
          } ${snapshot.isDragging ? "shadow-modal scale-[1.02] rotate-1" : "shadow-card hover:shadow-card-hover"}`}
        >
          {/* Número, hora e tempo decorrido */}
          <div className="flex items-center justify-between mb-2">
            <span className="text-xs font-bold tracking-wider text-gray-400">{shortOrderId(task.id)}</span>
            <span
              className={`inline-flex items-center gap-1 text-xs font-medium ${late ? "text-red-600" : "text-gray-500"}`}
              title={data.created_at ? new Date(data.created_at).toLocaleString("pt-BR") : ""}
            >
              <FiClock className="h-3 w-3" />
              {formatTime(data.created_at)} · {elapsedLabel(data.created_at, now)}
            </span>
          </div>

          {data.status === "SCHEDULED" && data.scheduled_at && (
            <div className="mb-2 inline-flex items-center gap-1 rounded-full bg-indigo-50 px-2.5 py-1 text-xs font-semibold text-indigo-700">
              <FiCalendar className="h-3 w-3" /> Agendado para {new Date(data.scheduled_at).toLocaleString("pt-BR", { day: "2-digit", month: "2-digit", hour: "2-digit", minute: "2-digit" })}
            </div>
          )}

          {/* Cliente */}
          <div className="mb-3">
            <p className="font-bold text-sm text-gray-900">{data.user?.nome || "Cliente"}</p>
            {data.user?.phone && (
              <p className="flex items-center gap-1 text-xs text-gray-500">
                <FiPhone className="h-3 w-3" />{" "}
                <a href={`tel:${String(data.user.phone).replace(/[^\d+]/g, "")}`} className="hover:underline" onClick={(e) => e.stopPropagation()}>
                  {formatPhone(data.user.phone)}
                </a>
              </p>
            )}
          </div>

          {/* Pagamento e total */}
          <div className="flex items-center justify-between mb-3">
            <span className="inline-flex items-center px-2.5 py-1 rounded-full text-xs font-medium bg-gray-100 text-gray-700">
              {paymentLabel}
            </span>
            <span className="text-lg font-bold text-gray-900">{helper.formatCurrency(orderTotal(data))}</span>
          </div>

          {/* Código de retirada: o entregador informa este código ao retirar. */}
          {data.pickup_code && (
            <div className="mb-3 flex flex-wrap items-center justify-between gap-1 p-2.5 rounded-lg bg-yellow-50 border border-yellow-100">
              <span className="text-xs text-yellow-800 font-medium">Código de retirada</span>
              <span className="font-mono font-bold tracking-widest text-yellow-900">{data.pickup_code}</span>
            </div>
          )}

          {courier && (
            <div className="mb-3 p-2.5 rounded-lg bg-blue-50 border border-blue-100">
              <p className="text-xs text-blue-700">{Texts.entregador}</p>
              <p className="text-sm font-semibold text-blue-900 truncate">{courier.name || "—"}</p>
              {courier.status && (
                <span className="mt-1 inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-800">
                  {Texts[courier.status] || courier.status}
                </span>
              )}
            </div>
          )}

          {/* Itens */}
          <button
            type="button"
            onClick={() => setShowItems(!showItems)}
            className="w-full flex items-center justify-between p-2.5 rounded-lg bg-gray-50 hover:bg-gray-100 transition-colors text-sm"
          >
            <span className="font-medium text-gray-700">
              Itens do pedido ({itemsCount}){cart.some(itemNote) ? " · com observação" : ""}
            </span>
            {showItems ? <FiChevronUp className="h-4 w-4 text-gray-500" /> : <FiChevronDown className="h-4 w-4 text-gray-500" />}
          </button>
          {showItems && (
            <div className="mt-2 space-y-1.5 animate-slide-up">
              {cart.map((item, idx) => (
                <div key={idx} className="p-2 rounded-lg bg-gray-50">
                  <p className="text-sm">
                    <span className="font-bold">{item.quantity}x</span>{" "}
                    <span className="font-medium text-gray-900">{itemName(item)}</span>
                  </p>
                  {itemNote(item) && (
                    <p className="mt-1 rounded-md bg-amber-50 px-2 py-1 text-xs font-medium text-amber-900">
                      Obs.: {itemNote(item)}
                    </p>
                  )}
                  {selectedAdditionals(item).length > 0 && (
                    <div className="mt-1 flex flex-wrap gap-1">
                      {selectedAdditionals(item).map((a, aidx) => (
                        <span key={aidx} className="text-xs px-2 py-0.5 rounded-full bg-white border border-gray-200 text-gray-600">
                          + {a.name}
                        </span>
                      ))}
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}

          {/* Ações: botões grandes, para tablet na cozinha (arrastar continua valendo). */}
          {actions.length > 0 && (
            <div className="mt-3 flex flex-wrap gap-2">
              {actions.map((a) => {
                const isConfirming = confirming === a.to;
                // flex-wrap + largura mínima: em coluna estreita o botão
                // secundário desce de linha em vez de vazar para fora do card.
                const base = "flex-1 min-w-[6.5rem] whitespace-nowrap rounded-lg px-3 py-2.5 text-sm font-semibold transition-colors disabled:opacity-50";
                const style =
                  a.kind === "danger"
                    ? isConfirming
                      ? "bg-red-600 text-white hover:bg-red-700"
                      : "bg-white border border-red-200 text-red-600 hover:bg-red-50"
                    : "bg-[#DC2626] text-white hover:bg-[#B91C1C]";
                return (
                  <button key={a.to} type="button" disabled={busy} onClick={() => run(a)} className={`${base} ${style}`}>
                    {isConfirming ? `Confirmar: ${a.label.toLowerCase()}` : a.label}
                  </button>
                );
              })}
            </div>
          )}
          {confirming && (
            <p className="mt-2 text-xs text-gray-500">
              {actions.find((a) => a.to === confirming)?.confirm}{" "}
              <button type="button" className="font-semibold text-gray-700 underline" onClick={() => setConfirming(null)}>
                Voltar
              </button>
            </p>
          )}
        </div>
      )}
    </Draggable>
  );
};

export default Task;
