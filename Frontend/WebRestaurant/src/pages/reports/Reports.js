import React, { useState, useEffect } from "react";
import { useAuth } from "../../context/AuthContext";
import {
  FiBarChart2,
  FiDollarSign,
  FiShoppingBag,
  FiTruck,
  FiCalendar,
  FiLoader,
  FiDownload,
  FiTrendingUp,
  FiArrowUp,
  FiArrowDown,
} from "react-icons/fi";
import api from "../../services/api";

const Reports = () => {
  const { user } = useAuth();
  const [loading, setLoading] = useState(true);
  const [period, setPeriod] = useState("month");
  const [stats, setStats] = useState({
    totalRevenue: 0,
    totalOrders: 0,
    avgTicket: 0,
    deliveryRevenue: 0,
    ordersByStatus: {},
    revenueByDay: [],
  });

  useEffect(() => {
    fetchStats();
  }, [period]);

  const fetchStats = async () => {
    setLoading(true);
    try {
      const establishmentId = user?.establishment?.id || user?.id;
      const { data } = await api.get(
        `/reports/establishment/${establishmentId}?period=${period}`
      );
      setStats(data);
    } catch (err) {
      console.error("Failed to load reports:", err);
      // Dados mockados para demonstração
      setStats({
        totalRevenue: 12450.80,
        totalOrders: 187,
        avgTicket: 66.58,
        deliveryRevenue: 2340.00,
        ordersByStatus: {
          delivered: 165,
          cancelled: 15,
          pending: 7,
        },
        revenueByDay: [
          { date: "01/07", revenue: 420.5 },
          { date: "02/07", revenue: 380.2 },
          { date: "03/07", revenue: 510.8 },
          { date: "04/07", revenue: 290.0 },
          { date: "05/07", revenue: 620.3 },
          { date: "06/07", revenue: 580.1 },
          { date: "07/07", revenue: 450.9 },
        ],
      });
    } finally {
      setLoading(false);
    }
  };

  const formatCurrency = (value) => {
    return new Intl.NumberFormat("pt-BR", {
      style: "currency",
      currency: "BRL",
    }).format(value);
  };

  const periodLabels = {
    week: "Esta Semana",
    month: "Este Mês",
    quarter: "Este Trimestre",
    year: "Este Ano",
  };

  const StatCard = ({ icon: Icon, label, value, color, trend }) => (
    <div className="bg-white rounded-2xl border border-gray-100 shadow-card p-5">
      <div className="flex items-center justify-between">
        <div>
          <p className="text-xs font-semibold text-gray-500 uppercase">{label}</p>
          <p className="text-2xl font-bold text-gray-900 mt-1">{value}</p>
          {trend !== undefined && (
            <div
              className={`flex items-center gap-1 mt-1 text-xs font-medium ${
                trend >= 0 ? "text-green-600" : "text-red-600"
              }`}
            >
              {trend >= 0 ? (
                <FiArrowUp className="h-3 w-3" />
              ) : (
                <FiArrowDown className="h-3 w-3" />
              )}
              {Math.abs(trend)}% vs mês anterior
            </div>
          )}
        </div>
        <div className={`p-3 rounded-xl ${color}`}>
          <Icon className="h-6 w-6 text-white" />
        </div>
      </div>
    </div>
  );

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <FiLoader className="animate-spin h-8 w-8" style={{ color: "#EA1D2C" }} />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div className="flex items-center gap-2">
          <div className="p-2.5 rounded-xl bg-red-50">
            <FiBarChart2 className="h-5 w-5" style={{ color: "#EA1D2C" }} />
          </div>
          <div>
            <h1 className="text-lg font-bold text-gray-900">Relatórios</h1>
            <p className="text-xs text-gray-500">Acompanhe o desempenho do seu restaurante</p>
          </div>
        </div>

        <div className="flex items-center gap-2">
          {/* Seletor de período */}
          <div className="flex bg-gray-100 rounded-xl p-1">
            {Object.entries(periodLabels).map(([key, label]) => (
              <button
                key={key}
                onClick={() => setPeriod(key)}
                className={`px-3 py-1.5 text-xs font-medium rounded-lg transition-all ${
                  period === key
                    ? "bg-white text-gray-900 shadow-sm"
                    : "text-gray-500 hover:text-gray-700"
                }`}
              >
                {label}
              </button>
            ))}
          </div>

          {/* Exportar */}
          <button className="p-2.5 rounded-xl border border-gray-200 hover:bg-gray-50 transition-colors">
            <FiDownload className="h-5 w-5 text-gray-600" />
          </button>
        </div>
      </div>

      {/* Cards de estatísticas */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard
          icon={FiDollarSign}
          label="Receita Total"
          value={formatCurrency(stats.totalRevenue)}
          color="bg-green-500"
          trend={12.5}
        />
        <StatCard
          icon={FiShoppingBag}
          label="Total de Pedidos"
          value={stats.totalOrders}
          color="bg-blue-500"
          trend={8.3}
        />
        <StatCard
          icon={FiTrendingUp}
          label="Ticket Médio"
          value={formatCurrency(stats.avgTicket)}
          color="bg-purple-500"
          trend={3.2}
        />
        <StatCard
          icon={FiTruck}
          label="Receita Entrega"
          value={formatCurrency(stats.deliveryRevenue)}
          color="bg-orange-500"
          trend={-2.1}
        />
      </div>

      {/* Gráfico de receita por dia */}
      <div className="bg-white rounded-2xl border border-gray-100 shadow-card p-6">
        <div className="flex items-center gap-2 mb-6">
          <FiCalendar className="h-5 w-5 text-gray-400" />
          <h2 className="text-sm font-semibold text-gray-900">Receita Diária</h2>
        </div>

        <div className="space-y-3">
          {stats.revenueByDay.map((day, index) => {
            const maxRevenue = Math.max(
              ...stats.revenueByDay.map((d) => d.revenue)
            );
            const percentage = maxRevenue > 0 ? (day.revenue / maxRevenue) * 100 : 0;

            return (
              <div key={index} className="flex items-center gap-4">
                <span className="text-xs font-medium text-gray-500 w-12">
                  {day.date}
                </span>
                <div className="flex-1 bg-gray-100 rounded-full h-6 overflow-hidden">
                  <div
                    className="h-full rounded-full transition-all duration-500"
                    style={{
                      width: `${percentage}%`,
                      background: "linear-gradient(135deg, #EA1D2C, #C41420)",
                    }}
                  />
                </div>
                <span className="text-xs font-semibold text-gray-700 w-20 text-right">
                  {formatCurrency(day.revenue)}
                </span>
              </div>
            );
          })}
        </div>
      </div>

      {/* Status dos pedidos */}
      <div className="bg-white rounded-2xl border border-gray-100 shadow-card p-6">
        <h2 className="text-sm font-semibold text-gray-900 mb-4">Pedidos por Status</h2>
        <div className="grid grid-cols-3 gap-4">
          <div className="text-center p-4 bg-green-50 rounded-xl">
            <p className="text-2xl font-bold text-green-600">
              {stats.ordersByStatus.delivered || 0}
            </p>
            <p className="text-xs text-green-600 font-medium mt-1">Entregues</p>
          </div>
          <div className="text-center p-4 bg-yellow-50 rounded-xl">
            <p className="text-2xl font-bold text-yellow-600">
              {stats.ordersByStatus.pending || 0}
            </p>
            <p className="text-xs text-yellow-600 font-medium mt-1">Pendentes</p>
          </div>
          <div className="text-center p-4 bg-red-50 rounded-xl">
            <p className="text-2xl font-bold text-red-600">
              {stats.ordersByStatus.cancelled || 0}
            </p>
            <p className="text-xs text-red-600 font-medium mt-1">Cancelados</p>
          </div>
        </div>
      </div>
    </div>
  );
};

export default Reports;
