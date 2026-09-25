/**
 * Tela principal do AppRestaurante — Lista de pedidos.
 *
 * Exibe pedidos recebidos com status, valor e horário.
 * O restaurante pode aceitar, rejeitar ou marcar como pronto.
 */
import React, { useState, useCallback } from "react";
import {
  View,
  Text,
  StyleSheet,
  FlatList,
  TouchableOpacity,
  RefreshControl,
  Alert,
} from "react-native";
import { useFocusEffect } from "expo-router/react-navigation";
import { Feather } from "@expo/vector-icons";
import api from "@/services/api";
import { useApi } from "@/contexts/ApiContext";

// Pedido como GET /orders/:establishmentId devolve (payload do pedido +
// _id, status e created_at). A tela usava outro formato (id, total,
// createdAt) e outros status (pending/preparing/ready) — nada batia, os
// botões nunca apareciam e a rota que ela chamava (PATCH
// /orders/:id/status) não existe.
interface Order {
  _id: string;
  status: string;
  created_at?: string;
  order_total?: number;
  user?: { nome?: string; phone?: string };
  cart?: { quantity?: number }[];
  deliveryman?: { id?: number; name?: string };
  scheduled_at?: string;
}

// ─── Máquina de estado do pedido (lógica pura, testada em
// __tests__/index.test.js) ───

const statusColors: Record<string, { bg: string; text: string; label: string }> = {
  AWAIT_APPROVE: { bg: "#FEF3C7", text: "#B45309", label: "Aguardando" },
  REQUEST_APPROVE: { bg: "#FEF3C7", text: "#B45309", label: "Aguardando" },
  SCHEDULED: { bg: "#E0E7FF", text: "#4338CA", label: "Agendado" },
  APPROVED: { bg: "#DBEAFE", text: "#1D4ED8", label: "Aceito" },
  PREPARING: { bg: "#E0F2FE", text: "#0369A1", label: "Em preparo" },
  DONE: { bg: "#D1FAE5", text: "#047857", label: "Pronto" },
  IN_ROUTE_DELIVERY: { bg: "#EDE9FE", text: "#6D28D9", label: "A caminho" },
  FINISHED: { bg: "#ECFDF5", text: "#047857", label: "Entregue" },
  DENIED: { bg: "#FEE2E2", text: "#B91C1C", label: "Recusado" },
  CANCELLED: { bg: "#FEE2E2", text: "#B91C1C", label: "Cancelado" },
};

// Badge de status com fallback pra status desconhecido — nunca deixa a
// tela sem cor/label mesmo se o backend mandar um status novo.
export function resolveOrderStatus(status: string): { bg: string; text: string; label: string } {
  return statusColors[status] || { bg: "#F3F4F6", text: "#374151", label: status };
}

export type OrderAction = "accept" | "reject" | "start" | "ready" | "cancel" | "dispatched" | "delivered";

// Status para onde cada ação leva — espelha validTransitions de
// Backend/orders_api/app/handlers/orders.go.
export const ACTION_TO_STATUS: Record<OrderAction, string> = {
  accept: "APPROVED",
  reject: "DENIED",
  start: "PREPARING",
  ready: "DONE",
  cancel: "CANCELLED",
  dispatched: "IN_ROUTE_DELIVERY",
  delivered: "FINISHED",
};

// O que a loja pode fazer em cada status. Com entregador atribuído, "saiu"
// e "entregue" são do app do entregador.
export function getAvailableActions(status: string, hasCourier = false): OrderAction[] {
  switch (status) {
    case "AWAIT_APPROVE":
    case "REQUEST_APPROVE":
    case "SCHEDULED":
      return ["accept", "reject"];
    case "APPROVED":
      return ["start", "cancel"];
    case "PREPARING":
      return ["ready", "cancel"];
    case "DONE":
      return hasCourier ? [] : ["dispatched"];
    case "IN_ROUTE_DELIVERY":
      return hasCourier ? [] : ["delivered"];
    default:
      return [];
  }
}

const FINAL = new Set(["FINISHED", "DENIED", "CANCELLED"]);

// A lista mostra os pedidos em andamento (a API devolve até 500, com os
// encerrados), os que esperam a loja primeiro e, dentro de cada grupo, os
// mais antigos primeiro — é a ordem de atendimento.
export function activeOrders<T extends { status: string; created_at?: string }>(orders: T[]): T[] {
  const waiting = (o: T) => (o.status === "AWAIT_APPROVE" || o.status === "REQUEST_APPROVE" ? 0 : 1);
  const time = (o: T) => Date.parse(o.created_at || "") || 0;
  return orders
    .filter((o) => !FINAL.has(o.status))
    .sort((a, b) => waiting(a) - waiting(b) || time(a) - time(b));
}

export function shortOrderId(id: string): string {
  return id ? `#${String(id).slice(-4).toUpperCase()}` : "";
}

export function elapsedLabel(createdAt?: string, now = Date.now()): string {
  const t = Date.parse(createdAt || "");
  if (Number.isNaN(t)) return "";
  const min = Math.max(0, Math.floor((now - t) / 60000));
  if (min < 1) return "agora";
  if (min < 60) return `há ${min} min`;
  const h = Math.floor(min / 60);
  return min % 60 ? `há ${h} h ${min % 60} min` : `há ${h} h`;
}

// Atualização otimista da lista local após confirmar no backend — só o
// pedido alvo muda; os demais saem exatamente iguais.
export function applyStatusUpdate<T extends { _id: string; status: string }>(
  orders: T[],
  orderId: string,
  newStatus: string
): T[] {
  return orders.map((o) => (o._id === orderId ? { ...o, status: newStatus } : o));
}

