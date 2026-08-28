import React, { useEffect, useState } from "react";
import {
  ActivityIndicator,
  Alert,
  StyleSheet,
  Text,
  TouchableOpacity,
  View,
} from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import Colors from "@/constants/Colors";
import { useNavigation, useLocalSearchParams } from "expo-router";
import Texts from "@/constants/Texts";
import helper from "@/helpers/helper";
import api from "@/services/api";
import { useAuthApi } from "@/contexts/AuthContext";

const ConfirmScreen = () => {
  const insets = useSafeAreaInsets();
  const nav = useNavigation();
  const { isActiveOrder } = useAuthApi();
  const { order, delivery } = useLocalSearchParams() as any;
  const [loading, setLoading] = useState(false);

  async function aceitarRota() {
    setLoading(true);
    try {
      const { data } = await api.put(
        "/solicitation-orders/hand-shake",
        {
          order_id: order.order_id,
          deliveryman: delivery,
        }
      );
      await isActiveOrder();

      nav.navigate("(tabs)");
    } catch (error: any) {
      await isActiveOrder();

      if (error.response) {
        // O pedido pode ter sido aceito por outro entregador (corrida do
        // polling) ou o entregador não ser mais elegível — mensagem sempre
        // definida (antes podia exibir "undefined").
        Alert.alert(
          "",
          error.response.data?.error ||
            "Não foi possível aceitar esta entrega. Ela pode ter sido aceita por outro entregador.",
          [
            {
              text: "OK",
              onPress: () => nav.navigate("(tabs)" as never),
            },
          ],
          { cancelable: false }
        );
      } else {
        Alert.alert(
          "",
          "Sem conexão com o servidor. Verifique sua internet e tente novamente.",
          [{ text: "OK" }]
        );
      }
    }
    setLoading(false);
  }

  return (
    <View
      style={[
        styles.container,
        { paddingBottom: insets.bottom, paddingTop: insets.top },
      ]}
    >
      <View style={styles.content}>
        <Text style={{ ...styles.text, marginBottom: 30 }}>
          {Texts.aceitar_rota}
        </Text>
        <Text style={styles.text}>
          {helper.formatCurrency(order.valueDelivery)}
        </Text>
        <Text style={{ ...styles.description, marginTop: 30 }}>
          {order.distance.toFixed(1)}
          {Texts.km} -{" "}
          {helper.calcularDistanciaMediaDeBike(order.distance).toFixed(1)}{" "}
          {Texts.minutos}
        </Text>
      </View>

      <View style={styles.buttons}>
        <TouchableOpacity
          onPress={() => aceitarRota()}
          disabled={loading}
          style={{ ...styles.button, ...styles.acceptButton }}
        >
          {loading ? (
            <ActivityIndicator />
          ) : (
            <Text style={{ ...styles.buttonText, color: Colors.light.tint }}>
              {Texts.aceitar}
            </Text>
          )}
        </TouchableOpacity>
        <TouchableOpacity
          onPress={() => nav.goBack()}
          style={[styles.button, styles.rejectButton]}
        >
          <Text style={{ ...styles.buttonText, color: Colors.light.white }}>
            {Texts.voltar}
          </Text>
        </TouchableOpacity>
      </View>
    </View>
  );
};

const styles = StyleSheet.create({
  container: {
    backgroundColor: Colors.light.tint,
    justifyContent: "space-between",
    alignItems: "center",
    paddingHorizontal: 20,
    height: "100%",
  },
  content: {
    alignItems: "center",
    marginTop: "30%",
  },
  text: {
    fontSize: 40,
    color: Colors.light.white,

    fontWeight: "600",
    textAlign: "center",
  },
  description: {
    fontSize: 20,
    color: Colors.light.white,
    textAlign: "center",
    marginTop: 20,
  },
  buttons: {
    width: "100%",
  },
  button: {
    paddingVertical: 20,
    borderRadius: 5,
    alignItems: "center",
    marginBottom: 10,
  },
  acceptButton: {
    backgroundColor: Colors.light.white,
  },
  rejectButton: {
    backgroundColor: Colors.light.tint,
    borderWidth: 1,
    borderColor: Colors.light.white,
    color: Colors.light.white,
  },
  buttonText: {
    fontSize: 20,
    fontWeight: "bold",
  },
});

export default ConfirmScreen;
