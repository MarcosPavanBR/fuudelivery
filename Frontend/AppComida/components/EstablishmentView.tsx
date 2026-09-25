import React, { useEffect, useState } from "react";
import { View, Text, Image, StyleSheet, TouchableOpacity } from "react-native";
import Colors from "@/constants/Colors";
import helpers from "@/helpers/helpers";
import Texts from "@/constants/Texts";
import { closedLabel, isStoreOpen, ratingLabel } from "@/helpers/storeStatus";

const EstablishmentView = ({
  item,
  onPress,
}: {
  item: any;
  onPress: () => void;
}) => {
  const [distance, setDistance] = useState<number | null>(null);
  async function init() {
    try {
      const result = await helpers.calcularDistancia(item.lat, item.long);
      setDistance(result);
    } catch (e) {
      // Sem GPS/permissão: card aparece sem distância.
    }
  }
  useEffect(() => {
    init();
  }, []);

  return (
    <TouchableOpacity
      onPress={onPress}
      style={[styles.establishmentContainer, !isStoreOpen(item) && { opacity: 0.55 }]}
    >
      {item.image ? (
        <Image source={{ uri: item.image }} style={styles.establishmentImage} />
      ) : null}
      <View style={styles.establishmentDetails}>
        {item.is_sponsored ? (
          // Anúncio identificado como tal (CDC art. 36).
          <Text style={styles.sponsored}>Patrocinado</Text>
        ) : null}
        <Text style={styles.establishmentName}>{item.name}</Text>
        <Text style={styles.description} numberOfLines={2}>
          {item.description}
        </Text>
        <Text style={styles.description}>
          <Text style={styles.rating}>{ratingLabel(item)}</Text>
          {distance ? `  ·  ${distance.toFixed(1)} ${Texts.km}` : ""}
        </Text>
        {closedLabel(item) ? <Text style={styles.closed}>{closedLabel(item)}</Text> : null}
      </View>
    </TouchableOpacity>
  );
};

const styles = StyleSheet.create({
  establishmentContainer: {
    marginBottom: 10,
    flexDirection: "row-reverse",
    width: "100%",

    borderBottomColor: Colors.light.tabIconDefault,
    borderBottomWidth: 1,
    paddingBottom: 10,
    alignContent: "center",
    alignItems: "center",
  },
  establishmentDetails: {
    justifyContent: "space-between",
    flex: 1,
    padding: 10,
    gap: 5,
  },
  establishmentImage: {
    width: 80,
    height: 80,
    marginBottom: 2,
    borderWidth: 1,
    borderColor: Colors.light.tabIconDefault,
    marginRight: 5,
    borderRadius: 5,
  },
  establishmentName: {
    fontSize: 16,
    fontWeight: "400",
    color: Colors.light.text,
  },

  sponsored: {
    alignSelf: "flex-start",
    fontSize: 10.5,
    fontWeight: "600",
    color: Colors.light.secondaryText,
    borderWidth: 1,
    borderColor: Colors.light.tabIconDefault,
    borderRadius: 4,
    paddingHorizontal: 5,
    paddingVertical: 1,
  },
  rating: {
    color: "#B45309",
    fontWeight: "600",
  },
  closed: {
    fontSize: 12.5,
    fontWeight: "600",
    color: "#B91C1C",
  },
  description: {
    width: "95%",
    color: Colors.light.secondaryText,
    fontSize: 12.5,
    textAlign: "justify",
  },
});

export default EstablishmentView;
