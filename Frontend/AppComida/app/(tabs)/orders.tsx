import {
  Image,
  ScrollView,
  StyleSheet,
  TouchableOpacity,
  type ViewStyle,
} from "react-native";
import { Text, View } from "@/components/Themed";
import Colors from "@/constants/Colors";
import OrderSummary from "@/components/OrderSummary";
import { useEffect, useRef, useState } from "react";
import { useApi } from "@/contexts/ApiContext";
import { useIsFocused } from "expo-router/react-navigation";
import Texts from "@/constants/Texts";
import helpers from "@/helpers/helpers";
import Strings from "@/constants/Strings";
import orderModel from "@/services/order.model";
import ReorderButton from "@/components/ReorderButton";
import { useCartApi } from "@/contexts/ApiCartContext";
import { useNavigation } from "expo-router";
import LiveTrackingReadonly from "@/components/LiveTrackingReadonly";

export default function TabTwoScreen() {
  const { getUserData } = useApi();
  const { addCart, cleanCart } = useCartApi();
  const navigation = useNavigation();
  const [myOrders, setMyOrders] = useState([]);
  const isFocused = useIsFocused();
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const [expandedOrder, setExpandedOrder] = useState<string | null>(null);
  const [userHasPhone, setUserHasPhone] = useState<boolean | null>(null);

  function sortObjectsByLastModified(arr: any) {
    arr.sort((a: any, b: any) => {
      const aTime = a.lastModified ? new Date(a.lastModified).getTime() : 0;
      const bTime = b.lastModified ? new Date(b.lastModified).getTime() : 0;
      return bTime - aTime;
    });
    return arr;
  }

  async function getMyOrders() {
    const userData = await getUserData();
    if (!userData?.phone) {
      setUserHasPhone(false);
      setMyOrders([]);
      return;
    }

    setUserHasPhone(true);
    const data = await orderModel.getOrders(userData?.phone);
    setMyOrders(sortObjectsByLastModified(data));
  }

  const handleReorder = (cart: any[]) => {
    cleanCart();
    cart.forEach((item: any) => addCart(item));
    navigation.navigate("cart");
  };

  // Total real do pedido = soma dos itens + taxa de entrega.
  // Antes exibia apenas a taxa de entrega como se fosse o total.
  const orderTotal = (e: any) => {
    const items = (e.cart || []).reduce(
      (sum: number, item: any) =>
        sum + (item.item?.Price || item.item?.price || 0) * (item.quantity || 1),
      0
    );
    return items + (e.deliveryValue || 0);
  };

  const toggleExpand = (orderId: string) => {
    setExpandedOrder(expandedOrder === orderId ? null : orderId);
  };

  useEffect(() => {
    if (isFocused) {
      getMyOrders();
      intervalRef.current = setInterval(() => {
        getMyOrders();
      }, Strings.wait_interval);
    }

    return () => {
      // useRef garante o ID atual no cleanup (useState guardava valor stale
      // e o intervalo continuava rodando com a aba fora de foco).
      if (intervalRef.current) {
        clearInterval(intervalRef.current);
        intervalRef.current = null;
      }
    };
  }, [isFocused]);

  return (
    <ScrollView style={styles.container}>
      <View style={{ alignItems: "center", paddingTop: 10 }}>
        {userHasPhone === false && (
          <View style={styles.noPhoneBox}>
            <Text style={styles.noPhoneTitle}>
              Seus pedidos aparecem pelo telefone cadastrado
            </Text>
            <Text style={styles.noPhoneText}>
              Adicione seu telefone em Configurações para ver o histórico de
              pedidos aqui.
            </Text>
            <TouchableOpacity
              style={styles.noPhoneButton}
              onPress={() => navigation.navigate("config" as never)}
            >
              <Text style={styles.noPhoneButtonText}>Adicionar telefone</Text>
            </TouchableOpacity>
          </View>
        )}

        {userHasPhone === true && myOrders.length === 0 && (
          <Text style={styles.emptyText}>
            Você ainda não fez nenhum pedido.
          </Text>
        )}

        {myOrders?.map((e: any) => {
          const orderId = e._id?.$oid || e._id?.toString() || "";
          const isExpanded = expandedOrder === orderId;
          return (
            <TouchableOpacity key={orderId} style={styles.containerStyle} onPress={() => toggleExpand(orderId)} activeOpacity={0.7}>
              <View style={styles.contStl}>
                <View style={styles.container2}>
                  <View style={styles.container3}>
                    <Image
                      source={{ uri: e?.establishment?.image }}
                      style={styles.imageStyle}
                    />
                    <Text style={styles.text}>{e?.establishment?.name}</Text>
                  </View>
                </View>

                {e.status !== "FINISHED" ? (
                  <View style={styles.fins}>
                    <Text style={styles.codtexts}>Código</Text>
                    <Text style={{ fontSize: 20, fontWeight: "bold" }}>
                      {/* Código gerado pelo SERVIDOR quando o pedido fica
                          pronto (DONE). Sem código server-side ainda (pedidos
                          antigos), mostra aviso em vez do código previsível. */}
                      {e.pickup_code ?? "gera quando pronto"}
                    </Text>
                  </View>
                ) : null}

                <View style={context1(e.status)}>
                  <View style={context2(e.status)}>
                    <Text style={styles.codtext2}>
                      {(Texts as Record<string, string>)[e.status] ?? e.status}
                    </Text>
                    <Text
                      style={{
                        color: Colors.light.white,
                        fontWeight: "600",
                        fontSize: 13,
                      }}
                    >
                      {e.lastModified
                        ? helpers.formatDate(e.lastModified)
                        : null}
                    </Text>
                  </View>
                </View>
              </View>

              {isExpanded && (
                <View style={styles.expandedSection}>
                  <Text style={styles.sectionTitle}>Itens do Pedido</Text>
                  {e.cart?.map((item: any, idx: number) => (
                    <View key={idx} style={styles.itemRow}>
                      <Text style={styles.itemName}>
                        {item.quantity}x {item.item?.Name || item.item?.name}
                      </Text>
                      <Text style={styles.itemPrice}>
                        {helpers.formatCurrency(
                          (item.item?.Price || item.item?.price || 0) * (item.quantity || 1)
                        )}
                      </Text>
                    </View>
                  ))}

                  {e.location && (
                    <>
                      <Text style={[styles.sectionTitle, { marginTop: 10 }]}>Endereço de Entrega</Text>
                      <Text style={styles.detailText}>
                        {e.location.logradouro}, {e.location.numero} - {e.location.bairro}, {e.location.localidade}
                      </Text>
                    </>
                  )}

                  {e.status !== "FINISHED" && e.status !== "CANCELLED" && (
                    <>
                      <Text style={[styles.sectionTitle, { marginTop: 10 }]}>Rastreamento ao Vivo</Text>
                      <LiveTrackingReadonly
                        orderId={orderId}
                        originLat={e.establishment?.lat}
                        originLng={e.establishment?.long}
                        destinationLat={e.location?.coords?.latitude}
                        destinationLng={e.location?.coords?.longitude}
                      />
                    </>
                  )}

                  {e.paymentMethod && (
                    <>
                      <Text style={[styles.sectionTitle, { marginTop: 10 }]}>Forma de Pagamento</Text>
                      <Text style={styles.detailText}>{e.paymentMethod.type}</Text>
                    </>
                  )}

                  <Text style={[styles.sectionTitle, { marginTop: 10 }]}>Total</Text>
                  <Text style={styles.totalText}>
                    {helpers.formatCurrency(orderTotal(e))}
                  </Text>

                  {e.status === "FINISHED" && (
                    <ReorderButton
                      cart={e.cart || []}
                      onReorder={handleReorder}
                    />
                  )}
                </View>
              )}

              <OrderSummary disabled={true} data={e.cart} />
            </TouchableOpacity>
          );
        })}
      </View>
    </ScrollView>
  );
}

