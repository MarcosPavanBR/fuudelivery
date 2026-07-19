import React from "react";
import { View, Text, StyleSheet } from "react-native";
import { Ionicons } from "@expo/vector-icons";

interface OrderTrackingProps {
  currentStatus: string;
}

const STATUS_FLOW = [
  { key: "AWAIT_APPROVE", label: "Aguardando", icon: "time-outline" },
  { key: "PREPARING", label: "Preparando", icon: "restaurant-outline" },
  { key: "DONE", label: "Pronto", icon: "checkmark-circle-outline" },
  { key: "IN_ROUTE", label: "Saiu para entrega", icon: "bicycle-outline" },
  { key: "FINISHED", label: "Entregue", icon: "home-outline" },
];

const OrderTracking: React.FC<OrderTrackingProps> = ({ currentStatus }) => {
  const currentIndex = STATUS_FLOW.findIndex(s => s.key === currentStatus);

  return (
    <View style={styles.container}>
      {STATUS_FLOW.map((step, index) => {
        const isCompleted = index <= currentIndex;
        const isCurrent = index === currentIndex;

        return (
          <View key={step.key} style={styles.stepContainer}>
            <View style={styles.stepRow}>
              <View style={[styles.dot, isCompleted && styles.dotCompleted, isCurrent && styles.dotCurrent]}>
                <Ionicons
                  name={step.icon as any}
                  size={16}
                  color={isCompleted ? "#FFF" : "#999"}
                />
              </View>
              {index < STATUS_FLOW.length - 1 && (
                <View style={[styles.line, isCompleted && styles.lineCompleted]} />
              )}
            </View>
            <Text style={[styles.label, isCompleted && styles.labelCompleted]}>
              {step.label}
            </Text>
          </View>
        );
      })}
    </View>
  );
};

const styles = StyleSheet.create({
  container: {
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "flex-start",
    paddingVertical: 16,
    paddingHorizontal: 8,
  },
  stepContainer: {
    alignItems: "center",
    flex: 1,
  },
  stepRow: {
    flexDirection: "row",
    alignItems: "center",
  },
  dot: {
    width: 32,
    height: 32,
    borderRadius: 16,
    backgroundColor: "#EEE",
    justifyContent: "center",
    alignItems: "center",
  },
  dotCompleted: {
    backgroundColor: "#22C55E",
  },
  dotCurrent: {
    backgroundColor: "#F97316",
    transform: [{ scale: 1.15 }],
  },
  line: {
    flex: 1,
    height: 3,
    backgroundColor: "#DDD",
    marginHorizontal: 4,
  },
  lineCompleted: {
    backgroundColor: "#22C55E",
  },
  label: {
    fontSize: 10,
    color: "#999",
    marginTop: 6,
    textAlign: "center",
  },
  labelCompleted: {
    color: "#333",
    fontWeight: "600",
  },
});

export default OrderTracking;
