import React from "react";
import { FiMoon, FiSun } from "react-icons/fi";
import { useAuth } from "../context/AuthContext";

const ThemeToggle = () => {
  const { theme, toggleTheme } = useAuth();

  return (
    <button
      onClick={toggleTheme}
      style={{
        background: "transparent",
        border: "none",
        cursor: "pointer",
        color: "#FFF",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        padding: 8,
        borderRadius: 8,
        transition: "background 0.2s",
      }}
      title={theme === "light" ? "Modo escuro" : "Modo claro"}
    >
      {theme === "light" ? <FiMoon size={20} /> : <FiSun size={20} />}
    </button>
  );
};

export default ThemeToggle;
