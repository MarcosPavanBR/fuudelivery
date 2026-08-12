import React, { useState, useEffect } from "react";
import api from "../services/api";

const DashboardCharts = ({ establishmentId }) => {
  const [stats, setStats] = useState({
    todayOrders: 0,
    weekRevenue: 0,
    avgTicket: 0,
    popularProducts: [],
    revenueByDay: [],
  });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
    if (establishmentId) {
      loadStats();
    }
  }, [establishmentId]);

  const loadStats = async () => {
    setLoading(true);
    setError(null);
    try {
      // Busca relatório semanal do Payment Service
      const reportRes = await api.get(
        `/reports/establishment/${establishmentId}?period=week`
      );
      const report = reportRes.data;

      // Busca pedidos de hoje
      let todayOrders = 0;
      try {
        const ordersRes = await api.get(`/orders/${establishmentId}`);
        const orders = ordersRes.data || [];
        const today = new Date().toISOString().split("T")[0];
        todayOrders = orders.filter((o) => {
          const orderDate = new Date(o.created_at || o.CreatedAt);
          return orderDate.toISOString().split("T")[0] === today;
        }).length;
      } catch {
        // Se falhar, usa 0 (endpoint pode não existir ainda)
        todayOrders = 0;
      }

      // Mapeia receita por dia para o formato da UI
      const dayNames = ["Dom", "Seg", "Ter", "Qua", "Qui", "Sex", "Sáb"];
      const revenueByDay = (report.revenue_by_day || []).map((d) => {
        const date = new Date(d.date || d.Date);
        return {
          day: dayNames[date.getDay()] || d.date,
          value: Math.round(d.revenue || d.Revenue || 0),
        };
      });

      // Se não houver dados reais, mostra zeros (não mock)
      setStats({
        todayOrders,
        weekRevenue: report.total_revenue || 0,
        avgTicket: report.avg_ticket || 0,
        popularProducts: [], // TODO: endpoint de produtos mais vendidos
        revenueByDay: revenueByDay.length > 0
          ? revenueByDay
          : ["Seg", "Ter", "Qua", "Qui", "Sex", "Sáb", "Dom"].map((day) => ({
              day,
              value: 0,
            })),
      });
    } catch (err) {
      console.error("Erro ao carregar dashboard:", err);
      setError("Não foi possível carregar os dados. Verifique sua conexão.");
      // Mantém zeros em caso de erro
    } finally {
      setLoading(false);
    }
  };

  const maxRevenue = Math.max(...stats.revenueByDay.map((d) => d.value), 1);

  if (loading) {
    return (
      <div className="mb-6 animate-fade-in">
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4 mb-6">
          {[1, 2, 3].map((i) => (
            <div
              key={i}
              className="bg-white rounded-2xl p-5 shadow-card border border-gray-100 animate-pulse"
            >
              <div className="h-4 bg-gray-200 rounded w-24 mb-3" />
              <div className="h-8 bg-gray-200 rounded w-32" />
            </div>
          ))}
        </div>
        <div className="bg-white rounded-2xl p-6 shadow-card border border-gray-100 mb-6">
          <div className="h-6 bg-gray-200 rounded w-40 mb-6" />
          <div className="h-48 bg-gray-100 rounded" />
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="mb-6 bg-red-50 border border-red-200 rounded-2xl p-6 text-center">
        <p className="text-red-600 font-medium">{error}</p>
        <button
          onClick={loadStats}
          className="mt-3 px-4 py-2 bg-red-500 text-white rounded-xl hover:bg-red-600 transition-colors"
        >
          Tentar novamente
        </button>
      </div>
    );
  }

  const statCards = [
    {
      label: "Pedidos Hoje",
      value: stats.todayOrders,
      icon: (
        <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth="2">
          <path strokeLinecap="round" strokeLinejoin="round" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
        </svg>
      ),
      color: "#EA1D2C",
      bg: "#FEF2F2",
      accent: "linear-gradient(135deg, #EA1D2C, #FF6B35)",
    },
    {
      label: "Receita da Semana",
      value: `R$ ${stats.weekRevenue.toFixed(2)}`,
      icon: (
        <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth="2">
          <path strokeLinecap="round" strokeLinejoin="round" d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
      ),
      color: "#F7A11E",
      bg: "#FFFBEB",
      accent: "linear-gradient(135deg, #F7A11E, #FBBF24)",
    },
    {
      label: "Ticket Médio",
      value: `R$ ${stats.avgTicket.toFixed(2)}`,
      icon: (
        <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth="2">
          <path strokeLinecap="round" strokeLinejoin="round" d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6" />
        </svg>
      ),
      color: "#10B981",
      bg: "#ECFDF5",
      accent: "linear-gradient(135deg, #10B981, #34D399)",
    },
  ];

  const hasRevenue = stats.revenueByDay.some((d) => d.value > 0);

  return (
    <div className="mb-6 animate-fade-in">
      {/* Stat Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4 mb-6">
        {statCards.map((card, i) => (
          <div
            key={i}
            className="relative bg-white rounded-2xl p-5 shadow-card hover:shadow-card-hover transition-all duration-300 border border-gray-100 overflow-hidden"
          >
            <div
              className="absolute left-0 top-0 bottom-0 w-1"
              style={{ background: card.accent }}
            />
            <div className="flex items-start justify-between">
              <div className="min-w-0">
                <p className="text-sm font-medium text-gray-500">{card.label}</p>
                <p
                  className="text-2xl font-bold mt-2"
                  style={{ color: card.color }}
                >
                  {card.value}
                </p>
              </div>
              <div
                className="p-3 rounded-xl flex-shrink-0"
                style={{ background: card.bg, color: card.color }}
              >
                {card.icon}
              </div>
            </div>
          </div>
        ))}
      </div>

      {/* Revenue Chart */}
      <div className="bg-white rounded-2xl p-6 shadow-card border border-gray-100 mb-6">
        <h3 className="text-lg font-bold text-gray-900 mb-6">
          Receita por Dia
        </h3>
        {hasRevenue ? (
          <div className="flex items-end justify-between h-48 gap-3">
            {stats.revenueByDay.map((day, i) => (
              <div key={i} className="flex flex-col items-center flex-1 h-full justify-end">
                {day.value > 0 && (
                  <span className="text-xs font-semibold text-gray-700 mb-2">
                    R${day.value}
                  </span>
                )}
                <div
                  className="w-full rounded-t-lg transition-all duration-500 hover:opacity-80 cursor-pointer"
                  style={{
                    height: `${Math.max((day.value / maxRevenue) * 160, day.value > 0 ? 8 : 2)}px`,
                    background:
                      i === 5 || i === 6
                        ? "linear-gradient(180deg, #F7A11E, #F59E0B)"
                        : "linear-gradient(180deg, #EA1D2C, #FF6B35)",
                  }}
                />
                <span className="text-xs font-medium text-gray-500 mt-2">
                  {day.day}
                </span>
              </div>
            ))}
          </div>
        ) : (
          <div className="flex flex-col items-center justify-center h-48 rounded-xl bg-gray-50 border border-dashed border-gray-200">
            <svg className="w-10 h-10 text-gray-300 mb-2" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth="1.5">
              <path strokeLinecap="round" strokeLinejoin="round" d="M3 3v18h18M7 14l4-4 3 3 5-6" />
            </svg>
            <p className="text-sm font-medium text-gray-500">Sem receita ainda</p>
            <p className="text-xs text-gray-400 mt-1">
              Os pedidos concluídos aparecerão aqui
            </p>
          </div>
        )}
      </div>

      {/* Popular Products - placeholder until endpoint exists */}
      <div className="bg-white rounded-2xl p-6 shadow-card border border-gray-100">
        <h3 className="text-lg font-bold text-gray-900 mb-4">
          Produtos Mais Vendidos
        </h3>
        {stats.popularProducts.length > 0 ? (
          <div className="space-y-3">
            {stats.popularProducts.map((p, i) => (
              <div
                key={i}
                className="flex items-center gap-4 p-3 rounded-xl bg-gray-50 hover:bg-gray-100 transition-colors"
              >
                <div
                  className="w-10 h-10 rounded-xl flex items-center justify-center text-white font-bold text-sm"
                  style={{
                    background:
                      i === 0
                        ? "linear-gradient(135deg, #EA1D2C, #FF6B35)"
                        : i === 1
                        ? "linear-gradient(135deg, #F7A11E, #FBBF24)"
                        : "linear-gradient(135deg, #6B7280, #9CA3AF)",
                  }}
                >
                  {i + 1}
                </div>
                <div className="flex-1">
                  <span className="font-semibold text-gray-900">{p.name}</span>
                </div>
                <div className="text-right">
                  <span className="font-bold text-gray-900">{p.count}</span>
                  <span className="text-xs text-gray-500 ml-1">un.</span>
                </div>
              </div>
            ))}
          </div>
        ) : (
          <div className="text-center py-10">
            <div className="mx-auto w-12 h-12 rounded-2xl bg-gray-50 flex items-center justify-center mb-3">
              <svg className="w-6 h-6 text-gray-300" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth="1.5">
                <path strokeLinecap="round" strokeLinejoin="round" d="M12 6v6m0 0v6m0-6h6m-6 0H6" />
              </svg>
            </div>
            <p className="font-medium text-gray-500">Dados de produtos em breve</p>
            <p className="text-xs text-gray-400 mt-1">
              Os produtos mais vendidos aparecerão aqui
            </p>
          </div>
        )}
      </div>
    </div>
  );
};

export default DashboardCharts;
