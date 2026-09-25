import React from "react";
import helper from "../helpers/helper";
import CardapioEditModal from "../components/CardapioEditModal";
import { isAvailable } from "../services/products.model";
import { FiImage, FiPause, FiPlay } from "react-icons/fi";

const CardapioList = ({
  items,
  onSave,
  editModalOpen,
  setEditModalOpen,
  selectedItem,
  setSelectedItem,
  onRefreshItens,
  onToggleAvailability,
}) => {
  const handleEditClick = (item) => {
    setSelectedItem(item);
    setEditModalOpen(true);
  };

  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4 px-4 animate-fade-in">
      {items.map((item) => {
        const on = isAvailable(item);
        return (
        <div
          key={item.ID}
          className={`bg-white rounded-2xl border border-gray-100 shadow-card hover:shadow-card-hover transition-all duration-300 cursor-pointer overflow-hidden group ${on ? "" : "opacity-75"}`}
          onClick={() => handleEditClick(item)}
        >
          {/* Foto sempre com a mesma altura: sem foto, um aviso no lugar —
              os cards ficavam desalinhados e a falta de foto passava batido. */}
          <div className="relative h-40 overflow-hidden bg-gray-50">
            {item.Image ? (
              <img
                src={item?.Image}
                alt={item?.Name}
                className={`w-full h-full object-cover group-hover:scale-105 transition-transform duration-500 ${on ? "" : "grayscale"}`}
              />
            ) : (
              <div className="w-full h-full flex flex-col items-center justify-center gap-1 text-gray-300">
                <FiImage className="h-8 w-8" />
                <span className="text-xs text-gray-400">Sem foto — toque para adicionar</span>
              </div>
            )}
            {!on && (
              <span className="absolute top-2 left-2 rounded-full bg-gray-900/80 px-2.5 py-1 text-xs font-semibold text-white">
                Esgotado
              </span>
            )}
          </div>
          <div className="p-4">
            <div className="flex items-start justify-between mb-2">
              <h3 className="font-bold text-gray-900 group-hover:text-red-600 transition-colors">
                {item?.Name}
              </h3>
              <span className="font-bold text-lg whitespace-nowrap" style={{ color: "#DC2626" }}>
                {helper.formatCurrency(item.Price)}
              </span>
            </div>
            {item?.Description && (
              <p className="text-sm text-gray-500 line-clamp-2">{item.Description}</p>
            )}
            {item?.Categories?.length > 0 && (
              <div className="flex flex-wrap gap-1.5 mt-2">
                {item.Categories.map((e, i) => (
                  <span
                    key={i}
                    className="text-xs font-medium px-2.5 py-1 rounded-full"
                    style={{ background: "#FEF2F2", color: "#DC2626" }}
                  >
                    {e.Name}
                  </span>
                ))}
              </div>
            )}
            {onToggleAvailability && (
              <button
                type="button"
                onClick={(e) => {
                  e.stopPropagation();
                  onToggleAvailability(item);
                }}
                className={`mt-3 w-full inline-flex items-center justify-center gap-1.5 rounded-lg px-3 py-2 text-sm font-semibold transition-colors ${
                  on
                    ? "border border-gray-200 text-gray-700 hover:bg-gray-50"
                    : "bg-[#DC2626] text-white hover:bg-[#B91C1C]"
                }`}
              >
                {on ? <FiPause className="h-4 w-4" /> : <FiPlay className="h-4 w-4" />}
                {on ? "Pausar (esgotado)" : "Voltar a vender"}
              </button>
            )}
          </div>
        </div>
        );
      })}
      <CardapioEditModal
        isOpen={editModalOpen}
        onClose={() => setEditModalOpen(false)}
        item={selectedItem}
        onSave={onSave}
        onRefreshItens={onRefreshItens}
      />
    </div>
  );
};

export default CardapioList;
