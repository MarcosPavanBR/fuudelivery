import React, {
  createContext,
  useContext,
  useState,
  ReactNode,
  useEffect,
} from "react";
import {
  View,
  Text,
  StyleSheet,
  TouchableOpacity,
  Alert,
  Platform,
} from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import Colors from "@/constants/Colors";
import Texts from "@/constants/Texts";
import { useNavigation } from "expo-router";
import helpers from "@/helpers/helpers";

import * as Location from "expo-location";
import api from "@/services/api";
import storage from "@/config/storage";
import Strings from "@/constants/Strings";
import { ESTABLISHMENT, PAYMENT_TYPE } from "@/config/config";

interface ApiContextProps {
  cart: object[];
  addCart(item: object): void;
  removeCart(item: object): void;
  editCart(item: object): void;
  cleanCart(): void;
  setPaymentMethod(method: object): void;
  submitCart(user: any): Promise<{
    ok: boolean;
    orderId?: string;
    orderTotal?: number;
    discount?: number;
  }>;
  couponCode: string;
  setCouponCode(code: string): void;

  validDelivery(): boolean;
  paymentMethod: any;
  deliveryValue: number | null;

  setHiddenCart(state: boolean): void;
  setEstablishment(establishment: any): void;
  getValueDelivery(ns: number, id: string | number): Promise<any>;
  distance: number | null;
  setMyLocation(location: object): void;
  location: any;
  establishment: any;
}

// ─── Lógica pura do checkout (extraída para ser testável sem renderizar
// o provider inteiro — ver contexts/__tests__/ApiCartContext.test.ts) ───

// Sem distância calculada, ou distância acima do raio do estabelecimento,
// o pedido não pode ser entregue.
export function isDeliveryValid(
  distance: number | null,
  establishment: { max_distance_delivery: number }
): boolean {
  if (!distance || distance > establishment.max_distance_delivery) {
    return false;
  }
  return true;
}

export interface OrderPayload {
  cart: object[];
  distance: number | null;
  location: Record<string, unknown>;
  paymentMethod: unknown;
  deliveryValue: number | null;
  user: unknown;
  establishmentId: number;
  establishment: Record<string, unknown>;
  // Só o CÓDIGO vai daqui. Valor do desconto, vigência, limites e de quem
  // sai a promoção são decididos no servidor — o app não tem como (nem
  // deve) opinar sobre isso. Omitido quando não há cupom, para o pedido
  // sem cupom continuar exatamente igual ao que era antes.
  coupon_code?: string;
}

// Monta o corpo de POST /orders. Isolado do submitCart para poder checar,
// sem chamar a API de verdade, que o pedido leva o estabelecimento e a
// taxa de entrega corretos — já houve caso aqui de enviar o delivery_value
// do estabelecimento anterior ao trocar de restaurante no carrinho.
export function buildOrderPayload(params: {
  cart: object[];
  distance: number | null;
  location: Record<string, unknown>;
  coordsLocation: unknown;
  paymentMethod: unknown;
  deliveryValue: number | null;
  user: unknown;
  establishment: { id: number };
  couponCode?: string;
}): OrderPayload {
  const coupon = (params.couponCode || "").trim().toUpperCase();
  return {
    ...(coupon ? { coupon_code: coupon } : {}),
    cart: params.cart,
    distance: params.distance,
    location: {
      ...params.location,
      coords: params.coordsLocation,
    },
    paymentMethod: params.paymentMethod,
    deliveryValue: params.deliveryValue,
    user: params.user,
    establishmentId: params.establishment.id,
    establishment: {
      ...params.establishment,
    },
  };
}

// Interpreta a resposta de POST /orders. orderId precisa virar string pois é
// usado depois para gerar a cobrança PIX (POST /payments/pix/generate), que
// espera o id como string.
//
// orderTotal é o total que o SERVIDOR calculou, já com desconto de cupom. É
// ele que a cobrança tem de usar: payment_api confere o amount contra o
// order_total gravado no pedido, então recalcular no app faria toda cobrança
// de pedido com cupom ser recusada. Vem opcional porque resposta de servidor
// antigo não traz o campo — nesse caso o chamador cai no cálculo local.
export function parseOrderResponse(data: any): {
  ok: true;
  orderId?: string;
  orderTotal?: number;
  discount?: number;
  couponCode?: string;
} {
  const total = Number(data?.order_total);
  const desconto = Number(data?.discount_amount);
  return {
    ok: true,
    orderId: data?.orderId ? String(data.orderId) : undefined,
    orderTotal: Number.isFinite(total) && total > 0 ? total : undefined,
    discount: Number.isFinite(desconto) && desconto > 0 ? desconto : undefined,
    couponCode: data?.coupon_code || undefined,
  };
}

// Mensagem de erro de POST /orders para mostrar ao cliente.
//
// O servidor recusa o pedido inteiro quando o cupom não vale ("cupom: cupom
// expirado", "cupom: valor mínimo do pedido não atingido"...). Engolir isso
// num "erro ao fazer o pedido" genérico deixaria o cliente sem saber que o
// problema é o código que ele digitou — e sem como corrigir.
export function orderErrorMessage(err: any, fallback: string): string {
  const doServidor = err?.response?.data?.error;
  return typeof doServidor === "string" && doServidor.trim()
    ? doServidor
    : fallback;
}

const ApiContext = createContext<ApiContextProps | undefined>(undefined);

interface ApiCartProviderProps {
  children: ReactNode;
}

