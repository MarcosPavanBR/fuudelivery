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
    try {
      setStats({
        todayOrders: 12,
        weekRevenue: 4580.50,
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
          { day: "Sáb", value: 450 },
          { day: "Dom", value: 370 },
        ],
      });
    } catch (e) {
      console.error(e);
    }
  };

  const maxRevenue = Math.max(...stats.revenueByDay.map(d => d.value), 1);

  const styles = {
    container: {
      padding: 24,
    },
    title: {
      fontSize: 24,
      fontWeight: "bold",
      marginBottom: 24,
      color: "#333",
    },
    grid: {
      display: "grid",
      gridTemplateColumns: "repeat(auto-fit, minmax(200px, 1fr))",
      gap: 16,
      marginBottom: 32,
    },
    card: {
      background: "#FFF",
      borderRadius: 12,
      padding: 20,
      boxShadow: "0 2px 8px rgba(0,0,0,0.08)",
    },
    bigNumber: {
      fontSize: 28,
      fontWeight: "bold",
      color: "#F97316",
      marginTop: 8,
    },
    chartSection: {
      background: "#FFF",
      borderRadius: 12,
      padding: 20,
      boxShadow: "0 2px 8px rgba(0,0,0,0.08)",
      marginBottom: 32,
    },
    chart: {
      display: "flex",
      alignItems: "flex-end",
      justifyContent: "space-around",
      height: 240,
      paddingTop: 20,
    },
    barContainer: {
      display: "flex",
      flexDirection: "column",
      alignItems: "center",
      flex: 1,
    },
    bar: {
      width: 40,
      backgroundColor: "#F97316",
      borderRadius: "6px 6px 0 0",
      display: "flex",
      alignItems: "flex-start",
      justifyContent: "center",
      transition: "height 0.3s ease",
      minHeight: 4,
    },
    barValue: {
      fontSize: 11,
      color: "#FFF",
      fontWeight: "bold",
      marginTop: 4,
    },
    barLabel: {
      marginTop: 8,
      fontSize: 12,
      color: "#666",
    },
    tableSection: {
      background: "#FFF",
      borderRadius: 12,
      padding: 20,
      boxShadow: "0 2px 8px rgba(0,0,0,0.08)",
    },
    table: {
      width: "100%",
      borderCollapse: "collapse",
      marginTop: 12,
    },
  };

  return (
    <div style={styles.container}>
      <h2 style={styles.title}>Dashboard</h2>
      <div style={styles.grid}>
        <div style={styles.card}>
          <h3 style={{ margin: 0, color: "#666", fontSize: 14 }}>Pedidos Hoje</h3>
          <p style={styles.bigNumber}>{stats.todayOrders}</p>
        </div>
        <div style={styles.card}>
          <h3 style={{ margin: 0, color: "#666", fontSize: 14 }}>Receita da Semana</h3>
          <p style={styles.bigNumber}>R$ {stats.weekRevenue.toFixed(2)}</p>
        </div>
        <div style={styles.card}>
          <h3 style={{ margin: 0, color: "#666", fontSize: 14 }}>Ticket Médio</h3>
          <p style={styles.bigNumber}>R$ {stats.avgTicket.toFixed(2)}</p>
        </div>
      </div>

      <div style={styles.chartSection}>
        <h3 style={{ margin: 0, color: "#333" }}>Receita por Dia</h3>
        <div style={styles.chart}>
          {stats.revenueByDay.map((day, i) => (
            <div key={i} style={styles.barContainer}>
              <div
                style={{
                  ...styles.bar,
                  height: `${(day.value / maxRevenue) * 200}px`,
                }}
              >
                <span style={styles.barValue}>R${day.value}</span>
              </div>
              <span style={styles.barLabel}>{day.day}</span>
            </div>
          ))}
        </div>
      </div>

      <div style={styles.tableSection}>
        <h3 style={{ margin: 0, color: "#333" }}>Produtos Mais Vendidos</h3>
        <table style={styles.table}>
          <thead>
            <tr>
              <th style={{ textAlign: "left", padding: 8, borderBottom: "2px solid #EEE", color: "#666" }}>Produto</th>
              <th style={{ textAlign: "left", padding: 8, borderBottom: "2px solid #EEE", color: "#666" }}>Qtd</th>
            </tr>
          </thead>
          <tbody>
            {stats.popularProducts.map((p, i) => (
              <tr key={i}>
                <td style={{ padding: 8, borderBottom: "1px solid #EEE" }}>{p.name}</td>
                <td style={{ padding: 8, borderBottom: "1px solid #EEE", fontWeight: "bold" }}>{p.count}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
};

export default DashboardCharts;
