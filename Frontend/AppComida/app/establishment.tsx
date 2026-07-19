import React, { useEffect, useState } from "react";
import {
  ScrollView,
  View,
  Text,
  TextInput,
  StyleSheet,
  ActivityIndicator,
} from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { Ionicons } from "@expo/vector-icons";
import api from "@/services/api";
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
  const { establishment } = useCartApi();
  const insets = useSafeAreaInsets();

  const init = async () => {
    try {
      const categories = await api.get(
        "/api/order/categories/product/" + establishment.id
      );

      const produtos = await api.get(
        "/api/order/products/" + establishment.id
      );

      setCadProdcts([
        ...categories.data,
        {
          Id: 9999,
          Name: Texts.todos,
          EstablishmentId: establishment.Id,
          Products: helpers.orderByImage(produtos.data),
        },
      ]);
    } catch (e) {
      console.log(e);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    init();
  }, []);

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
      ) : (
        <ScrollView
          showsVerticalScrollIndicator={false}
          contentContainerStyle={styles.scrollContent}
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
});