export const ApiCartProvider: React.FC<ApiCartProviderProps> = ({
  children,
}) => {
  const [cart, setCart] = useState<object[]>([]);
  // Código do cupom digitado no carrinho. Vive no contexto (e não na tela)
  // porque submitCart é quem o envia, e porque ele precisa sobreviver ao
  // cliente sair do carrinho para adicionar mais um item.
  const [couponCode, setCouponCode] = useState("");
  const insets = useSafeAreaInsets();
  const nav = useNavigation();
  const [hiddenCart, setHiddenCart] = useState(false);
  const [distance, setDistance] = useState<null | number>(null);
  const [deliveryValue, setDeliveryValue] = useState<null | number>(0);
  const [establishment, setEstablishment] = useState(ESTABLISHMENT);

  const [location, setLocation] = useState({
    cep: null,
    logradouro: null,
    complemento: null,
    bairro: null,
    localidade: null,
    uf: null,
    ibge: null,
    gia: null,
    numero: null,
    ddd: null,
    siafi: null,
  });

  const [paymentMethod, setPaymentMethod] = useState(PAYMENT_TYPE[0]);

  const cleanCart = () => {
    setCart([]);
  };

  const addCart = (item: object) => {
    // Update funcional: evita closure stale ao adicionar vários itens em
    // sequência (ex.: "repetir pedido"), que fazia apenas o último entrar.
    setCart((prev) => [...prev, { ...item, id: helpers.generateId(15) }]);
  };

  const removeCart = (item: any) => {
    setCart((prev) => prev.filter((e: any) => e.id !== item.id));
  };

  const editCart = (item: any) => {
    setCart((prev) => prev.map((e: any) => (e.id === item.id ? item : e)));
  };

  const validDelivery = () => isDeliveryValid(distance, establishment);

  const getValueDelivery = async (ns: number, id: string | number) => {
    try {
      const { data } = await api.post(
        "/delivery/calculate-delivery-value",
        {
          distance: ns,
          establishmentId: id,
        }
      );

      return data.deliveryValue;
    } catch (e) {
      return null;
    }
  };

  function getMyLocationStorange() {
    const locs = storage.getItem(Strings.token_location);
    if (locs) {
      try {
        setLocation(JSON.parse(locs));
      } catch (e) {
        // Localização salva corrompida — ignora e segue com default.
      }
    }
  }

  function setMyLocation(locs: any) {
    storage.setItem(Strings.token_location, JSON.stringify(locs));
    setLocation(locs);
  }

  async function init() {
    try {
      const dist = await helpers.calcularDistancia(
        establishment.lat,
        establishment.long
      );
      setDistance(dist);

      getMyLocationStorange();
      if (dist) {
        const distVal = await getValueDelivery(dist, establishment.id);
        setDeliveryValue(distVal);
      } else {
        setDeliveryValue(null);
      }
    } catch (e) {
      setDeliveryValue(null);
    }
  }

  async function submitCart(user: any): Promise<{
    ok: boolean;
    orderId?: string;
    orderTotal?: number;
    discount?: number;
  }> {
    if (!validDelivery()) {
      Alert.alert("", Texts.erroPedido);
      return { ok: false };
    }
    const coords_location = await helpers.getLocationDistance();
    const body = buildOrderPayload({
      cart,
      distance,
      location,
      coordsLocation: coords_location,
      paymentMethod,
      deliveryValue,
      user,
      establishment,
      couponCode,
    });

    try {
      const { data } = await api.post(`/orders`, body);
      setCart([]);
      // Cupom consumido: o próximo pedido começa sem código preenchido.
      setCouponCode("");
      return parseOrderResponse(data);
    } catch (e) {
      // A mensagem do servidor manda: é ela que diz que foi o cupom, e qual
      // o problema. O carrinho NÃO é limpo — o cliente corrige o código e
      // tenta de novo sem remontar o pedido.
      Alert.alert("", orderErrorMessage(e, Texts.erroPedido));
      return { ok: false };
    }
  }

  useEffect(() => {
    init();
    // Recalcula distância/taxa sempre que o estabelecimento mudar — antes,
    // o valor de entrega do restaurante selecionado anteriormente (ou do
    // default) era enviado no pedido.
  }, [establishment]);

  return (
    <ApiContext.Provider
      value={
        {
          addCart,
          editCart,
          cleanCart,
          removeCart,
          submitCart,
          setEstablishment,
          setMyLocation,
          validDelivery,
          setHiddenCart,
          setPaymentMethod,

          establishment,
          cart,
          distance,
          location,
          paymentMethod,
          deliveryValue,
          couponCode,
          setCouponCode,
        } as any
      }
    >
      {children}
      {!hiddenCart && cart.length !== 0 && (
        <TouchableOpacity
          style={{
            ...styles.cartContainer,
            paddingBottom: Platform.OS === "android" ? 15 : insets.bottom,
          }}
          onPress={() => nav.navigate("cart")}
        >
          <Text
            style={{
              ...styles.cartText,
              color: Colors.light.tint,
            }}
          >
            {Texts.carrinho}
          </Text>
          <Text style={{ ...styles.cartText }}>
            {cart.length.toString().padStart(2, "0")}{" "}
            {cart.length === 1 ? Texts.item : Texts.items}
          </Text>
        </TouchableOpacity>
      )}
    </ApiContext.Provider>
  );
};

const styles = StyleSheet.create({
  cartContainer: {
    backgroundColor: Colors.light.white,
    alignContent: "center",
    paddingLeft: 20,
    paddingRight: 20,
    paddingTop: 10,
    marginTop: -20,
    flexDirection: "row",
    justifyContent: "space-between",
    borderTopColor: Colors.light.tabIconDefault,
    borderTopWidth: 1,
  },
  cartText: {
    fontSize: 18,
    color: Colors.light.tint,
    fontWeight: "400",
  },
});

export const useCartApi = (): ApiContextProps => {
  const context = useContext(ApiContext);
  if (!context) {
    throw new Error("useCartApi must be used within an ApiCartProvider");
  }
  return context;
};
