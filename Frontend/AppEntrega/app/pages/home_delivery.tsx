import { useEffect, useRef, useState } from "react";
import {
  Dimensions,
  Image,
  Platform,
  StyleSheet,
  Text,
  TouchableOpacity,
  View,
} from "react-native";
import {
  Map as MapLibreMap,
  Camera,
  ViewAnnotation,
  type CameraRef,
} from "@maplibre/maplibre-react-native";
import * as Location from "expo-location";
import { MaterialIcons } from "@expo/vector-icons";
import Colors from "@/constants/Colors";
import HeaderDelivery from "@/componentes/HeaderDelivery";
import { useAuthApi } from "@/contexts/AuthContext";
import helper from "@/helpers/helper";
import api from "@/services/api";
import MinimizableModal from "@/componentes/ModalMinimize";
import { useIsFocused } from "@react-navigation/native";
import Config, { MAP_STYLE_URL } from "@/constants/Config";

function HomeDelivery() {
  const mapViewRef = useRef<CameraRef>(null);

  const {
    inWork,
    disponivel,
    isActiveOrder,
    mylocation,
    setMyLocation,
  } = useAuthApi();

  const [markers, setMarkers] = useState<any>([]);
  const isAndroid = Platform.OS === "android";
  const isFocused = useIsFocused();
  const [hasFirst, setHasFirst] = useState(false);
  const intervalRef = useRef<any>(null);
  const gpsIntervalRef = useRef<any>(null);
  const inWorkRef = useRef<any>(inWork);
  inWorkRef.current = inWork;

  const centerMapOnUser = async () => {
    if (mylocation) {
      const { latitude, longitude } = mylocation.coords ?? {
        latitude: 0,
        longitude: 0,
      };
      mapViewRef.current?.flyTo({
        center: [longitude, latitude],
        zoom: 14,
        duration: 500,
      });
    }
    // Guard: has-active pode devolver [] / payload parcial — antes,
    // order[0].location.coords derrubava a tela com TypeError.
    const firstOrder: any = Array.isArray(inWork.order)
      ? inWork.order[0]
      : inWork.order;
    if (!firstOrder?.location?.coords || !firstOrder?.establishment) {
      return;
    }
    setMarkers([
      helper.getMarkerClient(
        firstOrder.location.coords.latitude,
        firstOrder.location.coords.longitude
      ),
      helper.getMarkerEstablishment(
        firstOrder.establishment.lat,
        firstOrder.establishment.long
      ),
      helper.getMarkerUser(mylocation),
    ]);
  };

  async function getPermission() {
    // Foreground primeiro (pré-requisito), background depois.
    try {
      let { status } = await Location.requestForegroundPermissionsAsync();
      if (status !== "granted") return false;

      try {
        const bg = await Location.requestBackgroundPermissionsAsync();
        if (bg.status === "granted") return true;
      } catch (e) {
        // Background negado: segue com foreground.
      }
      return true;
    } catch (e) {
      return false;
    }
  }

  async function start() {
    try {
      const status = await getPermission();
      if (!status) {
        return;
      }

      let location = await Location.getCurrentPositionAsync({});
      setMyLocation({
        ...location,
        coords: {
          ...location.coords,
          latitude: location.coords.latitude,
          longitude: location.coords.longitude,
        },
      });
    } catch (e) {
      // Silencioso: próxima iteração tenta de novo.
    }
  }

  // Envia o GPS do entregador ao servidor durante a rota ativa.
  // POST /delivery/location → alimenta o rastreamento ao vivo do cliente
  // (WS /ws/delivery/:orderId). Sem isso, o cliente nunca vê o entregador
  // se mover. POST /dispatch/location → mantém o dispatch atualizado.
  async function pushGpsToServer() {
    try {
      const loc = await Location.getCurrentPositionAsync({
        accuracy: Location.Accuracy.High,
      });
      const current: any = Array.isArray(inWorkRef.current.order)
        ? inWorkRef.current.order[0]
        : inWorkRef.current.order;
      const orderId = current?.order_id;

      if (orderId) {
        api
          .post("/delivery/location", {
            lat: loc.coords.latitude,
            lng: loc.coords.longitude,
            order_id: String(orderId),
          })
          .catch(() => {});
      }
      api
        .post("/dispatch/location", {
          lat: loc.coords.latitude,
          lng: loc.coords.longitude,
          status: "busy",
        })
        .catch(() => {});
    } catch (e) {
      // Silencioso: GPS pode falhar pontualmente (sinal, permissão).
    }
  }

  useEffect(() => {
    const iniciarIntervalo = () => {
      start();
      pushGpsToServer();
      intervalRef.current = setInterval(() => {
        isActiveOrder();
      }, Config.msUpdateOffDelivery);
      // GPS a cada 10s durante a rota ativa.
      gpsIntervalRef.current = setInterval(pushGpsToServer, 10000);
    };

    iniciarIntervalo();

    return () => {
      clearInterval(intervalRef.current);
      if (gpsIntervalRef.current) clearInterval(gpsIntervalRef.current);
    };
  }, []);

  useEffect(() => {
    if (!hasFirst && mylocation?.coords) {
      centerMapOnUser();
      setHasFirst(true);
    }
  }, [mylocation]);

  useEffect(() => {
    if (isFocused) {
      isActiveOrder();
    }
  }, [isFocused]);

  return (
    <View style={styles.container}>
      <HeaderDelivery
        loading={false}
        disponivel={disponivel}
        inWork={inWork}
        headerView={true}
        disabled={true}
        onDisponivel={(disp: boolean) => {}}
      />

      <MapLibreMap style={styles.map} mapStyle={MAP_STYLE_URL}>
        <Camera
          ref={mapViewRef}
          center={[
            mylocation?.coords.longitude || 0,
            mylocation?.coords.latitude || 0,
          ]}
          zoom={12}
        />
        {markers.map((marker: any) => (
          <ViewAnnotation
            key={marker.id}
            id={String(marker.id)}
            lngLat={[
              marker.coordinates.longitude,
              marker.coordinates.latitude,
            ]}
          >
            <View>
              {marker?.icon ? (
                <Image source={marker?.icon} style={styles.markerImage} />
              ) : null}
            </View>
          </ViewAnnotation>
        ))}
      </MapLibreMap>
      <MinimizableModal />

      <TouchableOpacity
        style={styles.centerButton}
        onPress={() => {
          centerMapOnUser();
          isActiveOrder();
        }}
      >
        <MaterialIcons name="my-location" size={24} color="white" />
      </TouchableOpacity>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: "#fff",
    alignItems: "center",
    justifyContent: "center",
  },
  map: {
    width: Dimensions.get("window").width,
    height: Dimensions.get("window").height,
    minHeight: "100%",
  },
  centerButton: {
    position: "absolute",
    bottom: "15%",
    marginBottom: 15,
    right: 16,
    backgroundColor: Colors.light.tint,
    borderRadius: 50,
    padding: 10,
  },
  markerImage: {
    width: 70,
    height: 50,
    resizeMode: "contain",
    shadowColor: "#000",
    shadowOffset: { width: 5, height: 5 },
    shadowOpacity: 0.3,
    shadowRadius: 5,
  },
});

export default HomeDelivery;
