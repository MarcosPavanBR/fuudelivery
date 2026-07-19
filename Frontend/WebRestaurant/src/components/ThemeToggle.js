import React, { useState } from "react";
import { useAuth } from "../context/AuthContext";
import { FiMoon, FiSun } from "react-icons/fi";

const ThemeToggle = () => {
  const { theme, toggleTheme } = useAuth();

  return (
    <button
      onClick={toggleTheme}
      className="p-2.5 rounded-xl transition-all duration-200 hover:bg-gray-100"
      style={{ color: theme === "dark" ? "#F7A11E" : "#6B7280" }}
      title={theme === "light" ? "Modo escuro" : "Modo claro"}
    >
      {theme === "light" ? <FiMoon className="h-5 w-5" /> : <FiSun className="h-5 w-5" />}
    </button>
  );
};

export default ThemeToggle;
