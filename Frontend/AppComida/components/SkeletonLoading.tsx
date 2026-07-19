import React, { useEffect, useRef } from "react";
import { View, Animated, StyleSheet } from "react-native";

interface SkeletonLoadingProps {
  width?: number | string;
  height?: number;
  borderRadius?: number;
  style?: any;
}

const SkeletonLoading: React.FC<SkeletonLoadingProps> = ({
  width = "100%",
  height = 20,
  borderRadius = 4,
  style,
}) => {
  const opacity = useRef(new Animated.Value(0.3)).current;

  useEffect(() => {
    const animation = Animated.loop(
      Animated.sequence([
        Animated.timing(opacity, {
          toValue: 1,
          duration: 800,
          useNativeDriver: true,
        }),
        Animated.timing(opacity, {
          toValue: 0.3,
          duration: 800,
          useNativeDriver: true,
        }),
      ])
    );
    animation.start();
    return () => animation.stop();
  }, []);

  return (
    <Animated.View
      style={[
        styles.skeleton,
        { width: width as any, height, borderRadius, opacity },
        style,
      ]}
    />
  );
};

const styles = StyleSheet.create({
  skeleton: {
    backgroundColor: "#E1E9EE",
  },
});

export default SkeletonLoading;
