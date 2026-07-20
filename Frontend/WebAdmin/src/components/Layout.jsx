import React, { useState } from "react";
import { Outlet, NavLink, useNavigate } from "react-router-dom";
import { useAuth } from "../context/AuthContext";
import { FiMenu, FiX, FiHome, FiBuilding2, FiUsers, FiShoppingBag, FiTruck, FiCreditCard, FiSettings, FiLogOut, FiBarChart2, FiChevronLeft } from "react-icons/fi";

const iconMap = {
  dashboard: FiBarChart2,
  establishments: FiBuilding2,
  users: FiUsers,
  orders: FiShoppingBag,
  "delivery-men": FiTruck,
  payments: FiCreditCard,
  settings: FiSettings,
};

const menuItems = [
  { path: "/", label: "Dashboard", iconKey: "dashboard" },
  { path: "/establishments", label: "Estabelecimentos", iconKey: "establishments" },
  { path: "/users", label: "Usuarios", iconKey: "users" },
  { path: "/orders", label: "Pedidos", iconKey: "orders" },
  { path: "/delivery-men", label: "Entregadores", iconKey: "delivery-men" },
  { path: "/payments", label: "Pagamentos", iconKey: "payments" },
  { path: "/settings", label: "Configuracoes", iconKey: "settings" },
];

function SidebarIcon({ iconKey, className }) {
  const Icon = iconMap[iconKey];
  if (!Icon) return null;
  return <Icon className={className} />;
}

