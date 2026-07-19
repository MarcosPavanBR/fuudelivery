import React, { useEffect } from "react";
import { View, Text, StyleSheet, Animated } from "react-native";
import Colors from "@/constants/Colors";
import { useRouter } from "expo-router";

export default function SplashScreen() {
  const router = useRouter();
  const scaleAnim = React.useRef(new Animated.Value(0.8)).current;
  const opacityAnim = React.useRef(new Animated.Value(0)).current;
  const taglineAnim = React.useRef(new Animated.Value(0)).current;

  useEffect(() => {
    Animated.sequence([
      Animated.parallel([
        Animated.timing(scaleAnim, {
          toValue: 1,
          duration: 800,
          useNativeDriver: true,
        }),
        Animated.timing(opacityAnim, {
          toValue: 1,
          duration: 600,
          useNativeDriver: true,
        }),
      ]),
      Animated.timing(taglineAnim, {
        toValue: 1,
        duration: 400,
        useNativeDriver: true,
      }),
    ]).start();

    const timer = setTimeout(() => {
      router.replace("/");
    }, 2500);

    return () => clearTimeout(timer);
  }, []);

  return (
    <View style={styles.container}>
      <Animated.View
        style={[
          styles.logoContainer,
          { opacity: opacityAnim, transform: [{ scale: scaleAnim }] },
        ]}
      >
        <View style={styles.box}>
          <View style={styles.lid} />
          <View style={styles.line1} />
          <View style={styles.line2} />
          <View style={styles.star}>
            <Text style={styles.starText}>★</Text>
          </View>
        </View>

        <Text style={styles.title}>
          <Text style={styles.fuud}>FUUD</Text>
          <Text style={styles.elivery}>ELIVERY</Text>
        </Text>

        <View style={styles.divider} />
      </Animated.View>

      <Animated.Text style={[styles.tagline, { opacity: taglineAnim }]}>
        SABOR · RAPIDEZ · CONFIANÇA
      </Animated.Text>

      <Text style={styles.version}>v1.0.0</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: Colors.light.primary,
    alignItems: "center",
    justifyContent: "center",
  },
  logoContainer: {
    alignItems: "center",
  },
  box: {
    width: 96,
    height: 78,
    backgroundColor: Colors.light.white,
    borderRadius: 16,
    alignItems: "center",
    justifyContent: "center",
    marginBottom: 28,
    shadowColor: "#000",
    shadowOffset: { width: 0, height: 6 },
    shadowOpacity: 0.3,
    shadowRadius: 12,
    elevation: 12,
  },
  lid: {
    width: 72,
    height: 10,
    backgroundColor: Colors.light.primaryDark,
    borderRadius: 3,
    marginBottom: 10,
  },
  line1: {
    width: 56,
    height: 6,
    backgroundColor: Colors.light.primary,
    borderRadius: 2,
    marginBottom: 8,
  },
  line2: {
    width: 38,
    height: 6,
    backgroundColor: Colors.light.primary,
    borderRadius: 2,
    opacity: 0.6,
  },
  star: {
    position: "absolute",
    top: -8,
    right: -8,
    width: 28,
    height: 28,
    borderRadius: 14,
    backgroundColor: Colors.light.secondary,
    alignItems: "center",
    justifyContent: "center",
  },
  starText: {
    fontSize: 16,
    color: Colors.light.white,
  },
  title: {
    fontSize: 52,
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
    width: 72,
    height: 4,
    backgroundColor: Colors.light.secondary,
    borderRadius: 2,
    marginTop: 16,
    alignSelf: "center",
  },
  tagline: {
    position: "absolute",
    bottom: 80,
    fontSize: 14,
    fontWeight: "500",
    color: Colors.light.white,
    opacity: 0.7,
    letterSpacing: 6,
  },
  version: {
    position: "absolute",
    bottom: 40,
    fontSize: 12,
    color: Colors.light.white,
    opacity: 0.4,
  },
});
