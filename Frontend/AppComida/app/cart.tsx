import HeaderMain from "@/components/HeaderMain";
import OrderSummary from "@/components/OrderSummary";
import OrderSummaryWithTotal from "@/components/OrderSummaryWithTotal";
import PIXQRCode from "@/components/PIXQRCode";
import Colors from "@/constants/Colors";
import Texts from "@/constants/Texts";
import { useCartApi } from "@/contexts/ApiCartContext";
import { useNavigation } from "expo-router";
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
  Alert,
  TextInput,
} from "react-native";
import api from "@/services/api";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { MaterialIcons } from "@expo/vector-icons";
import PaymentComponent from "@/components/PaymentComponent";
import { useApi } from "@/contexts/ApiContext";

// Mesmo cálculo do OrderSummaryWithTotal: item.Price + adicionais x quantidade.
// Módulo-level (não preso ao componente) para poder ser testado sem
// renderizar a tela — ver app/__tests__/cart.test.ts.
export function calculateSubtotal(items: any[]): number {
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

const cart = () => {
  const {
    setHiddenCart,
    cart,
    paymentMethod,
    submitCart,
    distance,
    deliveryValue,
    establishment,
    couponCode,
    setCouponCode,
  } = useCartApi();

  const [load, setLoad] = useState(false);
  const [pixData, setPixData] = useState<{
    qrCodeBase64: string;
    copyPaste: string;
    orderId: string;
  } | null>(null);
  const [pixPaid, setPixPaid] = useState(false);

  const { getUserData } = useApi();

  const nav = useNavigation();
  const insets = useSafeAreaInsets();

  async function generatePix(
    orderId: string,
    user: any,
    serverTotal?: number
  ): Promise<boolean> {
    // O valor cobrado é o que o SERVIDOR calculou para o pedido, não uma
    // conta refeita aqui. Com cupom os dois divergem, e payment_api confere
    // o amount contra o order_total gravado (tolerância de 1 centavo): a
    // conta local faria toda cobrança de pedido com cupom ser recusada.
    // O cálculo local fica só como fallback para resposta de servidor antigo,
    // que não devolve order_total.
    const amount =
      typeof serverTotal === "number" && serverTotal > 0
        ? serverTotal
        : calculateSubtotal(cart) + (deliveryValue || 0);
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
          orderId,
        });
        return true;
      }
    } catch (e) {
      // Erro tratado pelo Alert abaixo — o usuário pode tentar novamente.
    }
    // Falha visível: o pedido já existe — o cliente precisa saber que a
    // cobrança não foi gerada e poder tentar de novo.
    Alert.alert(
      "Falha ao gerar o PIX",
      "Seu pedido foi criado, mas não conseguimos gerar a cobrança. Tentar novamente?",
      [
        { text: "Continuar sem pagar", style: "cancel" },
        {
          text: "Tentar novamente",
          onPress: () => generatePix(orderId, user, serverTotal),
        },
      ]
    );
    return false;
  }

  async function handlerSubmit() {
    setLoad(true);
    try {
      const user = await getUserData();
      const res = await submitCart(user);

      if (res.ok) {
        if (paymentMethod.type === "pix" && res.orderId) {
          // Fluxo PIX: mostra o QR Code antes de sair da tela.
          await generatePix(res.orderId, user, res.orderTotal);
          setLoad(false);
          return;
        }
        nav.goBack();
        nav.navigate("orders");
      }
    } catch (e) {
      // Falha já sinalizada ao usuário pelo Alert de submitCart/generatePix.
    }
    setLoad(false);
  }

  function closePixAndContinue() {
    setPixData(null);
    setPixPaid(false);
    nav.goBack();
    nav.navigate("orders");
  }

  // Polling do status da cobrança enquanto o QR Code está aberto:
  // confirma o pagamento sem o cliente precisar confiar no "já paguei".
  useEffect(() => {
    if (!pixData?.orderId || pixPaid) return;
    let cancelled = false;
    let tries = 0;
    const MAX_TRIES = 36; // ~3 min a cada 5s

    const interval = setInterval(async () => {
      if (cancelled) return;
      tries += 1;
      try {
        const { data } = await api.get(`/payments/order/${pixData.orderId}`);
        if (data?.status === "CONFIRMED") {
          cancelled = true;
          setPixPaid(true);
          Alert.alert("Pagamento confirmado!", "Seu pedido segue para preparo.");
          clearInterval(interval);
        }
      } catch {
        // silencioso — tenta de novo no próximo tick
      }
      if (tries >= MAX_TRIES) {
        cancelled = true;
        clearInterval(interval);
      }
    }, 5000);

    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, [pixData?.orderId, pixPaid]);

  useEffect(() => {
    setHiddenCart(true);
    // Subscription precisa ser removida no unmount — sem isso o listener
    // acumulava a cada abertura do carrinho (leak).
    const unsub = nav.addListener("blur", () => {
      setHiddenCart(false);
    });
    return unsub;
  }, [nav]);

  // NÃO navega automaticamente quando o carrinho esvazia: o submitCart
  // limpa o carrinho e, com o goBack automático, o modal do PIX era
  // desmontado na mesma hora (corrida com handlerSubmit). Carrinho vazio
  // agora renderiza um estado vazio explícito, abaixo.

  return (
    <View
      style={{
        ...styles.container,
        paddingBottom: Platform.OS === "android" ? 10 : insets.bottom,
      }}
    >
      <ScrollView style={{ height: "95%" }}>
        <HeaderMain />

        {cart.length === 0 && !pixData ? (
          <View style={styles.emptyBox}>
            <Text style={styles.emptyTitle}>Seu carrinho está vazio</Text>
            <TouchableOpacity
              style={styles.emptyBtn}
              onPress={() => nav.navigate("(tabs)" as never)}
            >
              <Text style={styles.emptyBtnTxt}>Ver restaurantes</Text>
            </TouchableOpacity>
          </View>
        ) : (
          <>
            <OrderSummary data={cart} />

            <OrderSummaryWithTotal data={cart} />

            {/* Cupom.
              *
              * O cliente só digita o CÓDIGO. Quem decide se vale, quanto
              * desconta e de quem sai a promoção é o servidor, no
              * POST /orders — inclusive recusando o pedido inteiro se o
              * cupom não valer, com uma mensagem que a tela mostra tal e
              * qual. Não há validação de valor aqui de propósito: um
              * desconto calculado no app seria um palpite que o servidor
              * ignora, e mostrá-lo antes do pedido enganaria o cliente. */}
            <View style={styles.cupomBox}>
              <Text style={styles.cupomLabel}>Cupom de desconto</Text>
              <View style={styles.cupomRow}>
                <TextInput
                  style={styles.cupomInput}
                  placeholder="Tem um código? Digite aqui"
                  placeholderTextColor="#9CA3AF"
                  value={couponCode}
                  onChangeText={(t: string) => setCouponCode(t.toUpperCase())}
                  autoCapitalize="characters"
                  autoCorrect={false}
                  editable={!load}
                  accessibilityLabel="Código do cupom"
                />
                {couponCode.length > 0 && (
                  <TouchableOpacity
                    style={styles.cupomLimpar}
                    onPress={() => setCouponCode("")}
                    disabled={load}
                    accessibilityLabel="Limpar cupom"
                  >
                    <MaterialIcons name="close" size={18} color="#6B7280" />
                  </TouchableOpacity>
                )}
              </View>
              <Text style={styles.cupomHint}>
                O desconto é aplicado ao finalizar o pedido.
              </Text>
            </View>

            <PaymentComponent
              title={(Texts as Record<string, string>)[paymentMethod.type]}
              icon={paymentMethod.icon}
            />
          </>
        )}
      </ScrollView>
      {cart.length > 0 && (
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
      )}

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
  cupomBox: {
    marginHorizontal: 16,
    marginTop: 12,
    padding: 14,
    backgroundColor: "#FFFFFF",
    borderRadius: 12,
    borderWidth: 1,
    borderColor: "#E5E7EB",
  },
  cupomLabel: {
    fontSize: 14,
    fontWeight: "600",
    color: "#111827",
    marginBottom: 8,
  },
  cupomRow: {
    flexDirection: "row",
    alignItems: "center",
    borderWidth: 1,
    borderColor: "#D1D5DB",
    borderRadius: 8,
    paddingHorizontal: 10,
  },
  cupomInput: {
    flex: 1,
    paddingVertical: 10,
    fontSize: 15,
    color: "#111827",
  },
  cupomLimpar: {
    padding: 6,
  },
  cupomHint: {
    marginTop: 6,
    fontSize: 12,
    color: "#6B7280",
  },
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
    color: Colors.light.tint,
    fontWeight: "600",
    fontSize: 14,
  },
  emptyBox: {
    alignItems: "center",
    paddingVertical: 40,
    gap: 12,
  },
  emptyTitle: {
    fontSize: 16,
    color: Colors.light.secondaryText,
  },
  emptyBtn: {
    backgroundColor: Colors.light.tint,
    paddingVertical: 10,
    paddingHorizontal: 20,
    borderRadius: 3,
  },
  emptyBtnTxt: {
    color: Colors.light.white,
    fontWeight: "600",
  },
  txtFinal: {
    fontWeight: "500",
    color: Colors.light.white,
  },
  btns: {
    width: "95%",
    backgroundColor: Colors.light.tint,
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
