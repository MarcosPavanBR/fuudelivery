import React, { useEffect, useState, useRef } from "react";
import { View, Text, StyleSheet, Dimensions } from "react-native";
import {
  Map as MapLibreMap,
  Camera,
  ViewAnnotation,
  GeoJSONSource,
  Layer,
  type CameraRef,
} from "@maplibre/maplibre-react-native";
import * as Location from "expo-location";
import Colors from "@/constants/Colors";
import helper from "@/helpers/helper";
import api from "@/services/api";
import { MAP_STYLE_URL } from "@/constants/Config";

interface LiveTrackingProps {
  destinationLat: number;
  destinationLng: number;
  originLat: number;
  originLng: number;
  orderId: string;
}

interface RoutePoint {
  latitude: number;
  longitude: number;
}

export default function LiveTracking({
  destinationLat,
  destinationLng,
  originLat,
  originLng,
  orderId,
}: LiveTrackingProps) {
  const [currentLocation, setCurrentLocation] =
    useState<Location.LocationObjectCoords | null>(null);
  const [routeCoords, setRouteCoords] = useState<RoutePoint[]>([]);
  const [eta, setEta] = useState<string>("");
  const [error, setError] = useState<string | null>(null);
  const mapRef = useRef<CameraRef>(null);
  const intervalRef = useRef<any>(null);

  useEffect(() => {
    const startTracking = async () => {
      try {
        let { status } = await Location.requestForegroundPermissionsAsync();
        if (status !== "granted") {
          setError("Permissão de localização negada");
          return;
        }

        const updateLocation = async () => {
          try {
            const loc = await Location.getCurrentPositionAsync({
              accuracy: Location.Accuracy.High,
            });
            setCurrentLocation(loc.coords);

            const newRoutePoint = {
              latitude: loc.coords.latitude,
              longitude: loc.coords.longitude,
            };

            setRouteCoords((prev) => {
              const last = prev[prev.length - 1];
              if (
                last &&
                Math.abs(last.latitude - newRoutePoint.latitude) < 0.0001 &&
                Math.abs(last.longitude - newRoutePoint.longitude) < 0.0001
              ) {
                return prev;
              }
              const updated = [...prev, newRoutePoint];
              if (updated.length > 100) {
                return updated.slice(-100);
              }
              return updated;
            });

            // Send GPS to backend for real-time customer tracking
            api.post("/delivery/location", {
              lat: loc.coords.latitude,
              lng: loc.coords.longitude,
              order_id: orderId,
            }).catch(() => {});

            const distance = await helper.calcularDistancia(
              loc.coords.latitude,
              loc.coords.longitude,
              destinationLat,
              destinationLng
            );
            const speed = loc.coords.speed || 20;
            const etaMinutes =
              speed > 0 && distance ? (distance / speed) * 60 : 0;
            setEta(
              etaMinutes > 0
                ? `${Math.round(etaMinutes)} min`
                : "Calculando..."
            );
          } catch (e) {
            console.log("Erro ao atualizar localização:", e);
          }
        };

        await updateLocation();
        intervalRef.current = setInterval(updateLocation, 5000);
      } catch (e) {
        console.log("Erro ao iniciar tracking:", e);
        setError("Erro ao iniciar rastreamento");
      }
    };

    startTracking();

    return () => {
      if (intervalRef.current) {
        clearInterval(intervalRef.current);
      }
    };
  }, [destinationLat, destinationLng]);

  const fitMapToMarkers = () => {
    if (mapRef.current && currentLocation) {
      const lats = [originLat, destinationLat, currentLocation.latitude];
      const lngs = [originLng, destinationLng, currentLocation.longitude];
      mapRef.current.fitBounds(
        [Math.min(...lngs), Math.min(...lats), Math.max(...lngs), Math.max(...lats)],
        { padding: { top: 60, right: 60, bottom: 60, left: 60 }, duration: 300 }
      );
    }
  };

  useEffect(() => {
    if (currentLocation) {
      fitMapToMarkers();
    }
  }, [currentLocation]);

  if (error) {
    return (
      <View style={styles.errorContainer}>
        <Text style={styles.errorText}>{error}</Text>
      </View>
    );
  }

  return (
    <View style={styles.container}>
      <MapLibreMap style={styles.map} mapStyle={MAP_STYLE_URL}>
        <Camera
          ref={mapRef}
          center={[originLng || destinationLng, originLat || destinationLat]}
          zoom={13}
        />

        <ViewAnnotation id="origin" lngLat={[originLng, originLat]}>
          <View style={[styles.markerDot, { backgroundColor: Colors.light.tint }]} />
        </ViewAnnotation>

        <ViewAnnotation id="destination" lngLat={[destinationLng, destinationLat]}>
          <View style={[styles.markerDot, { backgroundColor: "green" }]} />
        </ViewAnnotation>

        {currentLocation && (
          <ViewAnnotation
            id="courier"
            lngLat={[currentLocation.longitude, currentLocation.latitude]}
          >
            <View style={[styles.markerDot, { backgroundColor: "blue" }]} />
          </ViewAnnotation>
        )}

        {routeCoords.length > 1 && (
          <GeoJSONSource
            id="routeSource"
            data={{
              type: "FeatureCollection",
              features: [
                {
                  type: "Feature",
                  properties: {},
                  geometry: {
                    type: "LineString",
                    coordinates: routeCoords.map((p) => [
                      p.longitude,
                      p.latitude,
                    ]),
                  },
                },
              ],
            }}
          >
            <Layer
              id="routeLine"
              type="line"
              source="routeSource"
              paint={{ "line-color": Colors.light.tint, "line-width": 3 }}
            />
          </GeoJSONSource>
        )}
      </MapLibreMap>

      {eta ? (
        <View style={styles.etaContainer}>
          <Text style={styles.etaLabel}>Previsão de chegada</Text>
          <Text style={styles.etaValue}>{eta}</Text>
        </View>
      ) : null}
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
  errorContainer: {
    padding: 20,
    alignItems: "center",
  },
  errorText: {
    color: "red",
    fontSize: 14,
  },
  etaContainer: {
    position: "absolute",
    bottom: 10,
    left: 10,
    right: 10,
    backgroundColor: "rgba(255,255,255,0.9)",
    borderRadius: 8,
    padding: 12,
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "center",
  },
  etaLabel: {
    fontSize: 14,
    color: Colors.light.secondaryText,
  },
  etaValue: {
    fontSize: 16,
    fontWeight: "bold",
    color: Colors.light.tint,
  },
});
