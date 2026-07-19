import React, { useState } from "react";
import { TouchableOpacity, Text, StyleSheet, Alert } from "react-native";
import { Ionicons } from "@expo/vector-icons";
import Colors from "@/constants/Colors";

interface ReorderButtonProps {
  cart: any[];
  onReorder: (items: any[]) => void;
}

const ReorderButton: React.FC<ReorderButtonProps> = ({ cart, onReorder }) => {
  const handleReorder = () => {
    Alert.alert(
      "Repetir Pedido",
      "Adicionar todos os itens deste pedido ao carrinho?",
      [
        { text: "Cancelar", style: "cancel" },
        { text: "Sim", onPress: () => onReorder(cart) },
      ]
    );
  };

  return (
    <TouchableOpacity style={styles.button} onPress={handleReorder}>
      <Ionicons name="refresh" size={16} color="#FFF" />
      <Text style={styles.text}>Repetir Pedido</Text>
    </TouchableOpacity>
  );
};

const styles = StyleSheet.create({
  button: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "center",
    gap: 6,
    backgroundColor: Colors.light.tint,
    paddingVertical: 10,
    paddingHorizontal: 16,
    borderRadius: 6,
    marginTop: 10,
  },
  text: { color: "#FFF", fontWeight: "600", fontSize: 14 },
});

export default ReorderButton;
