import React, { useRef } from "react";
import { Animated, StyleSheet, Text, View } from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";

interface AnimatedHeaderProps {
  scrollY: Animated.Value;
  title: string;
  subtitle?: string;
}

const AnimatedHeader: React.FC<AnimatedHeaderProps> = ({ scrollY, title, subtitle }) => {
  const insets = useSafeAreaInsets();

  const headerHeight = scrollY.interpolate({
    inputRange: [0, 100],
    outputRange: [120, 60 + insets.top],
    extrapolate: "clamp",
  });

  const titleScale = scrollY.interpolate({
    inputRange: [0, 100],
    outputRange: [1, 0.8],
    extrapolate: "clamp",
  });

  const titleTranslateY = scrollY.interpolate({
    inputRange: [0, 100],
    outputRange: [0, -20],
    extrapolate: "clamp",
  });

  return (
    <Animated.View style={[styles.container, { height: headerHeight, paddingTop: insets.top }]}>
      <Animated.View style={{ transform: [{ scale: titleScale }, { translateY: titleTranslateY }] }}>
        <Text style={styles.title}>{title}</Text>
        {subtitle && <Text style={styles.subtitle}>{subtitle}</Text>}
      </Animated.View>
    </Animated.View>
  );
};

const styles = StyleSheet.create({
  container: {
    backgroundColor: "#F97316",
    justifyContent: "flex-end",
    alignItems: "center",
    paddingBottom: 10,
  },
  title: {
    fontSize: 22,
    fontWeight: "bold",
    color: "#FFF",
    textAlign: "center",
  },
  subtitle: {
    fontSize: 14,
    color: "rgba(255,255,255,0.8)",
    textAlign: "center",
    marginTop: 4,
  },
});

export default AnimatedHeader;
