import React, { useState } from "react";
import { Draggable } from "react-beautiful-dnd";
import helper from "../helpers/helper";
import Texts from "../constants/Texts";
import { FiChevronDown, FiChevronUp, FiUser, FiPhone } from "react-icons/fi";

const Task = ({ task, index }) => {
  const [showItems, setShowItems] = useState(false);

  const calculateFinalPrice = ({ item, quantity, additionals = [] }) => {
    const additionalPricesSum = additionals?.reduce((sum, additionalId) => {
      const additional = item.additional.find((a) => a.ID === additionalId);
      return sum + (additional?.price || 0);
    }, 0);
    return quantity * (item.price + (additionalPricesSum || 0));
  };

  const subTotal =
    task.data.cart
      .map((e) => calculateFinalPrice(e))
      .reduce((e, f) => e + f, 0) || 0;

  const paymentLabel =
    Texts[task.data.paymentmethod.type] ?? task.data.paymentmethod.type;

  return (
    <Draggable id={task.id} draggableId={task.id} index={index} type="TASK">
      {(provided, snapshot) => (
        <div
          ref={provided.innerRef}
          {...provided.draggableProps}
          {...provided.dragHandleProps}
          className={`bg-white rounded-xl p-4 border border-gray-100 transition-all duration-200 ${
            snapshot.isDragging
              ? "shadow-modal scale-[1.02] rotate-1"
              : "shadow-card hover:shadow-card-hover"
          }`}
        >
          {/* Header */}
          <div className="flex items-start justify-between mb-3">
            <div className="flex items-center gap-2">
              <div
                className="w-8 h-8 rounded-lg flex items-center justify-center"
                style={{ background: "#FEF2F2", color: "#EA1D2C" }}
              >
                <FiUser className="h-4 w-4" />
              </div>
              <div>
                <p className="font-bold text-sm text-gray-900">
                  {task.data.user.nome}
                </p>
                <div className="flex items-center gap-1 text-gray-500">
                  <FiPhone className="h-3 w-3" />
                  <span className="text-xs">{task.data.user.phone}</span>
                </div>
              </div>
            </div>
          </div>

          {/* Payment & Total */}
          <div className="flex items-center justify-between mb-3">
            <span className="inline-flex items-center px-2.5 py-1 rounded-full text-xs font-medium bg-gray-100 text-gray-700">
              {paymentLabel}
            </span>
            <span className="text-lg font-bold" style={{ color: "#EA1D2C" }}>
              {helper.formatCurrency(subTotal)}
            </span>
          </div>

          {/* Delivery Code */}
          {task.data?.deliveryman?.id != 0 && task.data?.deliveryman && (
            <div className="mb-3 p-2.5 rounded-lg bg-yellow-50 border border-yellow-100">
              <div className="flex items-center justify-between">
                <span className="text-xs text-yellow-700 font-medium">
                  Código
                </span>
                <span className="font-bold text-yellow-800">
                  {helper.genCode(task.data._id, task.data.establishment.id)}
                </span>
              </div>
              <div className="flex items-center justify-between mt-1">
                <span className="text-xs text-yellow-700 font-medium">
                  Cliente
                </span>
                <span className="font-bold text-yellow-800">
                  {helper.genCode(task.data._id)}
                </span>
              </div>
            </div>
          )}

          {/* Deliveryman Info */}
          {task.data?.deliveryman && task.data?.deliveryman?.id != 0 && (
            <div className="mb-3 p-2.5 rounded-lg bg-blue-50 border border-blue-100">
              <div className="flex items-center justify-between">
                <span className="text-xs text-blue-700">
                  {Texts.entregador}
                </span>
                <span className="text-sm font-semibold text-blue-900">
                  {task.data?.deliveryman?.name}
                </span>
              </div>
              {task.data?.deliveryman?.phone && (
                <div className="flex items-center justify-between mt-1">
                  <span className="text-xs text-blue-700">{Texts.phone}</span>
                  <span className="text-sm text-blue-900">
                    {task.data?.deliveryman?.phone}
                  </span>
                </div>
              )}
              <div className="flex items-center justify-between mt-1">
                <span className="text-xs text-blue-700">{Texts.status}</span>
                <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-800">
                  {Texts[task.data?.deliveryman?.status]}
                </span>
              </div>
            </div>
          )}

          {/* Cart Items Toggle */}
          <button
            onClick={() => setShowItems(!showItems)}
            className="w-full flex items-center justify-between p-2.5 rounded-lg bg-gray-50 hover:bg-gray-100 transition-colors text-sm"
          >
            <span className="font-medium text-gray-700">
              {Texts.itens_carrinho}
            </span>
            {showItems ? (
              <FiChevronUp className="h-4 w-4 text-gray-500" />
            ) : (
              <FiChevronDown className="h-4 w-4 text-gray-500" />
            )}
          </button>

          {showItems && (
            <div className="mt-2 space-y-2 animate-slide-up">
              {task.data.cart.map((item, idx) => (
                <div
                  key={idx}
                  className={`p-2.5 rounded-lg bg-gray-50 ${
                    idx !== task.data.cart.length - 1 ? "border-b border-gray-100" : ""
                  }`}
                >
                  <div className="flex items-center justify-between">
                    <span className="text-sm">
                      <span className="font-bold">{item.quantity}x</span>{" "}
                      <span className="font-medium text-gray-900">
                        {item.item.name}
                      </span>
                    </span>
                  </div>
                  {item.item.additional?.length > 0 && (
                    <div className="mt-1 flex flex-wrap gap-1">
                      {item.item.additional.map((additional, aidx) => (
                        <span
                          key={aidx}
                          className="text-xs px-2 py-0.5 rounded-full bg-gray-200 text-gray-600"
                        >
                          {additional.name}
                        </span>
                      ))}
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </Draggable>
  );
};

export default Task;
