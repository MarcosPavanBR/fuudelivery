import Colors from "@/constants/Colors";
import Texts from "@/constants/Texts";
import { useAuthApi } from "@/contexts/AuthContext";
import { useNavigation } from "expo-router";
import React from "react";
import {
  ActivityIndicator,
  StyleSheet,
  Text,
  TouchableOpacity,
  View,
} from "react-native";

// Mini-card da entrega em andamento. Antes embutia o DeliveryMode COMPLETO
// (incluindo um segundo MapView interativo) dentro de 15% de altura — pesado
// e com toque aninhado. Agora é um resumo leve; o mapa fica no modal.
const MinimizableModal = () => {
  const nav = useNavigation();
  const { inWork } = useAuthApi();

  const current: any = Array.isArray(inWork.order)
    ? inWork.order[0]
    : inWork.order;

  const statusLabel =
    (Texts as Record<string, string>)[current?.deliveryman?.status] ??
    current?.deliveryman?.status ??
    "";

  const destination =
    current?.deliveryman?.status === "IN_ROUTE_DELIVERY"
      ? current?.user?.nome
      : current?.establishment?.name;

  return (
    <TouchableOpacity
      onPress={() => nav.navigate("delivery_mode" as never)}
      style={styles.container}
      activeOpacity={0.9}
    >
      <View style={styles.handle} />
      <View style={styles.row}>
        <View style={{ flex: 1 }}>
          <Text style={styles.status}>{statusLabel}</Text>
          <Text style={styles.dest} numberOfLines={1}>
            {destination ?? "—"}
          </Text>
        </View>
        <View style={styles.openBtn}>
          <Text style={styles.openTxt}>{Texts.maps}</Text>
        </View>
      </View>
    </TouchableOpacity>
  );
};

const styles = StyleSheet.create({
  container: {
    position: "absolute",
    bottom: 0,
    left: 0,
    right: 0,
    backgroundColor: Colors.light.white,
    borderTopLeftRadius: 20,
    borderTopRightRadius: 20,
    borderTopWidth: 1,
    borderTopColor: Colors.light.border,
    paddingHorizontal: 16,
    paddingTop: 8,
    paddingBottom: 14,
    shadowColor: "#000",
    shadowOffset: { width: 0, height: -3 },
    shadowOpacity: 0.12,
    shadowRadius: 6,
  },
  handle: {
    alignSelf: "center",
    width: 44,
    height: 5,
    borderRadius: 3,
    backgroundColor: Colors.light.secondaryText,
    marginBottom: 10,
  },
  row: {
    flexDirection: "row",
    alignItems: "center",
    gap: 12,
  },
  status: {
    fontSize: 13,
    fontWeight: "600",
    color: Colors.light.tint,
  },
  dest: {
    fontSize: 16,
    fontWeight: "700",
    color: Colors.light.text,
    marginTop: 2,
  },
  openBtn: {
    backgroundColor: Colors.light.tint,
    borderRadius: 8,
    paddingVertical: 10,
    paddingHorizontal: 16,
  },
  openTxt: {
    color: Colors.light.white,
    fontWeight: "600",
    fontSize: 14,
  },
});

export default MinimizableModal;
