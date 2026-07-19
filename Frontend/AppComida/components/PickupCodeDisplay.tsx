import React from "react";
import { View, Text, TouchableOpacity, StyleSheet } from "react-native";
import * as Clipboard from "expo-clipboard";
import { Ionicons } from "@expo/vector-icons";
import Colors from "@/constants/Colors";

interface PickupCodeDisplayProps {
  code: string;
}

const PickupCodeDisplay: React.FC<PickupCodeDisplayProps> = ({ code }) => {
  const copyToClipboard = async () => {
    await Clipboard.setStringAsync(code);
  };

  return (
    <View style={styles.container}>
      <Text style={styles.label}>Código de Retirada</Text>
      <Text style={styles.code}>{code}</Text>
      <TouchableOpacity style={styles.copyBtn} onPress={copyToClipboard}>
        <Ionicons name="copy-outline" size={16} color="#F97316" />
        <Text style={styles.copyText}>Copiar</Text>
      </TouchableOpacity>
    </View>
  );
};

const styles = StyleSheet.create({
  container: { alignItems: "center", padding: 12, backgroundColor: "#FFF7ED", borderRadius: 8, marginTop: 8 },
  label: { fontSize: 12, color: "#9A3412", fontWeight: "500" },
  code: { fontSize: 32, fontWeight: "800", letterSpacing: 8, color: "#C2410C", marginVertical: 4 },
  copyBtn: { flexDirection: "row", alignItems: "center", gap: 4 },
  copyText: { color: "#F97316", fontSize: 13, fontWeight: "500" },
});

export default PickupCodeDisplay;
