/**
 * Tela principal do AppRestaurante — Lista de pedidos.
 *
 * Exibe pedidos recebidos com status, valor e horário.
 * O restaurante pode aceitar, rejeitar ou marcar como pronto.
 */
import React, { useState, useEffect, useCallback } from "react";
import {
  View,
  Text,
  StyleSheet,
  FlatList,
  TouchableOpacity,
  RefreshControl,
  Alert,
} from "react-native";
import { useFocusEffect } from "@react-navigation/native";
import { Feather } from "@expo/vector-icons";
import api from "@/services/api";
import { useApi } from "@/contexts/ApiContext";

interface Order {
  id: number;
  user?: { nome?: string; name?: string; phone?: string };
  status: string;
  total: number;
  createdAt: string;
  items?: any[];
}

const statusColors: Record<string, { bg: string; text: string; label: string }> = {
  pending: { bg: "#FEF3C7", text: "#B45309", label: "Pendente" },
  approved: { bg: "#DBEAFE", text: "#1D4ED8", label: "Aprovado" },
  preparing: { bg: "#FEF3C7", text: "#B45309", label: "Preparando" },
  ready: { bg: "#D1FAE5", text: "#047857", label: "Pronto" },
  delivering: { bg: "#DBEAFE", text: "#1D4ED8", label: "Em Rota" },
  delivered: { bg: "#ECFDF5", text: "#047857", label: "Entregue" },
  cancelled: { bg: "#FEE2E2", text: "#B91C1C", label: "Cancelado" },
};

