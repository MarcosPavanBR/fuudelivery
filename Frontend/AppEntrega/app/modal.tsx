import { Linking, Platform, StyleSheet, TouchableOpacity } from "react-native";
import { SwipeButton } from "react-native-expo-swipe-button";

import { Text, View } from "@/components/Themed";
import Colors from "@/constants/Colors";
import { AntDesign, FontAwesome } from "@expo/vector-icons";
import Texts from "@/constants/Texts";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import {
  Map as MapLibreMap,
  Camera,
  ViewAnnotation,
  type CameraRef,
} from "@maplibre/maplibre-react-native";
import { useLocalSearchParams, useNavigation } from "expo-router";
import { useEffect, useRef } from "react";
import helper from "@/helpers/helper";
import SwipeButtonDelivery from "@/componentes/SwipButton";
import { useAuthApi } from "@/contexts/AuthContext";
import { MAP_STYLE_URL } from "@/constants/Config";

export default function ModalScreen() {
  const insets = useSafeAreaInsets();
  const nav = useNavigation();
  const { establishment }: any = useLocalSearchParams();
  const mapViewRef = useRef<CameraRef>(null);
  const { user } = useAuthApi();

  const centerMapOnUser = async () => {
    const { latitude, longitude } = establishment.coordinates;
    mapViewRef.current?.flyTo({
      center: [longitude, latitude],
      zoom: 14,
      duration: 500,
    });
  };

  const acceptEntrega = () => {
    nav.navigate("confirm", {
      order: establishment,
      delivery: user,
    });
  };

  const openMap = () => {
    const { latitude, longitude } = establishment.coordinates;

    const url = Platform.select({
      ios: `maps://${latitude},${longitude}?q=`,
      android: `geo:${latitude},${longitude}?q=`,
    });

    if (url) {
      Linking.openURL(url).catch((err) =>
        console.error("An error occurred", err)
      );
    }
  };

  useEffect(() => {
    setTimeout(() => {
      centerMapOnUser();
    }, 200);
  }, []);

  return (
    <View style={{ ...styles.container, paddingBottom: insets.bottom }}>
      <View>
        <View style={styles.boxOne}>
          <View style={styles.nameContainer}>
            <Text style={{ fontSize: 20 }}>{establishment.name}</Text>
            <Text style={styles.locationText}>
              {establishment.location_string}
            </Text>
          </View>
          <TouchableOpacity style={styles.btnMap} onPress={openMap}>
            <FontAwesome name="map" size={25} color={Colors.light.tint} />
            <Text style={styles.maptext}>{Texts.maps}</Text>
          </TouchableOpacity>
        </View>

        <View
          style={{ ...styles.boxOne, marginTop: 10, flexDirection: "column" }}
        >
          <Text style={styles.textMap}>{Texts.maps}</Text>
          <MapLibreMap style={styles.mapView} mapStyle={MAP_STYLE_URL}>
            <Camera
              ref={mapViewRef}
              center={[
                establishment.coordinates.longitude,
                establishment.coordinates.latitude,
              ]}
              zoom={14}
            />
            <ViewAnnotation
              id={String(establishment.id)}
              lngLat={[
                establishment.coordinates.longitude,
                establishment.coordinates.latitude,
              ]}
            >
              <View style={styles.pinDot} />
            </ViewAnnotation>
          </MapLibreMap>
        </View>

        <View
          style={{ ...styles.boxOne, marginTop: 10, flexDirection: "column" }}
        >
          <View style={styles.valores}>
            <Text style={styles.valueText}>{Texts.valorEntrega}</Text>
            <Text style={{ ...styles.valueText, fontSize: 16 }}>
              {helper.formatCurrency(establishment.valueDelivery)}
            </Text>
          </View>
          <View style={{ ...styles.valores, marginTop: 10 }}>
            <Text style={styles.valueText}>{Texts.distancia}</Text>
            <Text style={{ ...styles.valueText, fontSize: 16 }}>
              {establishment.distance.toFixed(1)}
              {Texts.km}
            </Text>
          </View>
        </View>
      </View>
      <SwipeButtonDelivery
        title={Texts.aceitar_entrega}
        onComplete={() => acceptEntrega()}
      />
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    backgroundColor: Colors.light.background,
    height: "100%",
    padding: 10,
    justifyContent: "space-between",
  },
  switTextStyle: { color: Colors.light.tint, fontWeight: "600", fontSize: 16 },
  swipContainer: {
    borderWidth: 0.8,
    borderColor: Colors.light.tint,
  },
  maptext: { color: Colors.light.tint, marginTop: 5 },
  nameContainer: {
    backgroundColor: Colors.light.white,
    width: "80%",
    paddingRight: 15,
  },
  mapView: {
    width: "100%",
    height: 200,
    borderColor: Colors.light.background,
    borderWidth: 1,
  },
  pinDot: {
    width: 16,
    height: 16,
    borderRadius: 8,
    backgroundColor: Colors.light.tint,
    borderWidth: 2,
    borderColor: "#fff",
  },
  valueText: { fontWeight: "500", fontSize: 15 },
  locationText: {
    marginTop: 10,
    fontSize: 14,
    textAlign: "left",
  },
  valores: {
    flexDirection: "row",
    justifyContent: "space-between",
    backgroundColor: Colors.light.white,
    width: "100%",
  },
  textMap: {
    alignSelf: "flex-start",
    marginBottom: 10,
    fontSize: 15,
    fontWeight: "500",
  },
  btnMap: {
    padding: 10,
    alignContent: "center",
    justifyContent: "center",
    alignItems: "center",
    gap: 2,
    backgroundColor: Colors.light.white,
    borderColor: Colors.light.tint,
    borderWidth: 1,
    borderRadius: 3,
    paddingLeft: 15,
    paddingRight: 14,
  },
  boxOne: {
    width: "100%",
    backgroundColor: Colors.light.white,
    borderRadius: 5,
    padding: 15,
    flexDirection: "row",
    justifyContent: "space-around",
    alignContent: "center",
    alignItems: "center",
  },
  btns: {
    backgroundColor: Colors.light.tint,
    padding: 15,
    borderRadius: 3,
    flexDirection: "row",
    justifyContent: "space-between",
    alignContent: "center",
    alignItems: "center",
  },

  title: {
    fontSize: 20,
    fontWeight: "bold",
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
  separator: {
    marginVertical: 30,
    height: 1,
    width: "80%",
  },
});
