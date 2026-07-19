import React, { useState, useEffect } from "react";

const DashboardCharts = ({ establishmentId }) => {
  const [stats, setStats] = useState({
    todayOrders: 0,
    weekRevenue: 0,
    avgTicket: 0,
    popularProducts: [],
    revenueByDay: [],
  });

  useEffect(() => {
    loadStats();
  }, []);

  const loadStats = async () => {
    setStats({
      todayOrders: 12,
      weekRevenue: 4580.5,
      avgTicket: 42.35,
      popularProducts: [
        { name: "X-Burger", count: 45 },
        { name: "Batata Frita", count: 38 },
        { name: "Refrigerante", count: 32 },
      ],
      revenueByDay: [
        { day: "Seg", value: 580 },
        { day: "Ter", value: 720 },
        { day: "Qua", value: 650 },
        { day: "Qui", value: 890 },
        { day: "Sex", value: 920 },
        { day: "Sab", value: 450 },
        { day: "Dom", value: 370 },
      ],
    });
  };

  const maxRevenue = Math.max(...stats.revenueByDay.map((d) => d.value), 1);

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
    },
  ];

  return (
    <div className="mb-6 animate-fade-in">
      {/* Stat Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4 mb-6">
        {statCards.map((card, i) => (
          <div
            key={i}
            className="bg-white rounded-2xl p-5 shadow-card hover:shadow-card-hover transition-all duration-300 border border-gray-100"
          >
            <div className="flex items-start justify-between">
              <div>
                <p className="text-sm font-medium text-gray-500">{card.label}</p>
                <p
                  className="text-2xl font-bold mt-2"
                  style={{ color: card.color }}
                >
                  {card.value}
                </p>
              </div>
              <div
                className="p-3 rounded-xl"
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
        <div className="flex items-end justify-between h-48 gap-3">
          {stats.revenueByDay.map((day, i) => (
            <div key={i} className="flex flex-col items-center flex-1 h-full justify-end">
              <span className="text-xs font-semibold text-gray-700 mb-2">
                R${day.value}
              </span>
              <div
                className="w-full rounded-t-lg transition-all duration-500 hover:opacity-80 cursor-pointer"
                style={{
                  height: `${(day.value / maxRevenue) * 160}px`,
                  background:
                    i === 5 || i === 6
                      ? "linear-gradient(180deg, #F7A11E, #F59E0B)"
                      : "linear-gradient(180deg, #EA1D2C, #FF6B35)",
                  minHeight: 8,
                }}
              />
              <span className="text-xs font-medium text-gray-500 mt-2">
                {day.day}
              </span>
            </div>
          ))}
        </div>
      </div>

      {/* Popular Products */}
      <div className="bg-white rounded-2xl p-6 shadow-card border border-gray-100">
        <h3 className="text-lg font-bold text-gray-900 mb-4">
          Produtos Mais Vendidos
        </h3>
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
      </div>
    </div>
  );
};

export default DashboardCharts;
