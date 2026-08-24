/**
 * Tela de Relatórios — Métricas de vendas do restaurante.
 *
 * Exibe pedidos hoje, receita da semana e ticket médio.
 */
import React, { useState, useEffect, useCallback } from "react";
import { View, Text, StyleSheet, ScrollView, RefreshControl } from "react-native";
import { useFocusEffect } from "@react-navigation/native";
import api from "@/services/api";
import { useApi } from "@/contexts/ApiContext";

export default function ReportsScreen() {
  const { getUserData } = useApi();
  const user = getUserData();
  const [stats, setStats] = useState({
    todayOrders: 0,
    weekRevenue: 0,
    avgTicket: 0,
  });
  const [refreshing, setRefreshing] = useState(false);

  const fetchStats = async () => {
    try {
      const establishmentId = user?.establishment_id;
      if (!establishmentId) return;

      const ordersResp = await api.get(`/orders/${establishmentId}`);
      const orders = ordersResp.data || [];

      const today = new Date().toDateString();
      const todayOrders = orders.filter(
        (o: any) => new Date(o.createdAt).toDateString() === today
      );

      const weekAgo = new Date();
      weekAgo.setDate(weekAgo.getDate() - 7);
      const weekOrders = orders.filter(
        (o: any) => new Date(o.createdAt) >= weekAgo && o.status !== "cancelled"
      );
      const weekRevenue = weekOrders.reduce(
        (sum: number, o: any) => sum + (o.total || 0),
        0
      );
      const avgTicket = weekOrders.length > 0 ? weekRevenue / weekOrders.length : 0;

      setStats({
        todayOrders: todayOrders.length,
        weekRevenue,
        avgTicket,
      });
    } catch (e) {
      console.error("Erro ao carregar relatórios:", e);
    }
  };

  useFocusEffect(
    useCallback(() => {
      fetchStats();
    }, [user?.establishment_id])
  );

  const onRefresh = async () => {
    setRefreshing(true);
    await fetchStats();
    setRefreshing(false);
  };

  return (
    <ScrollView
      style={styles.container}
      contentContainerStyle={styles.content}
      refreshControl={<RefreshControl refreshing={refreshing} onRefresh={onRefresh} />}
    >
      <View style={styles.card}>
        <Text style={styles.cardLabel}>Pedidos Hoje</Text>
        <Text style={[styles.cardValue, { color: "#DC2626" }]}>{stats.todayOrders}</Text>
      </View>

      <View style={styles.card}>
        <Text style={styles.cardLabel}>Receita da Semana</Text>
        <Text style={[styles.cardValue, { color: "#10B981" }]}>
          R$ {stats.weekRevenue.toFixed(2)}
        </Text>
      </View>

      <View style={styles.card}>
        <Text style={styles.cardLabel}>Ticket Médio</Text>
        <Text style={[styles.cardValue, { color: "#F7A11E" }]}>
          R$ {stats.avgTicket.toFixed(2)}
        </Text>
      </View>
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: "#F5F5F5" },
  content: { padding: 16, gap: 16 },
  card: {
    backgroundColor: "#FFF",
    borderRadius: 16,
    padding: 20,
    borderWidth: 1,
    borderColor: "#F3F4F6",
  },
  cardLabel: { fontSize: 14, color: "#6B7280", marginBottom: 8 },
  cardValue: { fontSize: 28, fontWeight: "700" },
});
