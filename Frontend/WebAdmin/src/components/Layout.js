import React, { useState } from "react";
import { Outlet, NavLink, useNavigate } from "react-router-dom";
import { useAuth } from "../context/AuthContext";
import { FiMenu, FiX, FiHome, FiBuilding2, FiUsers, FiShoppingBag, FiTruck, FiCreditCard, FiSettings, FiLogOut, FiBarChart2, FiChevronLeft } from "react-icons/fi";

const menuItems = [
  { path: "/", label: "Dashboard", icon: FiBarChart2 },
  { path: "/establishments", label: "Estabelecimentos", icon: FiBuilding2 },
  { path: "/users", label: "Usuários", icon: FiUsers },
  <NavLink key="orders" to="/orders" className={({ isActive }) => `flex items-center gap-3 px-4 py-3 rounded-xl transition-all duration-200 ${isActive ? "bg-fuu-red-light text-fuu-red font-semibold" : "text-gray-600 hover:bg-gray-100 hover:text-gray-900"} font-medium`}>
    <FiShoppingBag className="h-5 w-5 flex-shrink-0" />
    Pedidos
  </NavLink>,
  { path: "/delivery-men", label: "Entregadores", icon: FiTruck },
  { path: "/payments", label: "Pagamentos", icon: FiCreditCard },
  { path: "/settings", label: "Configurações", icon: FiSettings },
];

export default function Layout() {
  const [sidebarOpen, setSidebarOpen] = useState(true);
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const { user, logout } = useAuth();
  const navigate = useNavigate();

  const handleLogout = () => {
    logout();
    navigate("/login");
  };

  return (
    <div className="min-h-screen bg-gray-50">
      {/* Mobile Overlay */}
      {mobileMenuOpen && (
        <div
          className="fixed inset-0 bg-black/50 z-40 lg:hidden"
          onClick={() => setMobileMenuOpen(false)}
        />
      )}

      {/* Sidebar */}
      <aside
        className={`fixed lg:static inset-y-0 left-0 z-50 w-64 bg-white border-r border-gray-200 transform transition-transform duration-300 ease-in-out lg:translate-x-0 ${mobileMenuOpen ? "translate-x-0" : "-translate-x-full"}`}
      >
        <div className="flex flex-col h-full">
          {/* Logo */}
          <div className="flex items-center justify-between h-16 px-4 border-b border-gray-100">
            <div className="flex items-center gap-2">
              <div className="w-9 h-9 rounded-xl flex items-center justify-center" style={{ background: "linear-gradient(135deg, #EA1D2C, #FF4444, #F7A11E)" }}>
                <svg className="w-5 h-5 text-white" viewBox="0 0 48 48" fill="none">
                  <rect width="48" height="48" rx="14" fill="url(#grad)" />
                  <path d="M14 14h12c4.4 0 8 3.6 8 8s-3.6 8-8 8h-4v8h-8V14zm8 12h4c2.2 0 4-1.8 4-4s-1.8-4-4-4h-4v8z" fill="white" />
                  <circle cx="38" cy="12" r="4" fill="#F7A11E" />
                  <path d="M35 10l3 2 3-2" stroke="white" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" opacity="0.9" />
                </svg>
              </div>
              <div className="flex flex-col">
                <span className="text-xl font-black text-fuu-red leading-none">Fuu</span>
                <span className="text-xs font-bold text-gray-900 tracking-widest uppercase">Delivery</span>
              </div>
            </div>
            <button
              onClick={() => setSidebarOpen(!sidebarOpen)}
              className="lg:hidden p-1 rounded-lg hover:bg-gray-100"
            >
              <FiChevronLeft className="h-5 w-5" />
            </button>
          </div>

          {/* Navigation */}
          <nav className="flex-1 px-3 py-4 space-y-1 overflow-y-auto">
            {menuItems.map((item) => (
              <NavLink
                key={item.path}
                to={item.path}
                className={({ isActive }) =>
                  `flex items-center gap-3 px-4 py-3 rounded-xl transition-all duration-200 ${
                    isActive
                      ? "bg-fuu-red-light text-fuu-red font-semibold"
                      : "text-gray-600 hover:bg-gray-100 hover:text-gray-900"
                  } font-medium`
                }
              >
                <item.icon className="h-5 w-5 flex-shrink-0" />
                {item.label}
              </NavLink>
            ))}
          </nav>

          {/* User Info & Logout */}
          <div className="p-4 border-t border-gray-100">
            <div className="flex items-center gap-3 mb-3">
              <div className="w-9 h-9 rounded-xl bg-fuu-red-light flex items-center justify-center">
                <FiUsers className="h-5 w-5 text-fuu-red" />
              </div>
              <div className="flex-1 min-w-0">
                <p className="text-sm font-semibold text-gray-900 truncate">
                  {user?.name || "Admin"}
                </p>
                <p className="text-xs text-gray-500 truncate">{user?.email}</p>
              </div>
            </div>
            <button
              onClick={handleLogout}
              className="w-full flex items-center gap-3 px-4 py-2.5 rounded-xl text-gray-600 hover:bg-gray-100 hover:text-gray-900 transition-colors font-medium"
            >
              <FiLogOut className="h-5 w-5" />
              Sair
            </button>
          </div>
        </div>
      </aside>

      {/* Main Content */}
      <div className="lg:ml-64 min-h-screen">
        {/* Top Bar */}
        <header className="sticky top-0 z-30 bg-white border-b border-gray-100 lg:hidden">
          <div className="flex items-center justify-between h-16 px-4">
            <button
              onClick={() => setMobileMenuOpen(true)}
              className="p-2 rounded-lg hover:bg-gray-100"
            >
              <FiMenu className="h-6 w-6" />
            </button>
            <div className="flex items-center gap-2">
              <span className="text-xl font-black text-fuu-red">Fuu</span>
              <span className="text-sm font-bold text-gray-900">Delivery</span>
            </div>
            <div className="w-10" />
          </div>
        </header>

        <main className="p-4 lg:p-8 pt-0 lg:pt-8">
          <Outlet />
        </main>
      </div>
    </div>
  );
}