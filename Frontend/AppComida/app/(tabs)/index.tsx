import React, { useEffect, useState } from "react";
import { RefreshControl, ScrollView, StyleSheet, Text } from "react-native";

import Colors from "@/constants/Colors";
import { useNavigation } from "expo-router";
import { useIsFocused } from "expo-router/react-navigation";
import { APP_MODE, APP_MODE_OPTIONS } from "@/config/config";
import EstablishmentView from "@/components/EstablishmentView";
import Establishment from "../establishment";
import { useCartApi } from "@/contexts/ApiCartContext";
import { removeCached, CACHE_KEYS } from "@/config/cache";

import { View } from "@/components/Themed";
import Texts from "@/constants/Texts";
import establishmentsModel from "@/services/establishments.model";

export default function index() {
  return (
    <>{APP_MODE == APP_MODE_OPTIONS.unique ? <Establishment /> : TabTwo()}</>
  );
}

function TabTwo() {
  const { setEstablishment, cleanCart } = useCartApi();
  const nav = useNavigation();
  const isFocused = useIsFocused();

  const [estabelecimentos, setEstabelecimentos] = useState([]);
  const [refreshing, setRefreshing] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);

  async function init() {
    try {
      const resp = await establishmentsModel.getEstablishment();
      setEstabelecimentos(resp);
      setError(false);
    } catch (e) {
      // Antes a falha ficava invisível: o usuário via "nenhum restaurante
      // aberto" quando na verdade a API estava fora.
      setError(true);
    } finally {
      setLoading(false);
    }
  }

  // Pull-to-refresh: invalida o cache da lista e busca de novo.
  async function onRefresh() {
    setRefreshing(true);
    removeCached(CACHE_KEYS.ESTABLISHMENTS);
    await init();
    setRefreshing(false);
  }

  useEffect(() => {
    if (isFocused) {
      cleanCart();
    }
  }, [isFocused]);

  useEffect(() => {
    init();
  }, []);

  return (
    <ScrollView
      style={{
        backgroundColor: Colors.light.background,
        paddingTop: 10,
      }}
      showsVerticalScrollIndicator={false}
      refreshControl={
        <RefreshControl refreshing={refreshing} onRefresh={onRefresh} />
      }
    >
      {loading && estabelecimentos.length === 0 && (
        <View style={styles.container}>
          <Text style={styles.conttexts}>Carregando restaurantes...</Text>
        </View>
      )}
      {!loading && error && estabelecimentos.length === 0 && (
        <View style={styles.container}>
          <Text style={styles.errorTitle}>Não foi possível carregar</Text>
          <Text style={styles.conttexts}>
            Verifique sua conexão e puxe para atualizar.
          </Text>
        </View>
      )}
      {!loading && !error && estabelecimentos.length === 0 && (
        <View style={styles.container}>
          <Text style={styles.conttexts}>
            {Texts.nenhum_estabelecimento_aberto}
          </Text>
        </View>
      )}
      {estabelecimentos.map((e: any) => (
        <EstablishmentView
          key={e?.id ?? e?.ID ?? e?.Id}
          item={e}
          onPress={() => {
            setEstablishment(e);
            nav.navigate("establishment");
          }}
        />
      ))}
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: {
    marginTop: "20%",
    alignContent: "center",
    alignItems: "center",
    justifyContent: "center",
  },
  conttexts: {
    fontSize: 14,
    fontWeight: "300",
  },
  errorTitle: {
    fontSize: 16,
    fontWeight: "600",
    color: Colors.light.text,
    marginBottom: 6,
  },
});
