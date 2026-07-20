import React, { useState, useEffect } from "react";
import { FiUsers, FiShoppingBag, FiTruck, FiDollarSign, FiTrendingUp, FiActivity, FiBuilding2, FiMapPin, FiClock } from "react-icons/fi";
import api from "../services/api";

const statCards = [
  { label: "Restaurantes Ativos", value: "0", icon: FiBuilding2, color: "#EA1D2C", bg: "#FEF2F2" },
  { label: "Total de Usuários", value: "0", icon: FiUsers, color: "#F7A11E", bg: "#FFFBEB" },
  { label: "Pedidos Hoje", value: "0", icon: FiShoppingBag, color: "#10B981", bg: "#ECFDF5" },
  { label: "Entregadores Online", value: "0", icon: FiTruck, color: "#3B82F6", bg: "#DBEAFE" },
];

const deliveryStatusColors = {
  pending: { bg: "#FEF3C7", text: "#B45309", label: "Pendente" },
  approved: { bg: "#DBEAFE", text: "#1D4ED8", label: "Aprovado" },
  preparing: { bg: "#FEF3C7", text: "#B45309", label: "Preparando" },
  ready: { bg: "#DBEAFE", text: "#1D4ED8", label: "Pronto" },
  delivering: { bg: "#FEF3C7", text: "#B45309", label: "Em Rota" },
  delivered: { bg: "#ECFDF5", text: "#047857", label: "Entregue" },
  cancelled: { bg: "#FEE2E2", text: "#B91C1C", label: "Cancelado" },
};

export default function Dashboard() {
  const [stats, setStats] = useState({ restaurants: 0, users: 0, todayOrders: 0, onlineDrivers: 0 });
  const [recentOrders, setRecentOrders] = useState([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    loadDashboard();
  }, []);

  const loadDashboard = async () => {
    try {
      const [establishments, users, orders, drivers] = await Promise.all([
        api.get("/establishments"),
        api.get("/users"),
        api.get("/orders"),
        api.get("/delivery-man"),
      ]);

      const today = new Date().toDateString();
      const todayOrders = orders.data?.filter(o => new Date(o.createdAt).toDateString() === today) || [];

      const activeDrivers = drivers.data?.filter(d => d.status === "online" || d.status === "available") || [];

      setStats({
        restaurants: establishments.data?.length || 0,
        users: users.data?.length || 0,
        todayOrders: todayOrders.length,
        onlineDrivers: activeDrivers.length,
      });

      const recent = orders.data?.slice(0, 10).map(order => ({
        id: order._id || order.id,
        customer: order.user?.nome || order.customer?.name || "Cliente",
        establishment: order.establishment?.name || "Restaurante",
        status: order.status || "pending",
        total: order.total || 0,
        createdAt: order.createdAt,
      })) || [];

      setRecentOrders(recent);
    } catch (e) {
      console.error(e);
    }
    setLoading(false);
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <FiActivity className="animate-spin h-8 w-8 text-fuu-red" />
      </div>
    );
  }

  return (
    <div className="animate-fade-in space-y-8">
      {/* Page Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Dashboard</h1>
          <p className="text-gray-500 mt-1">Visão geral do sistema</p>
        </div>
        <div className="flex items-center gap-2 text-sm text-gray-500">
          <FiActivity className="h-4 w-4 text-green-500 animate-pulse" />
          <span>Sistema Online</span>
        </div>
      </div>

      {/* Stats Grid */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        {[
          { label: "Restaurantes Ativos", value: stats.restaurants, icon: FiBuilding2, color: "#EA1D2C", bg: "#FEF2F2" },
          { label: "Total de Usuários", value: stats.users, icon: FiUsers, color: "#F7A11E", bg: "#FFFBEB" },
          { label: "Pedidos Hoje", value: stats.todayOrders, icon: FiShoppingBag, color: "#10B981", bg: "#ECFDF5" },
          { label: "Entregadores Online", value: stats.onlineDrivers, icon: FiTruck, color: "#3B82F6", bg: "#DBEAFE" },
        ].map((stat, i) => (
          <div key={i} className="bg-white rounded-2xl p-5 shadow-card hover:shadow-card-hover transition-all duration-300 border border-gray-100">
            <div className="flex items-start justify-between">
              <div>
                <p className="text-sm font-medium text-gray-500">{stat.label}</p>
                <p className="text-3xl font-bold mt-2 text-gray-900">{stat.value}</p>
              </div>
              <div className="p-3 rounded-xl" style={{ background: stat.bg, color: stat.color }}>
                <stat.icon className="h-6 w-6" />
              </div>
            </div>
          </div>
        ))}
      </div>

      {/* Recent Orders */}
      <div className="bg-white rounded-2xl shadow-card border border-gray-100 overflow-hidden">
        <div className="flex items-center justify-between px-6 py-4 border-b border-gray-100">
          <h2 className="text-lg font-bold text-gray-900">Pedidos Recentes</h2>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Pedido</th>
                <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Cliente</th>
                <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Restaurante</th>
                <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Status</th>
                <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Total</th>
                <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Hora</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {recentOrders.length === 0 ? (
                <tr>
                  <td colSpan={6} className="px-6 py-12 text-center text-gray-500">
                    Nenhum pedido recente
                  </td>
                </tr>
              ) : (
                recentOrders.map((order) => (
                  <tr key={order.id} className="hover:bg-gray-50 transition-colors">
                    <td className="px-6 py-4">
                      <span className="font-medium text-gray-900">#{order.id?.toString().slice(-8)}</span>
                    </td>
                    <td className="px-6 py-4">
                      <div className="flex items-center gap-2">
                        <div className="w-8 h-8 rounded-full bg-fuu-red-light flex items-center justify-center">
                          <span className="text-xs font-bold text-fuu-red">
                            {order.customer?.charAt(0) || "C"}
                          </span>
                        </div>
                        <span className="text-sm text-gray-900">{order.customer}</span>
                      </div>
                    </td>
                    <td className="px-6 py-4 text-sm text-gray-600">{order.establishment}</td>
                    <td className="px-6 py-4">
                      {(deliveryStatusColors[order.status] ? (
                        <span className="inline-flex items-center px-2.5 py-1 rounded-full text-xs font-medium"
                          style={{ background: deliveryStatusColors[order.status].bg, color: deliveryStatusColors[order.status].text }}>
                          {deliveryStatusColors[order.status].label}
                        </span>
                      ) : (
                        <span className="inline-flex items-center px-2.5 py-1 rounded-full text-xs font-medium bg-gray-100 text-gray-600">
                          {order.status}
                        </span>
                      )))}
                    </td>
                    <td className="px-6 py-4 font-semibold text-gray-900">
                      R$ {order.total?.toFixed(2) || "0,00"}
                    </td>
                    <td className="px-6 py-4 text-sm text-gray-500">
                      {order.createdAt ? new Date(order.createdAt).toLocaleTimeString("pt-BR", { hour: "2-digit", minute: "2-digit" }) : "-"}
                    </td>
                  </tr>
                )))}
            </tbody>
          </table>
        </div>
        <div className="px-6 py-4 border-t border-gray-100">
          <a href="/orders" className="text-sm font-medium" style={{ color: "#EA1D2C" }}>
            Ver todos os pedidos →
          </a>
        </div>
      </div>
    </div>
  );
}