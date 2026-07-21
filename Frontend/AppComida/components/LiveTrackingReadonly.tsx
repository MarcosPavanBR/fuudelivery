import React, { useEffect, useState, useRef } from "react";
import { View, Text, StyleSheet } from "react-native";
import MapView, { Marker } from "react-native-maps";
import AsyncStorage from "@react-native-async-storage/async-storage";
import Colors from "@/constants/Colors";
import Strings from "@/constants/Strings";

interface LiveTrackingReadonlyProps {
  orderId: string;
  originLat?: number;
  originLng?: number;
  destinationLat?: number;
  destinationLng?: number;
}

interface DeliveryLocation {
  lat: number;
  lng: number;
  order_id: string;
  timestamp: number;
}

export default function LiveTrackingReadonly({
  orderId,
  originLat,
  originLng,
  destinationLat,
  destinationLng,
}: LiveTrackingReadonlyProps) {
  const [deliveryLocation, setDeliveryLocation] =
    useState<DeliveryLocation | null>(null);
  const [connected, setConnected] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const mapRef = useRef<MapView>(null);

  useEffect(() => {
    if (!orderId) return;

    const connectWebSocket = async () => {
      try {
        const token = await AsyncStorage.getItem(Strings.token_jwt);
        if (!token) {
          setError("Token não encontrado");
          return;
        }

        const wsUrl =
          process.env.EXPO_PUBLIC_WS_URL ||
          "wss://fuudelivery-api-8y6l.onrender.com";
        const ws = new WebSocket(
          `${wsUrl}/ws/delivery/${orderId}?token=${token}`
        );

        ws.onopen = () => {
          setConnected(true);
          setError(null);
        };

        ws.onmessage = (event) => {
          try {
            const data = JSON.parse(event.data);
            if (data.type === "location" && data.payload) {
              setDeliveryLocation(data.payload);
            }
          } catch (e) {
            console.log("Error parsing WS message:", e);
          }
        };

        ws.onerror = () => {
          setError("Erro na conexão");
          setConnected(false);
        };

        ws.onclose = () => {
          setConnected(false);
          setTimeout(() => connectWebSocket(), 5000);
        };

        wsRef.current = ws;
      } catch (e) {
        setError("Erro ao conectar");
      }
    };

    connectWebSocket();

    return () => {
      if (wsRef.current) {
        wsRef.current.close();
      }
    };
  }, [orderId]);

  useEffect(() => {
    if (mapRef.current && deliveryLocation) {
      mapRef.current.animateToRegion(
        {
          latitude: deliveryLocation.lat,
          longitude: deliveryLocation.lng,
          latitudeDelta: 0.01,
          longitudeDelta: 0.01,
        },
        300
      );
    }
  }, [deliveryLocation]);

  const centerLat =
    deliveryLocation?.lat ||
    destinationLat ||
    originLat ||
    -23.5505;
  const centerLng =
    deliveryLocation?.lng ||
    destinationLng ||
    originLng ||
    -46.6333;

  return (
    <View style={styles.container}>
      <MapView
        ref={mapRef}
        style={styles.map}
        initialRegion={{
          latitude: centerLat,
          longitude: centerLng,
          latitudeDelta: 0.05,
          longitudeDelta: 0.05,
        }}
      >
        {originLat && originLng && (
          <Marker
            coordinate={{ latitude: originLat, longitude: originLng }}
            title="Restaurante"
            pinColor={Colors.light.tint}
          />
        )}

        {destinationLat && destinationLng && (
          <Marker
            coordinate={{
              latitude: destinationLat,
              longitude: destinationLng,
            }}
            title="Destino"
            pinColor="green"
          />
        )}

        {deliveryLocation && (
          <Marker
            coordinate={{
              latitude: deliveryLocation.lat,
              longitude: deliveryLocation.lng,
            }}
            title="Entregador"
            pinColor="blue"
          />
        )}
      </MapView>

      <View style={styles.statusBar}>
        <View
          style={[
            styles.statusDot,
            { backgroundColor: connected ? "#4CAF50" : "#FF9800" },
          ]}
        />
        <Text style={styles.statusText}>
          {connected
            ? deliveryLocation
              ? "Entregador em movimento"
              : "Aguardando localização..."
            : "Conectando..."}
        </Text>
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    borderRadius: 8,
    overflow: "hidden",
  },
  map: {
    width: "100%",
    height: 300,
  },
  statusBar: {
    flexDirection: "row",
    alignItems: "center",
    padding: 10,
    backgroundColor: "rgba(255,255,255,0.9)",
    gap: 8,
  },
  statusDot: {
    width: 8,
    height: 8,
    borderRadius: 4,
  },
  statusText: {
    fontSize: 13,
    color: Colors.light.secondaryText,
  },
});
