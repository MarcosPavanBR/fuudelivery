import React, { useState } from "react";
import {
  ActivityIndicator,
  Alert,
  Image,
  StyleSheet,
  Text,
  TouchableOpacity,
  View,
} from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import Colors from "@/constants/Colors";
import { useAuthApi } from "@/contexts/AuthContext";

export default function Perfil() {
  const insets = useSafeAreaInsets();
  const { user, logout, inWork } = useAuthApi();
  const [loading, setLoading] = useState(false);

  const handleLogout = () => {
    if (inWork.status) {
      Alert.alert(
        "",
        "Você possui uma entrega em andamento. Finalize-a antes de sair."
      );
      return;
    }
    Alert.alert("Sair", "Deseja realmente sair da sua conta?", [
      { text: "Cancelar", style: "cancel" },
      {
        text: "Sair",
        style: "destructive",
        onPress: async () => {
          setLoading(true);
          try {
            await logout();
          } finally {
            setLoading(false);
          }
        },
      },
    ]);
  };

  return (
    <View style={[styles.container, { paddingTop: insets.top + 20 }]}>
      <Image
        source={require("../assets/images/deliveryman_happy.png")}
        style={styles.avatar}
      />
      <Text style={styles.name}>{user?.name ?? "Entregador"}</Text>
      <Text style={styles.detail}>{user?.email ?? ""}</Text>
      {user?.phone ? <Text style={styles.detail}>{user.phone}</Text> : null}

      <TouchableOpacity
        style={styles.logoutButton}
        onPress={handleLogout}
        disabled={loading}
      >
        {loading ? (
          <ActivityIndicator color={Colors.light.white} />
        ) : (
          <Text style={styles.logoutText}>Sair</Text>
        )}
      </TouchableOpacity>
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
  avatar: {
    width: 96,
    height: 96,
    borderRadius: 48,
    marginBottom: 16,
  },
  name: {
    fontSize: 20,
    fontWeight: "700",
    color: Colors.light.text,
  },
  detail: {
    fontSize: 14,
    color: Colors.light.secondaryText,
    marginTop: 4,
  },
  logoutButton: {
    marginTop: 32,
    backgroundColor: Colors.light.tint,
    paddingVertical: 12,
    paddingHorizontal: 40,
    borderRadius: 8,
  },
  logoutText: {
    color: Colors.light.white,
    fontWeight: "600",
    fontSize: 15,
  },
});
