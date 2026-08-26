import React, { useEffect, useState, useRef } from "react";
import { View, Text, StyleSheet } from "react-native";
import {
  Map as MapLibreMap,
  Camera,
  ViewAnnotation,
  type CameraRef,
} from "@maplibre/maplibre-react-native";
import Colors from "@/constants/Colors";
import { getWsUrl, requestWsTicket } from "@/config/api";
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

    let attempts = 0;
    const MAX_ATTEMPTS = 20;
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
    // Flag de descarte: sem ela, o close() do cleanup dispara onclose e
    // agenda uma reconexão pós-unmount (WebSocket zombie acumulando).
    let disposed = false;

    const connectWebSocket = async () => {
      try {
        if (!token) {
          setError("Token não encontrado");
          return;
        }

        // Troca JWT por ticket de 60s antes de conectar ao WS
        let wsUrl: string;
        try {
          const ticket = await requestWsTicket(token);
          wsUrl = `${getWsUrl()}/ws/delivery/${orderId}?ticket=${ticket}`;
        } catch {
          // Fallback: JWT na query string (deprecated)
          wsUrl = `${getWsUrl()}/ws/delivery/${orderId}?token=${token}`;
        }

        const ws = new WebSocket(wsUrl);

        ws.onopen = () => {
          setConnected(true);
          setError(null);
          attempts = 0;
        };

        ws.onmessage = (event) => {
          try {
            const data = JSON.parse(event.data);
            if (data.type === "location" && data.payload) {
              setDeliveryLocation(data.payload);
            }
          } catch (e) {
            // Payload não-JSON (keepalive etc.) — ignora.
          }
        };

        ws.onerror = () => {
          setError("Erro na conexão");
          setConnected(false);
        };

        ws.onclose = () => {
          setConnected(false);
          if (disposed) return;
          // Reconexão limitada: loop infinito a cada 5s drenava bateria
          // quando o servidor ficava fora por muito tempo (free tier).
          if (attempts < MAX_ATTEMPTS) {
            attempts += 1;
            reconnectTimer = setTimeout(() => connectWebSocket(), 5000);
          } else {
            setError("Conexão perdida. Reabra a tela para reconectar.");
          }
        };

        wsRef.current = ws;
      } catch (e) {
        setError("Erro ao conectar");
      }
    };

    connectWebSocket();

    return () => {
      disposed = true;
      if (reconnectTimer) clearTimeout(reconnectTimer);
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
