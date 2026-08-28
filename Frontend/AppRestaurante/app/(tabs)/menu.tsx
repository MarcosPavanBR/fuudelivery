/**
 * Tela de Cardápio — Gerenciamento de produtos do restaurante.
 *
 * Lista categorias e produtos. Permite visualizar o cardápio atual.
 */
import React, { useState, useEffect, useCallback } from "react";
import {
  View,
  Text,
  StyleSheet,
  FlatList,
  TouchableOpacity,
  RefreshControl,
  Image,
} from "react-native";
import { useFocusEffect } from "expo-router/react-navigation";
import api from "@/services/api";
import { useApi } from "@/contexts/ApiContext";

interface Product {
  id: number;
  name: string;
  description?: string;
  price: number;
  image?: string;
  categories?: { id: number; name: string }[];
}

export default function MenuScreen() {
  const { getUserData } = useApi();
  const user = getUserData();
  const [products, setProducts] = useState<Product[]>([]);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);

  const fetchProducts = async () => {
    try {
      const establishmentId = user?.establishment_id;
      if (!establishmentId) return;
      const resp = await api.get(`/products/${establishmentId}`);
      setProducts(resp.data || []);
    } catch (e) {
      console.error("Erro ao carregar cardápio:", e);
    }
    setLoading(false);
  };

  useFocusEffect(
    useCallback(() => {
      fetchProducts();
    }, [user?.establishment_id])
  );

  const onRefresh = async () => {
    setRefreshing(true);
    await fetchProducts();
    setRefreshing(false);
  };

  const renderProduct = ({ item }: { item: Product }) => (
    <View style={styles.productCard}>
      {item.image ? (
        <Image source={{ uri: item.image }} style={styles.productImage} />
      ) : (
        <View style={[styles.productImage, styles.productImagePlaceholder]}>
          <Text style={styles.placeholderText}>🍽️</Text>
        </View>
      )}
      <View style={styles.productInfo}>
        <Text style={styles.productName}>{item.name}</Text>
        {item.description ? (
          <Text style={styles.productDescription} numberOfLines={2}>
            {item.description}
          </Text>
        ) : null}
        <Text style={styles.productPrice}>R$ {item.price?.toFixed(2)}</Text>
        {item.categories && item.categories.length > 0 && (
          <View style={styles.categoriesRow}>
            {item.categories.map((cat) => (
              <View key={cat.id} style={styles.categoryBadge}>
                <Text style={styles.categoryText}>{cat.name}</Text>
              </View>
            ))}
          </View>
        )}
      </View>
    </View>
  );

  return (
    <View style={styles.container}>
      {loading ? (
        <View style={styles.center}>
          <Text style={styles.loadingText}>Carregando cardápio...</Text>
        </View>
      ) : products.length === 0 ? (
        <View style={styles.center}>
          <Text style={styles.emptyEmoji}>🍽️</Text>
          <Text style={styles.emptyTitle}>Nenhum produto</Text>
          <Text style={styles.emptySubtitle}>
            Cadastre produtos pelo painel web
          </Text>
        </View>
      ) : (
        <FlatList
          data={products}
          keyExtractor={(item) => String(item.id)}
          renderItem={renderProduct}
          contentContainerStyle={styles.list}
          refreshControl={
            <RefreshControl refreshing={refreshing} onRefresh={onRefresh} />
          }
        />
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: "#F5F5F5" },
  list: { padding: 16, gap: 12 },
  center: { flex: 1, justifyContent: "center", alignItems: "center", gap: 12 },
  loadingText: { fontSize: 14, color: "#666" },
  emptyEmoji: { fontSize: 48 },
  emptyTitle: { fontSize: 18, fontWeight: "700", color: "#1A1A1A" },
  emptySubtitle: { fontSize: 14, color: "#666" },
  productCard: {
    flexDirection: "row",
    backgroundColor: "#FFF",
    borderRadius: 16,
    overflow: "hidden",
    borderWidth: 1,
    borderColor: "#F3F4F6",
  },
  productImage: { width: 90, height: 90 },
  productImagePlaceholder: {
    backgroundColor: "#F3F4F6",
    justifyContent: "center",
    alignItems: "center",
  },
  placeholderText: { fontSize: 32 },
  productInfo: { flex: 1, padding: 12 },
  productName: { fontSize: 15, fontWeight: "700", color: "#1A1A1A", marginBottom: 2 },
  productDescription: { fontSize: 12, color: "#6B7280", marginBottom: 4 },
  productPrice: { fontSize: 16, fontWeight: "700", color: "#DC2626" },
  categoriesRow: { flexDirection: "row", gap: 6, marginTop: 6, flexWrap: "wrap" },
  categoryBadge: {
    backgroundColor: "#FEF2F2",
    paddingHorizontal: 8,
    paddingVertical: 2,
    borderRadius: 8,
  },
  categoryText: { fontSize: 11, color: "#DC2626", fontWeight: "600" },
});
