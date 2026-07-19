import React, { useState } from "react";
import {
  FiBox,
  FiEye,
  FiEyeOff,
  FiHardDrive,
  FiHome,
  FiPower,
  FiSettings,
  FiMenu,
  FiChevronLeft,
} from "react-icons/fi";
import { FaStore, FaStoreSlash } from "react-icons/fa";
import { useAuth } from "../context/AuthContext";
import ToggleSwitch from "../components/ToggleSwitch";
import { toast } from "react-toastify";
import Texts from "../constants/Texts";
import api from "../services/api";
import helper from "../helpers/helper";
import Logo from "./Logo";
import ThemeToggle from "./ThemeToggle";

const TopMenu = ({ toggleMenu, isOpen }) => {
  const { getUser, openEstablishment, refreshOpenawait } = useAuth();
  const user = getUser();

  const handlerBnt = async (res) => {
    try {
      await api.put("/establishments/status/handler/" + user.id);
      await refreshOpenawait();
    } catch (e) {
      console.log(e);
    }
    if (res && openEstablishment) toast.success(Texts.establishment_open);
    else toast.error("Seu estabelecimento foi fechado.");
  };

  return (
    <div
      className="sticky top-0 z-30 border-b"
      style={{
        background: "var(--bg-card, #FFFFFF)",
        borderColor: "var(--border-color, #E5E7EB)",
      }}
    >
      <div className="flex justify-between items-center px-6 py-3">
        <div className="flex items-center gap-4">
          <button
            onClick={toggleMenu}
            className="p-2 rounded-lg hover:bg-gray-100 transition-colors lg:hidden"
          >
            {isOpen ? (
              <FiChevronLeft className="h-5 w-5" />
            ) : (
              <FiMenu className="h-5 w-5" />
            )}
          </button>
          <div className="hidden lg:block">
            <h2
              className="text-lg font-bold"
              style={{ color: "var(--text-primary, #1A1A1A)" }}
            >
              {getUser()?.establishment_name}
            </h2>
          </div>
        </div>

        <div className="flex items-center gap-4">
          <div className="flex items-center gap-2 px-3 py-1.5 rounded-full bg-gray-50">
            <div
              className="w-2 h-2 rounded-full"
              style={{
                background: openEstablishment ? "#10B981" : "#9CA3AF",
              }}
            />
            <span
              className="text-xs font-medium"
              style={{ color: "var(--text-secondary, #666)" }}
            >
              {openEstablishment ? "Aberto" : "Fechado"}
            </span>
            <ToggleSwitch checked={openEstablishment} onChange={handlerBnt} />
          </div>
          <ThemeToggle />
        </div>
      </div>
    </div>
  );
};

const SideMenu = ({ isOpen }) => {
  const { logout } = useAuth();

  const MENUS = [
    {
      title: Texts.meus_pedidos,
      href: "/",
      icon: <FiHome className="h-5 w-5" />,
    },
    {
      title: "Cardápio",
      href: "/gestor-cardapio",
      icon: <FiBox className="h-5 w-5" />,
    },
    {
      title: "Delivery",
      href: "/taxas",
      icon: (
        <svg className="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <circle cx="12" cy="12" r="10" />
          <polyline points="12 6 12 12 16 14" />
        </svg>
      ),
    },
    {
      title: "Ajustes",
      href: "/perfil",
      icon: <FiSettings className="h-5 w-5" />,
    },
  ];

  return (
    <div
      className={`fixed top-0 left-0 h-full z-40 flex flex-col transition-all duration-300 ease-in-out ${
        isOpen ? "w-64" : "w-[72px]"
      }`}
      style={{
        background: "linear-gradient(180deg, #1A1A1A 0%, #111111 100%)",
        boxShadow: "4px 0 24px rgba(0,0,0,0.15)",
      }}
    >
      {/* Logo */}
      <div
        className={`flex items-center border-b border-white/10 ${
          isOpen ? "px-5 py-5" : "px-4 py-5 justify-center"
        }`}
      >
        {isOpen ? (
          <Logo size={36} variant="white" />
        ) : (
          <Logo size={32} variant="mark" />
        )}
      </div>

      {/* Navigation */}
      <nav className="flex-1 py-4 px-3 overflow-y-auto">
        <ul className="space-y-1">
          {MENUS.map((item, idx) => (
            <li key={idx}>
              <a
                href={item.href}
                className={`flex items-center gap-3 rounded-xl transition-all duration-200 group ${
                  isOpen ? "px-4 py-3" : "px-0 py-3 justify-center"
                } ${
                  window.location.hash === "#" + item.href
                    ? "text-white"
                    : "text-gray-400 hover:text-white"
                }`}
                style={
                  window.location.hash === "#" + item.href
                    ? {
                        background: "linear-gradient(135deg, #EA1D2C, #C41420)",
                      }
                    : {}
                }
                title={!isOpen ? item.title : undefined}
              >
                <span className="flex-shrink-0">{item.icon}</span>
                {isOpen && (
                  <span className="text-sm font-medium">{item.title}</span>
                )}
              </a>
            </li>
          ))}
        </ul>
      </nav>

      {/* Logout */}
      <div className="px-3 pb-4">
        <button
          onClick={logout}
          className={`flex items-center gap-3 w-full rounded-xl px-4 py-3 text-gray-400 hover:text-white hover:bg-white/10 transition-all duration-200 ${
            !isOpen ? "justify-center px-0" : ""
          }`}
          title={!isOpen ? "Sair" : undefined}
        >
          <FiPower className="h-5 w-5 flex-shrink-0" />
          {isOpen && <span className="text-sm font-medium">Sair</span>}
        </button>
      </div>
    </div>
  );
};

const MenuLayout = ({ children }) => {
  const tagitem = "ISOPEN";
  const getItem = localStorage.getItem(tagitem);
  const [isOpen, setIsOpen] = useState(
    getItem ? getItem === "true" : true
  );

  const toggleMenu = () => {
    const res = !isOpen;
    setIsOpen(res);
    localStorage.setItem(tagitem, JSON.stringify(res));
  };

  return (
    <div className="flex min-h-screen" style={{ background: "var(--bg-secondary, #F5F5F5)" }}>
      <SideMenu isOpen={isOpen} />

      <div
        className="flex-1 flex flex-col transition-all duration-300"
        style={{ marginLeft: isOpen ? "256px" : "72px" }}
      >
        <TopMenu toggleMenu={toggleMenu} isOpen={isOpen} />
        <main className="flex-1 p-6 overflow-x-hidden">{children}</main>
      </div>
    </div>
  );
};

export default MenuLayout;
