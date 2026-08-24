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
} from "react-icons/fi";
import api from "../../services/api";
import MenuLayout from "../../components/Menu";

const Reports = () => {
  const { user } = useAuth();
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState(false);
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
      setLoadError(false);
    } catch (err) {
      // Antes zerava tudo em silêncio: R$ 0,00 aparecia como dado real.
      setLoadError(true);
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

  const StatCard = ({ icon: Icon, label, value, color }) => (
    <div className="bg-white rounded-xl border border-gray-100 shadow-card p-6">
      <div className="flex items-center justify-between">
        <div>
          <p className="text-xs font-semibold text-gray-500 uppercase">{label}</p>
          <p className="text-2xl font-bold text-gray-900 mt-1">{value}</p>
        </div>
        <div className={`p-2 rounded-lg ${color}`}>
          <Icon className="h-6 w-6 text-white" />
        </div>
      </div>
    </div>
  );

  const exportCSV = () => {
    const rows = [
      ["FuuDelivery - Relatorio", periodLabels[period]],
      [],
      ["Indicador", "Valor"],
      ["Receita Total (R$)", Number(stats.totalRevenue || 0).toFixed(2)],
      ["Total de Pedidos", stats.totalOrders],
      ["Ticket Medio (R$)", Number(stats.avgTicket || 0).toFixed(2)],
      ["Receita Entrega (R$)", Number(stats.deliveryRevenue || 0).toFixed(2)],
      [],
      ["Data", "Receita (R$)"],
      ...(stats.revenueByDay || []).map((d) => [
        d.date,
        Number(d.revenue || 0).toFixed(2),
      ]),
    ];
    const csv = rows.map((r) => r.join(";")).join("\n");
    // BOM UTF-8: evita acentos quebrados ao abrir no Excel/Sheets.
    const blob = new Blob(["\uFEFF" + csv], {
      type: "text/csv;charset=utf-8;",
    });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `relatorio-${period}-${new Date().toISOString().slice(0, 10)}.csv`;
    a.click();
    URL.revokeObjectURL(url);
  };

  if (loading) {
    return (
      <MenuLayout>
        <div className="flex items-center justify-center min-h-[400px]">
          <FiLoader className="animate-spin h-8 w-8" style={{ color: "#DC2626" }} />
        </div>
      </MenuLayout>
    );
  }

  if (loadError) {
    return (
      <MenuLayout>
        <div className="flex flex-col items-center justify-center min-h-[400px] gap-3">
          <FiBarChart2 className="h-8 w-8 text-gray-300" />
          <p className="text-sm text-gray-500">
            Não foi possível carregar os relatórios. Verifique sua conexão.
          </p>
          <button
            onClick={fetchStats}
            className="rounded-lg bg-[#DC2626] px-4 py-2 text-sm font-semibold text-white hover:bg-[#B91C1C]"
          >
            Tentar novamente
          </button>
        </div>
      </MenuLayout>
    );
  }

  return (
    <MenuLayout>
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div className="flex items-center gap-2">
          <div className="p-2 rounded-lg bg-red-50">
            <FiBarChart2 className="h-5 w-5" style={{ color: "#DC2626" }} />
          </div>
          <div>
            <h1 className="text-lg font-bold text-gray-900">Relatórios</h1>
            <p className="text-xs text-gray-500">Acompanhe o desempenho do seu restaurante</p>
          </div>
        </div>

        <div className="flex items-center gap-2">
          {/* Seletor de período */}
          <div className="flex bg-gray-100 rounded-lg p-1">
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
          <button
            onClick={exportCSV}
            title="Exportar CSV"
            className="p-2 rounded-lg border border-gray-200 hover:bg-gray-50 transition-colors"
          >
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
        />
        <StatCard
          icon={FiShoppingBag}
          label="Total de Pedidos"
          value={stats.totalOrders}
          color="bg-blue-500"
        />
        <StatCard
          icon={FiTrendingUp}
          label="Ticket Médio"
          value={formatCurrency(stats.avgTicket)}
          color="bg-purple-500"
        />
        <StatCard
          icon={FiTruck}
          label="Receita Entrega"
          value={formatCurrency(stats.deliveryRevenue)}
          color="bg-orange-500"
        />
      </div>

      {/* Gráfico de receita por dia */}
      <div className="bg-white rounded-xl border border-gray-100 shadow-card p-6">
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
                      background: "linear-gradient(135deg, #DC2626, #B91C1C)",
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
      <div className="bg-white rounded-xl border border-gray-100 shadow-card p-6">
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
    </MenuLayout>
  );
};

export default Reports;
