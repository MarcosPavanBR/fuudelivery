import React from "react";
import { StyleSheet, Text, View } from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { FontAwesome } from "@expo/vector-icons";
import Colors from "@/constants/Colors";

export default function Notifications() {
  const insets = useSafeAreaInsets();

  return (
    <View style={[styles.container, { paddingTop: insets.top + 40 }]}>
      <FontAwesome name="bell-o" size={40} color={Colors.light.tabIconDefault} />
      <Text style={styles.title}>Nenhuma notificação</Text>
      <Text style={styles.subtitle}>
        Novidades sobre suas entregas aparecerão aqui.
      </Text>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: Colors.light.background,
    alignItems: "center",
    padding: 20,
  },
  title: {
    fontSize: 17,
    fontWeight: "700",
    color: Colors.light.text,
    marginTop: 16,
  },
  subtitle: {
    fontSize: 13,
    color: Colors.light.secondaryText,
    marginTop: 6,
    textAlign: "center",
  },
});
