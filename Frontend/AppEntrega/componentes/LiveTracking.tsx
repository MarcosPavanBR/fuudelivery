import React, { useEffect, useState, useRef } from "react";
import { View, Text, StyleSheet, Dimensions } from "react-native";
import MapView, { Marker, Polyline } from "react-native-maps";
import * as Location from "expo-location";
import Colors from "@/constants/Colors";
import helper from "@/helpers/helper";

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
  const mapRef = useRef<MapView>(null);
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

            const distance = helper.calcularDistancia(
              loc.coords.latitude,
              loc.coords.longitude,
              destinationLat,
              destinationLng
            );
            const speed = loc.speed || 20;
            const etaMinutes = speed > 0 ? (distance / speed) * 60 : 0;
            setEta(
              etaMinutes > 0
                ? `${Math.round(etaMinutes)} ${helper.minutos || "min"}`
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
      mapRef.current.fitToCoordinates(
        [
          { latitude: originLat, longitude: originLng },
          { latitude: destinationLat, longitude: destinationLng },
          {
            latitude: currentLocation.latitude,
            longitude: currentLocation.longitude,
          },
        ],
        {
          edgePadding: { top: 60, right: 60, bottom: 60, left: 60 },
          animated: true,
        }
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
      <MapView
        ref={mapRef}
        style={styles.map}
        googleMapId="6cba0e311b251b4c"
        initialRegion={{
          latitude: originLat || destinationLat,
          longitude: originLng || destinationLng,
          latitudeDelta: 0.05,
          longitudeDelta: 0.05,
        }}
      >
        <Marker
          coordinate={{ latitude: originLat, longitude: originLng }}
          title="Restaurante"
          pinColor={Colors.light.tint}
        />

        <Marker
          coordinate={{ latitude: destinationLat, longitude: destinationLng }}
          title="Destino"
          pinColor="green"
        />

        {currentLocation && (
          <Marker
            coordinate={{
              latitude: currentLocation.latitude,
              longitude: currentLocation.longitude,
            }}
            title="Entregador"
            pinColor="blue"
          />
        )}

        {routeCoords.length > 1 && (
          <Polyline
            coordinates={routeCoords}
            strokeColor={Colors.light.tint}
            strokeWidth={3}
          />
        )}
      </MapView>

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
