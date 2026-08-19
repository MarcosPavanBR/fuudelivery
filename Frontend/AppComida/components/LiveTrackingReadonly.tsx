import React, { useEffect, useState, useRef } from "react";
import { View, Text, StyleSheet } from "react-native";
import {
  Map as MapLibreMap,
  Camera,
  ViewAnnotation,
  type CameraRef,
} from "@maplibre/maplibre-react-native";
import Colors from "@/constants/Colors";
import { getWsUrl } from "@/config/api";
import { MAP_STYLE_URL } from "@/config/config";
import { useApi } from "@/contexts/ApiContext";

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
  // Token JWT do contexto (mesma fonte do SecureStore em ApiContext)
  const { token } = useApi();

  const [deliveryLocation, setDeliveryLocation] =
    useState<DeliveryLocation | null>(null);
  const [connected, setConnected] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const mapRef = useRef<CameraRef>(null);

  useEffect(() => {
    if (!orderId) return;

  const connectWebSocket = async () => {
    try {
      if (!token) {
        setError("Token não encontrado");
        return;
      }

      const apiUrl = getApiUrl();
      const resp = await fetch(`${apiUrl}/ws-token`, {
        headers: { Authorization: `Bearer ${token}` },
      });
      const data = await resp.json();
      if (!data.ws_token) {
        setError("Falha ao gerar token WebSocket");
        return;
      }

      const ws = new WebSocket(
        `${getWsUrl()}/ws/delivery/${orderId}?ws_token=${data.ws_token}`
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
  }, [orderId, token]);

  useEffect(() => {
    if (mapRef.current && deliveryLocation) {
      mapRef.current.flyTo({
        center: [deliveryLocation.lng, deliveryLocation.lat],
        zoom: 15,
        duration: 300,
      });
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
      <MapLibreMap style={styles.map} mapStyle={MAP_STYLE_URL}>
        <Camera
          ref={mapRef}
          center={[centerLng, centerLat]}
          zoom={12}
        />

        {originLat && originLng && (
          <ViewAnnotation id="origin" lngLat={[originLng, originLat]}>
            <View
              style={[styles.markerDot, { backgroundColor: Colors.light.tint }]}
            />
          </ViewAnnotation>
        )}

        {destinationLat && destinationLng && (
          <ViewAnnotation id="destination" lngLat={[destinationLng, destinationLat]}>
            <View style={[styles.markerDot, { backgroundColor: "green" }]} />
          </ViewAnnotation>
        )}

        {deliveryLocation && (
          <ViewAnnotation
            id="courier"
            lngLat={[deliveryLocation.lng, deliveryLocation.lat]}
          >
            <View style={[styles.markerDot, { backgroundColor: "blue" }]} />
          </ViewAnnotation>
        )}
      </MapLibreMap>

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
  markerDot: {
    width: 14,
    height: 14,
    borderRadius: 7,
    borderWidth: 2,
    borderColor: "#fff",
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