export default function OrdersScreen() {
  const { getUserData } = useApi();
  const user = getUserData();
  const [orders, setOrders] = useState<Order[]>([]);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);

  const fetchOrders = async () => {
    try {
      const establishmentId = user?.establishment_id;
      if (!establishmentId) return;
      const resp = await api.get(`/orders/${establishmentId}`);
      setOrders(resp.data || []);
    } catch (e) {
      console.error("Erro ao carregar pedidos:", e);
    }
    setLoading(false);
  };

  useFocusEffect(
    useCallback(() => {
      fetchOrders();
    }, [user?.establishment_id])
  );

  const onRefresh = async () => {
    setRefreshing(true);
    await fetchOrders();
    setRefreshing(false);
  };

  const updateStatus = async (orderId: number, newStatus: string) => {
    try {
      await api.patch(`/orders/${orderId}/status`, { status: newStatus });
      setOrders((prev) =>
        prev.map((o) => (o.id === orderId ? { ...o, status: newStatus } : o))
      );
    } catch (e) {
      Alert.alert("Erro", "Falha ao atualizar status do pedido.");
    }
  };

  const handleAccept = (order: Order) => {
    Alert.alert(
      "Aceitar Pedido",
      `Aceitar pedido #${order.id} de R$ ${order.total?.toFixed(2)}?`,
      [
        { text: "Cancelar", style: "cancel" },
        { text: "Aceitar", onPress: () => updateStatus(order.id, "preparing") },
      ]
    );
  };

  const handleReady = (order: Order) => {
    updateStatus(order.id, "ready");
  };

  const handleReject = (order: Order) => {
    Alert.alert(
      "Rejeitar Pedido",
      `Rejeitar pedido #${order.id}?`,
      [
        { text: "Cancelar", style: "cancel" },
        { text: "Rejeitar", style: "destructive", onPress: () => updateStatus(order.id, "cancelled") },
      ]
    );
  };

  const renderOrder = ({ item }: { item: Order }) => {
    const status = statusColors[item.status] || { bg: "#F3F4F6", text: "#374151", label: item.status };
    const customerName = item.user?.nome || item.user?.name || "Cliente";
    const time = item.createdAt
      ? new Date(item.createdAt).toLocaleTimeString("pt-BR", { hour: "2-digit", minute: "2-digit" })
      : "-";

    return (
      <View style={styles.orderCard}>
        <View style={styles.orderHeader}>
          <Text style={styles.orderId}>#{item.id}</Text>
          <View style={[styles.statusBadge, { backgroundColor: status.bg }]}>
            <Text style={[styles.statusText, { color: status.text }]}>{status.label}</Text>
          </View>
        </View>

        <Text style={styles.customerName}>{customerName}</Text>
        <Text style={styles.orderTime}>🕐 {time}</Text>
        <Text style={styles.orderTotal}>R$ {item.total?.toFixed(2) || "0,00"}</Text>

        <View style={styles.actions}>
          {item.status === "pending" && (
            <>
              <TouchableOpacity style={styles.rejectBtn} onPress={() => handleReject(item)}>
                <Feather name="x" size={16} color="#B91C1C" />
                <Text style={styles.rejectText}>Rejeitar</Text>
              </TouchableOpacity>
              <TouchableOpacity style={styles.acceptBtn} onPress={() => handleAccept(item)}>
                <Feather name="check" size={16} color="#FFF" />
                <Text style={styles.acceptText}>Aceitar</Text>
              </TouchableOpacity>
            </>
          )}
          {item.status === "preparing" && (
            <TouchableOpacity style={styles.readyBtn} onPress={() => handleReady(item)}>
              <Feather name="check" size={16} color="#FFF" />
              <Text style={styles.readyText}>Pronto</Text>
            </TouchableOpacity>
          )}
        </View>
      </View>
    );
  };

  return (
    <View style={styles.container}>
      {loading ? (
        <View style={styles.center}>
          <Feather name="refresh-cw" size={32} color="#DC2626" />
          <Text style={styles.loadingText}>Carregando pedidos...</Text>
        </View>
      ) : orders.length === 0 ? (
        <View style={styles.center}>
          <Text style={styles.emptyEmoji}>📦</Text>
          <Text style={styles.emptyTitle}>Nenhum pedido</Text>
          <Text style={styles.emptySubtitle}>Os pedidos aparecerão aqui</Text>
        </View>
      ) : (
        <FlatList
          data={orders}
          keyExtractor={(item) => String(item.id)}
          renderItem={renderOrder}
          contentContainerStyle={styles.list}
          refreshControl={<RefreshControl refreshing={refreshing} onRefresh={onRefresh} />}
        />
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: "#F5F5F5" },
  list: { padding: 16, gap: 12 },
  center: { flex: 1, justifyContent: "center", alignItems: "center", gap: 12 },
  loadingText: { fontSize: 14, color: "#666" },
  emptyEmoji: { fontSize: 48 },
  emptyTitle: { fontSize: 18, fontWeight: "700", color: "#1A1A1A" },
  emptySubtitle: { fontSize: 14, color: "#666" },
  orderCard: {
    backgroundColor: "#FFF",
    borderRadius: 16,
    padding: 16,
    borderWidth: 1,
    borderColor: "#F3F4F6",
  },
  orderHeader: {
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "center",
    marginBottom: 8,
  },
  orderId: { fontSize: 16, fontWeight: "700", color: "#1A1A1A" },
  statusBadge: { paddingHorizontal: 10, paddingVertical: 4, borderRadius: 12 },
  statusText: { fontSize: 12, fontWeight: "600" },
  customerName: { fontSize: 14, color: "#374151", marginBottom: 4 },
  orderTime: { fontSize: 13, color: "#6B7280", marginBottom: 4 },
  orderTotal: { fontSize: 18, fontWeight: "700", color: "#DC2626", marginBottom: 12 },
  actions: { flexDirection: "row", gap: 8, justifyContent: "flex-end" },
  rejectBtn: {
    flexDirection: "row",
    alignItems: "center",
    gap: 4,
    paddingHorizontal: 16,
    paddingVertical: 10,
    borderRadius: 10,
    backgroundColor: "#FEE2E2",
  },
  rejectText: { fontSize: 14, fontWeight: "600", color: "#B91C1C" },
  acceptBtn: {
    flexDirection: "row",
    alignItems: "center",
    gap: 4,
    paddingHorizontal: 16,
    paddingVertical: 10,
    borderRadius: 10,
    backgroundColor: "#DC2626",
  },
  acceptText: { fontSize: 14, fontWeight: "600", color: "#FFF" },
  readyBtn: {
    flexDirection: "row",
    alignItems: "center",
    gap: 4,
    paddingHorizontal: 16,
    paddingVertical: 10,
    borderRadius: 10,
    backgroundColor: "#10B981",
  },
  readyText: { fontSize: 14, fontWeight: "600", color: "#FFF" },
});
