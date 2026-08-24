import HeaderMain from "@/components/HeaderMain";
import OrderSummary from "@/components/OrderSummary";
import OrderSummaryWithTotal from "@/components/OrderSummaryWithTotal";
import PIXQRCode from "@/components/PIXQRCode";
import Colors from "@/constants/Colors";
import Texts from "@/constants/Texts";
import { useCartApi } from "@/contexts/ApiCartContext";
import { useNavigation } from "@react-navigation/native";
import React, { useEffect, useState } from "react";

import {
  View,
  StyleSheet,
  Text,
  TouchableOpacity,
  ScrollView,
  Platform,
  ActivityIndicator,
  Modal,
} from "react-native";
import api from "@/services/api";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { MaterialIcons } from "@expo/vector-icons";
import PaymentComponent from "@/components/PaymentComponent";
import { useApi } from "@/contexts/ApiContext";

const cart = () => {
  const { setHiddenCart, cart, paymentMethod, submitCart, distance, deliveryValue, establishment } =
    useCartApi();

  const [load, setLoad] = useState(false);
  const [pixData, setPixData] = useState<{
    qrCodeBase64: string;
    copyPaste: string;
  } | null>(null);

  const { getUserData } = useApi();

  const nav = useNavigation();
  const insets = useSafeAreaInsets();

  // Mesmo cálculo do OrderSummaryWithTotal: item.Price + adicionais x quantidade.
  function calculateSubtotal(items: any[]): number {
    return items.reduce((sum, entry) => {
      const additionalsSum = (entry.additionals || []).reduce(
        (acc: number, additionalId: number | string) => {
          const additional = (entry.item?.Additional || []).find(
            (a: any) => a.ID === additionalId
          );
          return acc + (additional?.Price || 0);
        },
        0
      );
      return sum + entry.quantity * ((entry.item?.Price || 0) + additionalsSum);
    }, 0);
  }

  async function generatePix(orderId: string, user: any) {
    const amount = calculateSubtotal(cart) + (deliveryValue || 0);
    try {
      const { data } = await api.post("/payments/pix/generate", {
        order_id: orderId,
        customer_id: Number(user?.id) || 0,
        establishment_id: Number(establishment?.id) || 0,
        amount,
        delivery_amount: deliveryValue || 0,
        method: "pix",
      });
      if (data?.qr_code_base64 || data?.pix_copy_paste) {
        setPixData({
          qrCodeBase64: data.qr_code_base64 || "",
          copyPaste: data.pix_copy_paste || "",
        });
        return true;
      }
      return false;
    } catch (e) {
      console.log("Erro ao gerar PIX:", e);
      return false;
    }
  }

  async function handlerSubmit() {
    setLoad(true);
    try {
      const user = await getUserData();
      const res = await submitCart(user);

      if (res.ok) {
        if (paymentMethod.type === "pix" && res.orderId) {
          // Fluxo PIX: mostra o QR Code antes de sair da tela.
          await generatePix(res.orderId, user);
          setLoad(false);
          return;
        }
        nav.goBack();
        nav.navigate("orders");
      }
    } catch (e) {
      console.log(e);
    }
    setLoad(false);
  }

  function closePixAndContinue() {
    setPixData(null);
    nav.goBack();
    nav.navigate("orders");
  }

  useEffect(() => {
    setHiddenCart(true);
    nav.addListener("blur", () => {
      setHiddenCart(false);
    });
  }, [nav]);

  useEffect(() => {
    if (cart.length <= 0) {
      nav.goBack();
    }
  }, [cart]);

  return (
    <View
      style={{
        ...styles.container,
        paddingBottom: Platform.OS === "android" ? 10 : insets.bottom,
      }}
    >
      <ScrollView style={{ height: "95%" }}>
        <HeaderMain />

        <OrderSummary data={cart} />

        <OrderSummaryWithTotal data={cart} />
        <PaymentComponent
          title={(Texts as Record<string, string>)[paymentMethod.type]}
          icon={paymentMethod.icon}
        />
      </ScrollView>
      <TouchableOpacity
        style={{ ...styles.btns, opacity: !distance || load ? 0.8 : 1 }}
        onPress={() => handlerSubmit()}
        disabled={!distance || load}
      >
        {!load ? (
          <>
            <Text style={styles.txtFinal}>{Texts.finalizar_pagamento}</Text>
            <MaterialIcons name="check" size={20} color={Colors.light.white} />
          </>
        ) : (
          <>
            <Text style={styles.txtFinal}>{Texts.finalizando_pedido}</Text>
            <ActivityIndicator
              size={20}
              color={Colors.light.white}
              style={{ alignSelf: "center" }}
            />
          </>
        )}
      </TouchableOpacity>

      {/* Modal do QR Code PIX — exibido após criar a cobrança no gateway. */}
      <Modal visible={!!pixData} transparent animationType="fade">
        <View style={styles.pixOverlay}>
          <View style={styles.pixCard}>
            {pixData && (
              <PIXQRCode
                qrCodeBase64={pixData.qrCodeBase64}
                copyPaste={pixData.copyPaste}
              />
            )}
            <TouchableOpacity style={styles.pixDoneBtn} onPress={closePixAndContinue}>
              <Text style={styles.pixDoneTxt}>Já paguei — acompanhar pedido</Text>
            </TouchableOpacity>
          </View>
        </View>
      </Modal>
    </View>
  );
};

const styles = StyleSheet.create({
  pixOverlay: {
    flex: 1,
    backgroundColor: "rgba(0,0,0,0.6)",
    justifyContent: "center",
    alignItems: "center",
  },
  pixCard: {
    backgroundColor: Colors.light.white,
    borderRadius: 16,
    paddingVertical: 12,
    width: "88%",
  },
  pixDoneBtn: {
    alignSelf: "center",
    marginTop: 4,
    marginBottom: 8,
    paddingHorizontal: 18,
    paddingVertical: 10,
  },
  pixDoneTxt: {
    color: Colors.dark.tint,
    fontWeight: "600",
    fontSize: 14,
  },
  txtFinal: {
    fontWeight: "500",
    color: Colors.light.white,
  },
  btns: {
    width: "95%",
    backgroundColor: Colors.dark.tint,
    paddingLeft: 10,
    paddingRight: 10,
    borderRadius: 3,
    flexDirection: "row",
    justifyContent: "space-between",
    height: 40,
    alignContent: "center",
    alignItems: "center",
    alignSelf: "center",
    marginBottom: 10,
  },
  container: {
    backgroundColor: Colors.light.white,
    minHeight: "100%",
    flexDirection: "column",
    justifyContent: "space-between",
  },
});

export default cart;
