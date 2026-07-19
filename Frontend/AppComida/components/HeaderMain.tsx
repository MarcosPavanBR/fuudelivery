import React from "react";
import { View, Text, StyleSheet, TouchableOpacity } from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { useRouter, useNavigation } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import Colors from "@/constants/Colors";
import { useCartApi } from "@/contexts/ApiCartContext";

interface HeaderMainProps {
  hiddenOpen?: boolean;
  hiddenBack?: boolean;
  title?: string;
}

export default function HeaderMain({
  hiddenOpen = false,
  hiddenBack = false,
  title,
}: HeaderMainProps) {
  const insets = useSafeAreaInsets();
  const router = useRouter();
  const navigation = useNavigation();
  const { establishment } = useCartApi();

  return (
    <View style={[styles.container, { paddingTop: insets.top + 8 }]}>
      <View style={styles.row}>
        <View style={styles.left}>
          {!hiddenBack && (
            <TouchableOpacity
              onPress={() => navigation.canGoBack() ? navigation.goBack() : router.push("/")}
              style={styles.backButton}
            >
              <Ionicons name="chevron-back" size={24} color={Colors.light.text} />
            </TouchableOpacity>
          )}
        </View>

        <View style={styles.center}>
          <View style={styles.logoRow}>
            <View style={styles.logoBox}>
              <View style={styles.logoLid} />
              <View style={styles.logoLine} />
              <View style={styles.logoLineShort} />
            </View>
            <Text style={styles.logoText}>
              <Text style={styles.logoFuud}>FUUD</Text>
              <Text style={styles.logoElivery}>ELIVERY</Text>
            </Text>
          </View>
          {title && <Text style={styles.title}>{title}</Text>}
        </View>

        <View style={styles.right}>
          {!hiddenOpen && (
            <TouchableOpacity
              onPress={() => router.push("/cart")}
              style={styles.cartButton}
            >
              <Ionicons name="cart-outline" size={24} color={Colors.light.primary} />
            </TouchableOpacity>
          )}
        </View>
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    paddingHorizontal: 16,
    paddingBottom: 8,
    backgroundColor: Colors.light.white,
    borderBottomWidth: 1,
    borderBottomColor: Colors.light.border,
  },
  row: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    height: 44,
  },
  left: {
    width: 44,
    alignItems: "flex-start",
  },
  center: {
    flex: 1,
    alignItems: "center",
  },
  right: {
    width: 44,
    alignItems: "flex-end",
  },
  backButton: {
    width: 44,
    height: 44,
    alignItems: "center",
    justifyContent: "center",
    marginLeft: -12,
  },
  cartButton: {
    width: 44,
    height: 44,
    alignItems: "center",
    justifyContent: "center",
    marginRight: -12,
  },
  logoRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: 6,
  },
  logoBox: {
    width: 26,
    height: 22,
    backgroundColor: Colors.light.primary,
    borderRadius: 4,
    alignItems: "center",
    justifyContent: "center",
    paddingTop: 2,
  },
  logoLid: {
    width: 18,
    height: 4,
    backgroundColor: Colors.light.primaryDark,
    borderRadius: 1,
    marginBottom: 2,
  },
  logoLine: {
    width: 16,
    height: 2.5,
    backgroundColor: Colors.light.white,
    borderRadius: 1,
    marginBottom: 2,
  },
  logoLineShort: {
    width: 12,
    height: 2.5,
    backgroundColor: Colors.light.white,
    borderRadius: 1,
    opacity: 0.7,
  },
  logoText: {
    fontSize: 20,
    letterSpacing: 0.5,
  },
  logoFuud: {
    fontWeight: "900",
    color: Colors.light.primary,
  },
  logoElivery: {
    fontWeight: "700",
    color: Colors.light.text,
  },
  title: {
    fontSize: 13,
    fontWeight: "500",
    color: Colors.light.secondaryText,
    marginTop: 2,
  },
});