const context1 = (status: string): ViewStyle => {
  return {
    padding: 5,
    paddingLeft: 10,
    paddingRight: 10,
    backgroundColor:
      status !== "FINISHED" ? Colors.light.secondaryText : Colors.light.green,
    justifyContent: "center",
    borderRadius: 3,
    height: 35,
    marginTop: 10,
    marginBottom: 10,
  };
};

const context2 = (status: string): ViewStyle => {
  return {
    backgroundColor:
      status !== "FINISHED" ? Colors.light.secondaryText : Colors.light.green,
    flexDirection: "row",
    justifyContent: "space-between",
  };
};

const styles = StyleSheet.create({
  container: {
    backgroundColor: Colors.light.background,
  },
  contStl: {
    flexDirection: "column",
    justifyContent: "space-between",
  },
  codtext2: {
    color: Colors.light.white,
    fontWeight: "600",
    fontSize: 13,
  },
  codtexts: {
    fontSize: 20,
    fontWeight: "bold",
    color: Colors.light.text,
  },
  fins: {
    flexDirection: "row",
    justifyContent: "space-between",
    backgroundColor: Colors.light.tabIconDefault,
    padding: 10,
    paddingLeft: 10,
    paddingRight: 10,
    borderRadius: 3,
    marginTop: 10,
  },
  container2: {
    display: "flex",
    justifyContent: "space-between",
    flexDirection: "row",
    alignItems: "center",
  },
  text: {
    fontSize: 19,
    fontWeight: "500",
    color: Colors.light.text,
  },
  container3: {
    flexDirection: "row",
    alignItems: "center",
    gap: 10,
  },
  imageStyle: {
    width: 50,
    height: 50,
    borderRadius: 50,
    borderWidth: 1,
    borderColor: Colors.light.tabIconDefault,
  },
  containerStyle: {
    borderWidth: 1,
    width: "95%",
    padding: 10,
    borderColor: Colors.light.tabIconDefault,
    borderRadius: 3,
    marginBottom: 10,
  },
  title: {
    fontSize: 20,
    fontWeight: "bold",
  },
  separator: {
    marginVertical: 30,
    height: 1,
    width: "80%",
  },
  expandedSection: {
    padding: 10,
    backgroundColor: Colors.light.lightGray,
    borderRadius: 6,
    marginTop: 5,
  },
  sectionTitle: {
    fontSize: 14,
    fontWeight: "700",
    color: Colors.light.text,
    marginBottom: 4,
  },
  itemRow: {
    flexDirection: "row",
    justifyContent: "space-between",
    paddingVertical: 3,
  },
  itemName: {
    fontSize: 13,
    color: Colors.light.text,
    flex: 1,
  },
  itemPrice: {
    fontSize: 13,
    fontWeight: "600",
    color: Colors.light.text,
  },
  detailText: {
    fontSize: 13,
    color: Colors.light.gray,
  },
  totalText: {
    fontSize: 16,
    fontWeight: "700",
    color: Colors.light.tint,
  },
  noPhoneBox: {
    width: "90%",
    alignItems: "center",
    padding: 20,
    marginTop: 30,
    borderWidth: 1,
    borderColor: Colors.light.tabIconDefault,
    borderRadius: 3,
  },
  noPhoneTitle: {
    fontSize: 16,
    fontWeight: "bold",
    color: Colors.light.text,
    textAlign: "center",
  },
  noPhoneText: {
    fontSize: 13,
    color: Colors.light.secondaryText,
    textAlign: "center",
    marginTop: 8,
  },
  noPhoneButton: {
    backgroundColor: Colors.light.tint,
    paddingVertical: 10,
    paddingHorizontal: 20,
    borderRadius: 3,
    marginTop: 16,
  },
  noPhoneButtonText: {
    color: Colors.light.white,
    fontSize: 14,
    fontWeight: "bold",
  },
  emptyText: {
    marginTop: 40,
    fontSize: 14,
    color: Colors.light.secondaryText,
  },
});
