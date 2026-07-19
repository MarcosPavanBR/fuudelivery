import React from "react";
import helper from "../helpers/helper";
import CardapioEditModal from "../components/CardapioEditModal";

const CardapioList = ({
  items,
  onSave,
  editModalOpen,
  setEditModalOpen,
  selectedItem,
  setSelectedItem,
  onRefreshItens,
}) => {
  const handleEditClick = (item) => {
    setSelectedItem(item);
    setEditModalOpen(true);
  };

  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4 px-4 animate-fade-in">
      {items.map((item) => (
        <div
          key={item.ID}
          className="bg-white rounded-2xl border border-gray-100 shadow-card hover:shadow-card-hover transition-all duration-300 cursor-pointer overflow-hidden group"
          onClick={() => handleEditClick(item)}
        >
          {item.Image && (
            <div className="h-40 overflow-hidden">
              <img
                src={item?.Image}
                alt={item?.Name}
                className="w-full h-full object-cover group-hover:scale-105 transition-transform duration-500"
              />
            </div>
          )}
          <div className="p-4">
            <div className="flex items-start justify-between mb-2">
              <h3 className="font-bold text-gray-900 group-hover:text-red-600 transition-colors">
                {item?.Name}
              </h3>
              <span className="font-bold text-lg whitespace-nowrap" style={{ color: "#EA1D2C" }}>
                {helper.formatCurrency(item.Price)}
              </span>
            </div>
            {item?.Categories?.length > 0 && (
              <div className="flex flex-wrap gap-1.5 mt-2">
                {item.Categories.map((e, i) => (
                  <span
                    key={i}
                    className="text-xs font-medium px-2.5 py-1 rounded-full"
                    style={{ background: "#FEF2F2", color: "#EA1D2C" }}
                  >
                    {e.Name}
                  </span>
                ))}
              </div>
            )}
          </div>
        </div>
      ))}
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
