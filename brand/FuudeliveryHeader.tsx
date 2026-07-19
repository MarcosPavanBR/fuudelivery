import React from "react";
import { View, Text, Image, StyleSheet } from "react-native";
import Colors from "@/constants/Colors";

interface FuudeliveryHeaderProps {
  variant?: "default" | "compact" | "splash";
}

export default function FuudeliveryHeader({ variant = "default" }: FuudeliveryHeaderProps) {
  if (variant === "splash") {
    return (
      <View style={splashStyles.container}>
        <View style={splashStyles.logoContainer}>
          <View style={splashStyles.iconRow}>
            <View style={splashStyles.box}>
              <View style={splashStyles.boxLid} />
              <View style={splashStyles.boxLine} />
              <View style={splashStyles.boxLineShort} />
            </View>
          </View>
          <Text style={splashStyles.title}>
            <Text style={splashStyles.fuud}>FUUD</Text>
            <Text style={splashStyles.elivery}>ELIVERY</Text>
          </Text>
          <View style={splashStyles.divider} />
          <Text style={splashStyles.tagline}>SABOR · RAPIDEZ · CONFIANÇA</Text>
        </View>
      </View>
    );
  }

  return (
    <View style={[styles.container, variant === "compact" && styles.compact]}>
      <View style={styles.logoRow}>
        <View style={styles.iconBox}>
          <View style={styles.boxIcon}>
            <View style={styles.lid} />
            <View style={styles.line1} />
            <View style={styles.line2} />
          </View>
        </View>
        <Text style={[styles.title, variant === "compact" && styles.titleCompact]}>
          <Text style={styles.fuud}>FUUD</Text>
          <Text style={styles.elivery}>ELIVERY</Text>
        </Text>
      </View>
      {variant !== "compact" && <Text style={styles.tagline}>SABOR · RAPIDEZ · CONFIANÇA</Text>}
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    alignItems: "center",
    paddingVertical: 12,
  },
  compact: {
    paddingVertical: 6,
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "center",
    paddingHorizontal: 16,
  },
  logoRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: 8,
  },
  iconBox: {
    width: 36,
    height: 36,
    alignItems: "center",
    justifyContent: "center",
  },
  boxIcon: {
    width: 28,
    height: 24,
    backgroundColor: Colors.light.primary,
    borderRadius: 4,
    alignItems: "center",
    justifyContent: "center",
    paddingTop: 2,
  },
  lid: {
    width: 20,
    height: 4,
    backgroundColor: Colors.light.primaryDark,
    borderRadius: 1,
    marginBottom: 3,
  },
  line1: {
    width: 18,
    height: 2.5,
    backgroundColor: Colors.light.white,
    borderRadius: 1,
    marginBottom: 3,
  },
  line2: {
    width: 14,
    height: 2.5,
    backgroundColor: Colors.light.white,
    borderRadius: 1,
  },
  title: {
    fontSize: 22,
    letterSpacing: 0.5,
  },
  titleCompact: {
    fontSize: 18,
  },
  fuud: {
    fontWeight: "900",
    color: Colors.light.primary,
  },
  elivery: {
    fontWeight: "700",
    color: Colors.light.text,
  },
  tagline: {
    fontSize: 10,
    fontWeight: "500",
    color: Colors.light.secondaryText,
    letterSpacing: 4,
    marginTop: 2,
  },
});

const splashStyles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: Colors.light.primary,
    alignItems: "center",
    justifyContent: "center",
  },
  logoContainer: {
    alignItems: "center",
  },
  iconRow: {
    marginBottom: 24,
  },
  box: {
    width: 80,
    height: 65,
    backgroundColor: Colors.light.white,
    borderRadius: 12,
    alignItems: "center",
    justifyContent: "center",
    shadowColor: "#000",
    shadowOffset: { width: 0, height: 4 },
    shadowOpacity: 0.2,
    shadowRadius: 8,
    elevation: 8,
  },
  boxLid: {
    width: 60,
    height: 8,
    backgroundColor: Colors.light.primaryDark,
    borderRadius: 2,
    marginBottom: 8,
  },
  boxLine: {
    width: 48,
    height: 5,
    backgroundColor: Colors.light.primary,
    borderRadius: 2,
    marginBottom: 6,
  },
  boxLineShort: {
    width: 32,
    height: 5,
    backgroundColor: Colors.light.primary,
    borderRadius: 2,
    opacity: 0.6,
  },
  title: {
    fontSize: 48,
    letterSpacing: 1,
  },
  fuud: {
    fontWeight: "900",
    color: Colors.light.white,
  },
  elivery: {
    fontWeight: "700",
    color: Colors.light.white,
    opacity: 0.9,
  },
  divider: {
    width: 60,
    height: 4,
    backgroundColor: Colors.light.secondary,
    borderRadius: 2,
    marginVertical: 12,
    alignSelf: "center",
  },
  tagline: {
    fontSize: 14,
    fontWeight: "500",
    color: Colors.light.white,
    opacity: 0.7,
    letterSpacing: 6,
  },
});
