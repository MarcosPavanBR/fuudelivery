import React from "react";
import { FiPlus } from "react-icons/fi";

const AddButton = ({ onClick, text = "Novo" }) => {
  return (
    <button
      onClick={onClick}
      className="flex items-center justify-center gap-2 text-white font-semibold py-2.5 px-5 rounded-xl transition-all duration-200 hover:shadow-lg hover:scale-[1.02] active:scale-[0.98]"
      style={{
        background: "linear-gradient(135deg, #EA1D2C, #C41420)",
      }}
    >
      <FiPlus className="h-5 w-5" />
      {text}
    </button>
  );
};

export default AddButton;
