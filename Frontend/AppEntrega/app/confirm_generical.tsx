import Colors from "@/constants/Colors";
import {
  Alert,
  Image,
  StyleSheet,
  Text,
  TextInput,
  TouchableOpacity,
  View,
} from "react-native";
import DeliveryHappy from "../assets/images/deliveryman_happy.png";
import Texts from "@/constants/Texts";
import { useNavigation, useLocalSearchParams } from "expo-router";
import { useEffect, useState } from "react";
import api from "@/services/api";

export default function ConfirmGenerical() {
  const nav = useNavigation();
  const { onConfirm, orderId, needsCode = false, legacyCode = false } =
    useLocalSearchParams() as any;
  const [code, setCode] = useState("");
  const [checking, setChecking] = useState(false);

  function proceed() {
    if (onConfirm) onConfirm();
    nav.goBack();
  }

  // Validação server-side do código de retirada. Fallback local apenas para
  // pedidos legados sem código gerado no servidor (400).
  async function verify() {
    if (!needsCode) {
      proceed();
      return;
    }
    if (legacyCode && code === legacyCode) {
      proceed();
      return;
    }
    setChecking(true);
    try {
      const { data } = await api.post("/orders/pickup-code/validate", {
        order_id: orderId,
        pickup_code: code,
      });
      if (data?.valid) {
        proceed();
        return;
      }
      Alert.alert("", Texts.codigo_errado);
    } catch (e: any) {
      if (e?.response?.status === 400) {
        // Pedido legado sem código server-side: mantém comparação local.
        Alert.alert("", Texts.codigo_errado);
      } else if (e?.response?.status === 401) {
        Alert.alert("", Texts.codigo_errado);
      } else {
        Alert.alert(
          "",
          "Não foi possível validar o código agora. Verifique sua conexão."
        );
      }
    } finally {
      setChecking(false);
    }
  }

  useEffect(() => {
    if (code.length >= 4 && !checking) verify();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [code]);
  useEffect(() => {
    setCode("");
  }, []);

  return (
    <View style={styles.containers}>
      <View></View>
      {!needsCode ? (
        <View style={styles.imageContainer}>
          <Image source={DeliveryHappy} style={styles.images} />
          <Text style={styles.textTwo}>{Texts.continue_duvida}</Text>
          <Text style={styles.titleOne}>{Texts.nao_sera_possivel_voltar}</Text>
        </View>
      ) : (
        <View style={styles.containerdata}>
          <Text
            style={{
              ...styles.textTwo,
              textAlign: "center",
              fontSize: 20,
              width: "80%",
            }}
          >
            {Texts.codigo}
          </Text>

          <View style={styles.containermine}>
            <TextInput
              value={code}
              keyboardType={"numeric"}
              style={styles.codes}
              autoFocus={true}
              placeholder="####"
              placeholderTextColor={Colors.light.secondaryText}
              maxLength={4}
              onChangeText={(text) => setCode(text)}
            />
          </View>
        </View>
      )}

      <View style={styles.buttons}>
        <TouchableOpacity
          disabled={(needsCode && code.length < 4) || checking}
          onPress={() => {
            verify();
          }}
          style={{
            ...styles.button,
            ...styles.acceptButton,
            ...((needsCode && code.length < 4) || checking
              ? {
                  backgroundColor: Colors.light.tabIconDefault,
                  borderColor: Colors.light.secondaryText,
                }
              : {}),
          }}
        >
          <Text
            style={{
              ...styles.buttonText,
              color:
                (needsCode && code.length < 4) || checking
                  ? Colors.light.secondaryText
                  : Colors.light.tint,
            }}
          >
            {checking ? "Validando..." : Texts.confirmar}
          </Text>
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
}

const styles = StyleSheet.create({
  containers: {
    backgroundColor: Colors.light.background,
    alignContent: "center",
    alignItems: "center",
    height: "100%",
    justifyContent: "space-between",
  },
  containerdata: {
    width: "95%",
    alignContent: "center",
    alignItems: "center",
  },
  containermine: {
    borderBottomWidth: 3,
    borderBottomColor: Colors.light.tint,
    marginTop: 30,
  },
  textTwo: {
    fontSize: 22,
    fontWeight: "500",
    color: Colors.light.text,
  },
  codes: {
    fontSize: 30,
    padding: 15,
    paddingLeft: 30,
    letterSpacing: 15,
    color: Colors.light.text,
  },
  imageContainer: {
    alignContent: "center",
    alignItems: "center",
  },
  images: {
    width: 250,
    height: 250,
    resizeMode: "contain",
  },
  container: {
    backgroundColor: Colors.light.tint,
    justifyContent: "space-between",
    alignItems: "center",
    paddingHorizontal: 20,
    height: "100%",
  },
  titleOne: {
    fontSize: 15,
    fontWeight: "300",
    marginTop: 20,
    color: Colors.light.secondaryText,
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
    padding: 20,
  },
  button: {
    paddingVertical: 20,
    borderRadius: 5,
    alignItems: "center",
    marginBottom: 10,
  },
  acceptButton: {
    backgroundColor: Colors.light.white,
    borderColor: Colors.light.tint,
    borderWidth: 1,
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
