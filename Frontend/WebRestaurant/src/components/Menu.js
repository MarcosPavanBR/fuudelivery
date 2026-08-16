import React, { useState, useEffect } from "react";
import {
  FiBox,
  FiHome,
  FiPower,
  FiSettings,
  FiMenu,
  FiChevronLeft,
  FiX,
  FiCreditCard,
  FiBarChart2,
} from "react-icons/fi";
import { Link, useLocation } from "react-router-dom";
import { useAuth } from "../context/AuthContext";
import ToggleSwitch from "../components/ToggleSwitch";
import { toast } from "react-toastify";
import Texts from "../constants/Texts";
import api from "../services/api";
import Logo from "./Logo";
import ThemeToggle from "./ThemeToggle";

const TopMenu = ({ toggleMenu, isOpen }) => {
  const { getUser, openEstablishment, refreshOpen } = useAuth();
  const user = getUser();

  const handlerBnt = async (res) => {
    try {
      await api.put("/establishments/status/handler/" + (user?.establishment_id || user?.id));
      await refreshOpen();
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
      <div className="flex justify-between items-center px-4 sm:px-6 py-3">
        <div className="flex items-center gap-4">
          <button
            onClick={toggleMenu}
            className="p-2 rounded-lg hover:bg-gray-100 transition-colors"
          >
            {isOpen ? (
              <FiChevronLeft className="h-5 w-5" />
            ) : (
              <FiMenu className="h-5 w-5" />
            )}
          </button>
          <div className="hidden lg:block min-w-0">
            <h2
              className="text-lg font-bold truncate max-w-[280px]"
              style={{ color: "var(--text-primary, #1A1A1A)" }}
            >
              {getUser()?.establishment_name}
            </h2>
          </div>
        </div>

        <div className="flex items-center gap-3 sm:gap-4">
          <div className="flex items-center gap-2 px-3 py-1.5 rounded-full bg-gray-50">
            <div
              className="w-2 h-2 rounded-full"
              style={{
                background: openEstablishment ? "#10B981" : "#9CA3AF",
              }}
            />
            <span
              className="text-xs font-medium hidden sm:inline"
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

const SideMenu = ({ isOpen, isMobile, onClose }) => {
  const { logout } = useAuth();
  const location = useLocation();

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
    {
      title: "Minha Carteira",
      href: "/carteira",
      icon: <FiCreditCard className="h-5 w-5" />,
    },
    {
      title: "Relatórios",
      href: "/relatorios",
      icon: <FiBarChart2 className="h-5 w-5" />,
    },
  ];

  const handleNavClick = () => {
    if (isMobile && onClose) onClose();
  };

  if (isMobile) {
    return (
      <>
        {isOpen && (
          <div
            className="fixed inset-0 bg-black/50 z-40 lg:hidden"
            onClick={onClose}
          />
        )}
        <div
          className={`fixed top-0 left-0 h-full z-50 flex flex-col transition-transform duration-300 ease-in-out lg:hidden ${
            isOpen ? "translate-x-0" : "-translate-x-full"
          }`}
          style={{
            width: "260px",
            background: "#1A1A1A",
            boxShadow: "4px 0 24px rgba(0,0,0,0.15)",
          }}
        >
          <div className="flex items-center justify-between px-5 py-5 border-b border-white/10">
            <Logo size={36} variant="white" />
            <button
              onClick={onClose}
              className="p-1.5 rounded-lg text-gray-400 hover:text-white hover:bg-white/10 transition-colors"
            >
              <FiX className="h-5 w-5" />
            </button>
          </div>

          <nav className="flex-1 py-4 px-3 overflow-y-auto">
            <ul className="space-y-1">
              {MENUS.map((item, idx) => (
                <li key={idx}>
                  <Link
                    to={item.href}
                    onClick={handleNavClick}
                    className={`flex items-center gap-2 px-4 py-2 rounded-r-lg transition-all duration-150 ${
                      location.pathname === item.href
                        ? "text-[#EA1D2C] font-semibold border-l-[3px] border-[#EA1D2C] bg-[rgba(234,29,44,0.10)]"
                        : "text-[#9CA3AF] hover:bg-[rgba(255,255,255,0.03)]"
                    }`}
                  >
                    <span className="flex-shrink-0">{item.icon}</span>
                    <span className="text-sm font-medium">{item.title}</span>
                  </Link>
                </li>
              ))}
            </ul>
          </nav>

          <div className="px-3 pb-4">
            <button
          onClick={logout}
          className="flex items-center gap-2 w-full rounded-r-lg px-4 py-2 text-[#9CA3AF] hover:text-white hover:bg-[rgba(255,255,255,0.03)] transition-all duration-150"
            >
              <FiPower className="h-5 w-5 flex-shrink-0" />
              <span className="text-sm font-medium">Sair</span>
            </button>
          </div>
        </div>
      </>
    );
  }

  return (
    <div
      className={`fixed top-0 left-0 h-full z-40 hidden lg:flex flex-col transition-all duration-300 ease-in-out ${
        isOpen ? "w-64" : "w-[72px]"
      }`}
      style={{
        background: "#1A1A1A",
        boxShadow: "4px 0 24px rgba(0,0,0,0.15)",
      }}
    >
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

      <nav className="flex-1 py-4 px-3 overflow-y-auto">
        <ul className="space-y-1">
          {MENUS.map((item, idx) => (
            <li key={idx}>
              <Link
                to={item.href}
                className={`flex items-center gap-2 rounded-r-lg transition-all duration-150 group ${
                  isOpen ? "px-4 py-2" : "px-0 py-2 justify-center"
                } ${
                  location.pathname === item.href
                    ? "text-[#EA1D2C] font-semibold border-l-[3px] border-[#EA1D2C] bg-[rgba(234,29,44,0.10)]"
                    : "text-[#9CA3AF] hover:bg-[rgba(255,255,255,0.03)]"
                }`}
                title={!isOpen ? item.title : undefined}
              >
                <span className="flex-shrink-0">{item.icon}</span>
                {isOpen && (
                  <span className="text-sm font-medium">{item.title}</span>
                )}
              </Link>
            </li>
          ))}
        </ul>
      </nav>

      <div className="px-3 pb-4">
        <button
          onClick={logout}
          className={`flex items-center gap-2 w-full rounded-r-lg px-4 py-2 text-[#9CA3AF] hover:text-white hover:bg-[rgba(255,255,255,0.03)] transition-all duration-150 ${
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
  const [isMobile, setIsMobile] = useState(false);
  const [mobileOpen, setMobileOpen] = useState(false);

  useEffect(() => {
    const checkMobile = () => {
      const mobile = window.innerWidth < 1024;
      setIsMobile(mobile);
      if (!mobile) setMobileOpen(false);
    };
    checkMobile();
    window.addEventListener("resize", checkMobile);
    return () => window.removeEventListener("resize", checkMobile);
  }, []);

  const toggleMenu = () => {
    if (isMobile) {
      setMobileOpen(!mobileOpen);
    } else {
      const res = !isOpen;
      setIsOpen(res);
      localStorage.setItem(tagitem, JSON.stringify(res));
    }
  };

  const closeMobile = () => setMobileOpen(false);

  const sidebarVisible = isMobile ? mobileOpen : isOpen;

  return (
    <div className="min-h-screen" style={{ background: "var(--bg-secondary, #F5F5F5)" }}>
      <SideMenu
        isOpen={isMobile ? mobileOpen : isOpen}
        isMobile={isMobile}
        onClose={closeMobile}
      />

      <div
        className="flex-1 min-w-0 flex flex-col transition-all duration-300 min-h-screen overflow-hidden"
        style={{
          marginLeft: isMobile ? 0 : (isOpen ? "256px" : "72px"),
          maxWidth: '100%',
        }}
      >
        <TopMenu toggleMenu={toggleMenu} isOpen={sidebarVisible} />
        <main className="flex-1 p-4 sm:p-6 overflow-x-hidden">{children}</main>
      </div>
    </div>
  );
};

export default MenuLayout;
