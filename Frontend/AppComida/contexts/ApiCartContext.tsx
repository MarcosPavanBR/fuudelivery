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
import { useNavigation } from "@react-navigation/native";
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
  submitCart(user: any): Promise<{ ok: boolean; orderId?: string }>;

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

const ApiContext = createContext<ApiContextProps | undefined>(undefined);

interface ApiCartProviderProps {
  children: ReactNode;
}

export const ApiCartProvider: React.FC<ApiCartProviderProps> = ({
  children,
}) => {
  const [cart, setCart] = useState<object[]>([]);
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

  const validDelivery = () => {
    if (!distance || distance > establishment.max_distance_delivery) {
      return false;
    }
    return true;
  };

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

  async function submitCart(user: any): Promise<{ ok: boolean; orderId?: string }> {
    if (!validDelivery()) {
      Alert.alert("", Texts.erroPedido);
      return { ok: false };
    }
    const coords_location = await helpers.getLocationDistance();
    const body = {
      cart,
      distance,
      location: {
        ...location,
        coords: coords_location,
      },
      paymentMethod,
      deliveryValue,
      user,
      establishmentId: establishment.id,
      establishment: {
        ...establishment,
      },
    };

    try {
      const { data } = await api.post(`/orders`, body);
      setCart([]);
      // A API responde { message, orderId } — o orderId é necessário para
      // gerar a cobrança PIX (POST /payments/pix/generate) no carrinho.
      return { ok: true, orderId: data?.orderId ? String(data.orderId) : undefined };
    } catch (e) {
      Alert.alert("", Texts.erroPedido);
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