const ACTION_LABEL: Record<OrderAction, string> = {
  accept: "Aceitar",
  reject: "Recusar",
  start: "Iniciar preparo",
  ready: "Pronto",
  cancel: "Cancelar",
  dispatched: "Saiu p/ entrega",
  delivered: "Entregue",
};
const NEEDS_CONFIRM: Partial<Record<OrderAction, string>> = {
  reject: "Recusar este pedido? O cliente será avisado.",
  cancel: "Cancelar este pedido?",
};
const DANGER = new Set<OrderAction>(["reject", "cancel"]);

export default function OrdersScreen() {
  const { getUserData } = useApi();
  const user = getUserData();
  const [orders, setOrders] = useState<Order[]>([]);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [loadError, setLoadError] = useState(false);
  const [now, setNow] = useState(Date.now());

  const fetchOrders = async () => {
    try {
      const establishmentId = user?.establishment_id;
      if (!establishmentId) return;
      const resp = await api.get(`/orders/${establishmentId}`);
      setOrders(activeOrders(resp.data || []));
      setLoadError(false);
    } catch (e) {
      setLoadError(true);
    }
    setLoading(false);
    setNow(Date.now());
  };

  useFocusEffect(
    useCallback(() => {
      fetchOrders();
      // Sem WebSocket no app: recarrega a cada 20 s enquanto a tela está
      // aberta, para pedido novo não ficar esperando um "puxar para baixo".
      const t = setInterval(fetchOrders, 20000);
      return () => clearInterval(t);
    }, [user?.establishment_id])
  );

  const onRefresh = async () => {
    setRefreshing(true);
    await fetchOrders();
    setRefreshing(false);
  };

  const updateStatus = async (orderId: string, newStatus: string) => {
    try {
      await api.put("/orders/status", { id: orderId, status: newStatus });
      setOrders((prev) => activeOrders(applyStatusUpdate(prev, orderId, newStatus)));
    } catch (e: any) {
      Alert.alert("Erro", e?.response?.data?.error || "Falha ao atualizar o pedido.");
      fetchOrders();
    }
  };

  const runAction = (order: Order, action: OrderAction) => {
    const to = ACTION_TO_STATUS[action];
    const question = NEEDS_CONFIRM[action];
    if (!question) {
      updateStatus(order._id, to);
      return;
    }
    Alert.alert(ACTION_LABEL[action], question, [
      { text: "Voltar", style: "cancel" },
      { text: ACTION_LABEL[action], style: "destructive", onPress: () => updateStatus(order._id, to) },
    ]);
  };

  const renderOrder = ({ item }: { item: Order }) => {
    const status = resolveOrderStatus(item.status);
    const hasCourier = !!(item.deliveryman && Number(item.deliveryman.id) > 0);
    const actions = getAvailableActions(item.status, hasCourier);
    const customerName = item.user?.nome || "Cliente";
    const time = item.created_at
      ? new Date(item.created_at).toLocaleTimeString("pt-BR", { hour: "2-digit", minute: "2-digit" })
      : "-";
    const itemsCount = (item.cart || []).reduce((s, c) => s + (c.quantity || 0), 0);
    const total = (item.order_total || 0).toFixed(2).replace(".", ",");

    return (
      <View style={styles.orderCard}>
        <View style={styles.orderHeader}>
          <Text style={styles.orderId}>{shortOrderId(item._id)}</Text>
          <View style={[styles.statusBadge, { backgroundColor: status.bg }]}>
            <Text style={[styles.statusText, { color: status.text }]}>{status.label}</Text>
          </View>
        </View>

        <Text style={styles.customerName}>{customerName}</Text>
        <Text style={styles.orderTime}>
          🕐 {time} · {elapsedLabel(item.created_at, now)} · {itemsCount} {itemsCount === 1 ? "item" : "itens"}
        </Text>
        {hasCourier && <Text style={styles.orderTime}>🛵 {item.deliveryman?.name || "Entregador"}</Text>}
        <Text style={styles.orderTotal}>R$ {total}</Text>

        {actions.length > 0 && (
          <View style={styles.actions}>
            {actions.map((a) => (
              <TouchableOpacity
                key={a}
                style={DANGER.has(a) ? styles.rejectBtn : styles.acceptBtn}
                onPress={() => runAction(item, a)}
              >
                <Feather name={DANGER.has(a) ? "x" : "check"} size={16} color={DANGER.has(a) ? "#B91C1C" : "#FFF"} />
                <Text style={DANGER.has(a) ? styles.rejectText : styles.acceptText}>{ACTION_LABEL[a]}</Text>
              </TouchableOpacity>
            ))}
          </View>
        )}
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
      ) : loadError && orders.length === 0 ? (
        <View style={styles.center}>
          <Text style={styles.emptyTitle}>Não foi possível carregar</Text>
          <TouchableOpacity style={styles.acceptBtn} onPress={onRefresh}>
            <Text style={styles.acceptText}>Tentar novamente</Text>
          </TouchableOpacity>
        </View>
      ) : orders.length === 0 ? (
        <View style={styles.center}>
          <Text style={styles.emptyEmoji}>📦</Text>
          <Text style={styles.emptyTitle}>Nenhum pedido em andamento</Text>
          <Text style={styles.emptySubtitle}>Os pedidos novos aparecem aqui</Text>
        </View>
      ) : (
        <FlatList
          data={orders}
          keyExtractor={(item) => item._id}
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