export default function Layout() {
  const [sidebarOpen, setSidebarOpen] = useState(true);
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const auth = useAuth();
  const logout = auth?.logout;
  const user = auth?.user;
  const navigate = useNavigate();

  const handleLogout = () => {
    if (logout) logout();
    navigate("/login");
  };

  return (
    <div className="flex min-h-screen bg-gray-50">
      {mobileMenuOpen && (
        <div
          className="fixed inset-0 bg-black/50 z-40 lg:hidden"
          onClick={() => setMobileMenuOpen(false)}
        />
      )}

      <aside
        className={`fixed lg:static inset-y-0 left-0 z-50 flex flex-col transition-all duration-300 ease-in-out ${
          sidebarOpen ? "w-64" : "w-20"
        } ${mobileMenuOpen ? "translate-x-0" : "-translate-x-full lg:translate-x-0"}`}
        style={{
          background: "linear-gradient(180deg, #1A1A1A 0%, #111111 100%)",
          boxShadow: "4px 0 24px rgba(0,0,0,0.15)",
        }}
      >
        <div
          className={`flex items-center border-b border-white/10 transition-all duration-300 ${
            sidebarOpen ? "px-5 py-5" : "px-4 py-5 justify-center"
          }`}
        >
          <svg width={sidebarOpen ? 36 : 32} height={sidebarOpen ? 36 : 32} viewBox="0 0 48 48" fill="none" xmlns="http://www.w3.org/2000/svg">
            <defs>
              <linearGradient id="fuuGrad" x1="0%" y1="0%" x2="100%" y2="100%">
                <stop offset="0%" stopColor="#EA1D2C" />
                <stop offset="50%" stopColor="#FF4444" />
                <stop offset="100%" stopColor="#F7A11E" />
              </linearGradient>
            </defs>
            <rect width="48" height="48" rx="14" fill="url(#fuuGrad)" />
            <path d="M14 14h12c4.4 0 8 3.6 8 8s-3.6 8-8 8h-4v8h-8V14zm8 12h4c2.2 0 4-1.8 4-4s-1.8-4-4-4h-4v8z" fill="white" />
            <circle cx="38" cy="12" r="4" fill="#F7A11E" />
          </svg>
          {sidebarOpen && (
            <div className="ml-3 flex flex-col leading-none">
              <span style={{ fontSize: "18px", fontWeight: 900, color: "#EA1D2C", letterSpacing: "-0.5px", lineHeight: 1 }}>Fuu</span>
              <span style={{ fontSize: "10px", fontWeight: 700, color: "#9CA3AF", letterSpacing: "2px", textTransform: "uppercase", lineHeight: 1, marginTop: 2 }}>Delivery</span>
            </div>
          )}
        </div>

        <nav className="flex-1 py-4 px-3 overflow-y-auto">
          <ul className="space-y-1">
            {menuItems.map((item) => (
              <li key={item.path}>
                <NavLink
                  to={item.path}
                  end={item.path === "/"}
                  className={({ isActive }) =>
                    "flex items-center gap-3 px-4 py-3 rounded-xl transition-all duration-200 group " +
                    (isActive ? "text-white font-semibold" : "text-gray-400 hover:text-white hover:bg-white/10") +
                    (!sidebarOpen ? " justify-center px-0" : "")
                  }
                  title={!sidebarOpen ? item.label : undefined}
                >
                  <span className="flex-shrink-0">
                    <SidebarIcon iconKey={item.iconKey} className="h-5 w-5" />
                  </span>
                  {sidebarOpen && <span className="text-sm font-medium">{item.label}</span>}
                </NavLink>
              </li>
            ))}
          </ul>
        </nav>

        <div className={`border-t border-white/10 p-4 transition-all duration-300 ${!sidebarOpen ? "px-2" : ""}`}>
          <div className={`flex items-center gap-3 rounded-xl p-2 ${sidebarOpen ? "hover:bg-white/10" : "justify-center"}`}>
            <div className="w-8 h-8 rounded-xl flex items-center justify-center flex-shrink-0" style={{ background: "linear-gradient(135deg, #EA1D2C, #C41420)" }}>
              <FiLogOut className="h-4 w-4 text-white" />
            </div>
            {sidebarOpen && (
              <button onClick={handleLogout} className="flex-1 text-left text-sm font-medium text-gray-300 hover:text-white transition-colors">
                Sair
              </button>
            )}
          </div>
        </div>
      </aside>

      <div className={`flex-1 flex flex-col transition-all duration-300 lg:ml-0 ${sidebarOpen ? "lg:ml-64" : "lg:ml-20"}`}>
        <header className="sticky top-0 z-30 border-b border-gray-200 bg-white/80 backdrop-blur-sm">
          <div className="flex items-center justify-between px-6 py-4">
            <div className="flex items-center gap-4">
              <button onClick={() => setSidebarOpen(!sidebarOpen)} className="lg:hidden p-2 rounded-xl hover:bg-gray-100 transition-colors">
                <FiMenu className="h-6 w-6 text-gray-700" />
              </button>
              <button onClick={() => setSidebarOpen(!sidebarOpen)} className="hidden lg:flex p-2 rounded-xl hover:bg-gray-100 transition-colors">
                {sidebarOpen ? <FiChevronLeft className="h-5 w-5 text-gray-500" /> : <FiMenu className="h-5 w-5 text-gray-500" />}
              </button>
            </div>
            <div className="flex items-center gap-4">
              <div className="hidden md:block relative">
                <input type="text" placeholder="Buscar..." className="w-72 pl-10 pr-4 py-2 bg-gray-100 border-none rounded-xl text-sm text-gray-700 placeholder-gray-400 focus:bg-white focus:ring-2 transition-all" style={{ outline: "none" }} />
                <svg className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                </svg>
              </div>
              <div className="relative">
                <button className="flex items-center gap-3 p-2 rounded-xl hover:bg-gray-100 transition-colors">
                  <div className="w-8 h-8 rounded-xl flex items-center justify-center" style={{ background: "linear-gradient(135deg, #EA1D2C, #C41420)" }}>
                    <span className="text-white font-bold text-sm">{user?.name?.charAt(0) || "A"}</span>
                  </div>
                  <div className="hidden sm:block text-left">
                    <p className="text-sm font-semibold text-gray-900">{user?.name || "Admin"}</p>
                    <p className="text-xs text-gray-500">{user?.establishment_name || "Sistema"}</p>
                  </div>
                </button>
              </div>
            </div>
          </div>
        </header>
        <main className="flex-1 p-6">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
