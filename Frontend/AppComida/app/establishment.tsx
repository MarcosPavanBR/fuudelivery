import React, { useEffect, useState } from "react";
import {
  ScrollView,
  View,
  Text,
  TextInput,
  StyleSheet,
  ActivityIndicator,
  RefreshControl,
} from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { Ionicons } from "@expo/vector-icons";
import api from "@/services/api";
import {
  fetchWithCache,
  removeCached,
  CACHE_TTL,
  CACHE_KEYS,
} from "@/config/cache";
import Colors from "@/constants/Colors";
import HeaderMain from "@/components/HeaderMain";
import { useCartApi } from "@/contexts/ApiCartContext";
import ProductCategory from "./pages/porducts/ProductCategory";
import helpers from "@/helpers/helpers";
import Texts from "@/constants/Texts";

export default function Establishment() {
  const [cadProdcts, setCadProdcts] = useState<any[]>([]);
  const [searchText, setSearchText] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const { establishment } = useCartApi();
  const insets = useSafeAreaInsets();

  const init = async () => {
    try {
      // Cardápio com cache local (TTL 15 min) — evita 2 chamadas de rede
      // a cada visita ao restaurante. Rede falhou → serve o último cache.
      const categories = await fetchWithCache(
        CACHE_KEYS.menuCategories(establishment.id),
        async () => (await api.get("/categories/product/" + establishment.id)).data,
        CACHE_TTL.MENU,
        []
      );

      const produtos = await fetchWithCache(
        CACHE_KEYS.menuProducts(establishment.id),
        async () => (await api.get("/products/" + establishment.id)).data,
        CACHE_TTL.MENU,
        []
      );

      setCadProdcts([
        ...categories,
        {
          Id: 9999,
          Name: Texts.todos,
          EstablishmentId: establishment.id,
          Products: helpers.orderByImage(produtos),
        },
      ]);
      setError(false);
    } catch (e) {
      // Antes: console.log e cardápio vazio sem explicação.
      setError(true);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    init();
  }, []);

  // Pull-to-refresh: invalida o cache do cardápio e busca de novo.
  async function onRefresh() {
    setRefreshing(true);
    removeCached(CACHE_KEYS.menuCategories(establishment.id));
    removeCached(CACHE_KEYS.menuProducts(establishment.id));
    await init();
    setRefreshing(false);
  }

  return (
    <View style={[styles.container, { paddingTop: insets.top }]}>
      <HeaderMain hiddenOpen={true} hiddenBack={false} />

      <View style={styles.searchContainer}>
        <Ionicons name="search" size={18} color={Colors.light.secondaryText} />
        <TextInput
          style={styles.searchInput}
          placeholder={Texts.search_placeholder}
          placeholderTextColor={Colors.light.secondaryText}
          value={searchText}
          onChangeText={setSearchText}
        />
      </View>

      {loading ? (
        <View style={styles.loadingContainer}>
          <ActivityIndicator size="large" color={Colors.light.primary} />
        </View>
      ) : error && cadProdcts.length <= 1 ? (
        <View style={styles.loadingContainer}>
          <Text style={styles.errorTitle}>Não foi possível carregar o cardápio</Text>
          <Text style={styles.errorText}>Verifique sua conexão e puxe para atualizar.</Text>
        </View>
      ) : (
        <ScrollView
          showsVerticalScrollIndicator={false}
          contentContainerStyle={styles.scrollContent}
          refreshControl={
            <RefreshControl refreshing={refreshing} onRefresh={onRefresh} />
          }
        >
          {cadProdcts
            .filter((cat) =>
              searchText
                ? cat.Products?.some((p: any) =>
                    p.Name?.toLowerCase().includes(searchText.toLowerCase())
                  )
                : true
            )
            .map((category: any) => (
              <View key={category.Id} style={styles.categorySection}>
                {category.Name !== Texts.todos && (
                  <View style={styles.categoryHeader}>
                    <View style={styles.categoryDot} />
                    <Text style={styles.categoryName}>{category.Name}</Text>
                  </View>
                )}
                <ProductCategory category={category} />
              </View>
            ))}
          <View style={{ height: 100 }} />
        </ScrollView>
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: Colors.light.background,
  },
  searchContainer: {
    flexDirection: "row",
    alignItems: "center",
    backgroundColor: Colors.light.surface,
    marginHorizontal: 16,
    marginVertical: 12,
    paddingHorizontal: 14,
    height: 42,
    borderRadius: 12,
    gap: 8,
    borderWidth: 1,
    borderColor: Colors.light.border,
  },
  searchInput: {
    flex: 1,
    fontSize: 15,
    color: Colors.light.text,
    height: "100%",
  },
  loadingContainer: {
    flex: 1,
    alignItems: "center",
    justifyContent: "center",
  },
  scrollContent: {
    paddingHorizontal: 16,
  },
  categorySection: {
    marginBottom: 8,
  },
  categoryHeader: {
    flexDirection: "row",
    alignItems: "center",
    gap: 8,
    marginBottom: 8,
    marginTop: 4,
  },
  categoryDot: {
    width: 8,
    height: 8,
    borderRadius: 4,
    backgroundColor: Colors.light.primary,
  },
  categoryName: {
    fontSize: 17,
    fontWeight: "700",
    color: Colors.light.text,
  },
  errorTitle: {
    fontSize: 16,
    fontWeight: "600",
    color: Colors.light.text,
    marginBottom: 6,
  },
  errorText: {
    fontSize: 14,
    color: Colors.light.secondaryText,
  },
});
