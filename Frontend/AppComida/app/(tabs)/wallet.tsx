import React, { useEffect, useState, useCallback } from "react";
import {
  StyleSheet,
  TouchableOpacity,
  ScrollView,
  RefreshControl,
  Alert,
  ActivityIndicator,
  Share,
} from "react-native";
import { useApi } from "@/contexts/ApiContext";
import { Text, View } from "@/components/Themed";
import Colors from "@/constants/Colors";
import { Ionicons } from "@expo/vector-icons";
import api from "@/services/api";
import { useCartApi } from "@/contexts/ApiCartContext";

export default function WalletScreen() {
  const { getUserData } = useApi();
  // Cupom resgatado/validado já vai para o carrinho (antes o cliente tinha
  // de anotar o código e digitar no checkout).
  const { setCouponCode: setCartCoupon } = useCartApi();
  const [referral, setReferral] = useState<any>(null);
  const [user, setUser] = useState<any>(null);
  const [walletBalance, setWalletBalance] = useState<number>(0);
  const [loyalty, setLoyalty] = useState<any>({ points: 0, tier: "bronze", total_orders: 0, total_spent: 0 });
  const [history, setHistory] = useState<any[]>([]);
  const [redeemPoints, setRedeemPoints] = useState<string>("10");
  const [couponCode, setCouponCode] = useState<string>("");
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [redeeming, setRedeeming] = useState(false);

  const fetchData = useCallback(async () => {
    try {
      const u = getUserData();
      if (!u?.id) return;
      setUser(u);

      // /wallet/balance espera user_id; /loyalty/* espera o telefone (user_phone).
      const userId = String(u.id);
      const userPhone = u.phone ? String(u.phone) : userId;
      const [walletRes, loyaltyRes, historyRes, referralRes] = await Promise.all([
        api.get(`/wallets/balance/${userId}`).catch(() => null),
        api.get(`/loyalty/balance/${userPhone}`).catch(() => null),
        api.get(`/loyalty/history/${userPhone}`).catch(() => null),
        api.get("/referral/me").catch(() => null),
      ]);
      if (referralRes?.data?.code) setReferral(referralRes.data);

      if (walletRes?.data?.balance !== undefined) {
        setWalletBalance(walletRes.data.balance);
      }
      if (loyaltyRes?.data) {
        setLoyalty(loyaltyRes.data);
      }
      if (historyRes?.data) {
        // O servidor devolve o modelo Go sem tags JSON (Description,
        // CreatedAt, Type, Points); a tela lia as minúsculas e o histórico
        // saía sem texto e com "Invalid Date".
        setHistory(
          (Array.isArray(historyRes.data) ? historyRes.data : []).map((t: any) => ({
            description: t.description ?? t.Description ?? "",
            created_at: t.created_at ?? t.CreatedAt,
            type: t.type ?? t.Type,
            points: Number(t.points ?? t.Points ?? 0),
          }))
        );
      }
    } catch (err) {
      console.error("Wallet fetch error:", err);
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  }, []);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const onRefresh = useCallback(() => {
    setRefreshing(true);
    fetchData();
  }, [fetchData]);

  const handleRedeem = async () => {
    const points = parseInt(redeemPoints, 10);
    if (!points || points < 10 || points % 10 !== 0) {
      Alert.alert("Erro", "Os pontos devem ser múltiplos de 10 (mínimo 10)");
      return;
    }
    setRedeeming(true);
    try {
      const res = await api.post("/loyalty/redeem", {
        user_phone: String(user?.phone || user?.id || ""),
        points,
        order_id: "",
      });
      if (res.data.coupon_code) {
        setCartCoupon(res.data.coupon_code);
        Alert.alert(
          "Cashback resgatado!",
          `Cupom ${res.data.coupon_code} de R$ ${Number(res.data.discount_value).toFixed(2).replace(".", ",")} já está no seu carrinho: o desconto entra no próximo pedido.\nValidade: ${res.data.coupon_expires}`,
          [{ text: "OK" }]
        );
        fetchData();
      }
    } catch (err: any) {
      Alert.alert("Erro", err?.response?.data?.error || "Erro ao resgatar pontos");
    } finally {
      setRedeeming(false);
    }
  };

  const handleValidateCoupon = async () => {
    if (!couponCode.trim()) {
      Alert.alert("Erro", "Digite um código de cupom");
      return;
    }
    try {
      const res = await api.post("/coupons/validate", {
        code: couponCode.trim().toUpperCase(),
        user_phone: String(user?.phone || user?.id || ""),
        order_value: 0,
      });
      if (res.data?.valid) {
        setCartCoupon(couponCode.trim().toUpperCase());
        Alert.alert(
          "Cupom válido!",
          `Desconto: ${res.data.discount_type === "FIXED" ? `R$ ${res.data.discount_value.toFixed(2)}` : `${res.data.discount_value}%`}. Já está no seu carrinho.`,
          [{ text: "OK" }]
        );
      } else {
        Alert.alert("Cupom inválido", res.data?.message || "Cupom não encontrado");
      }
    } catch (err: any) {
      Alert.alert("Erro", err?.response?.data?.error || "Erro ao validar cupom");
    }
  };

  if (loading) {
    return (
      <View style={styles.centerContainer}>
        <ActivityIndicator size="large" color={Colors.light.tint} />
      </View>
    );
  }

  return (
    <ScrollView
      style={styles.container}
      refreshControl={<RefreshControl refreshing={refreshing} onRefresh={onRefresh} />}
    >
      <View style={styles.section}>
        <View style={styles.sectionHeader}>
          <Ionicons name="wallet" size={24} color={Colors.light.tint} />
          <Text style={styles.sectionTitle}>Carteira</Text>
        </View>
        <Text style={styles.balanceValue}>R$ {walletBalance.toFixed(2)}</Text>
        <Text style={styles.balanceLabel}>Saldo disponível</Text>
      </View>

      <View style={styles.section}>
        <View style={styles.sectionHeader}>
          <Ionicons name="star" size={24} color={Colors.light.tint} />
          <Text style={styles.sectionTitle}>Pontos e Fidelidade</Text>
        </View>
        <View style={styles.loyaltyRow}>
          <View style={styles.loyaltyItem}>
            <Text style={styles.loyaltyValue}>{loyalty.points}</Text>
            <Text style={styles.loyaltyLabel}>Pontos</Text>
          </View>
          <View style={styles.loyaltyItem}>
            <Text style={styles.loyaltyValue}>{loyalty.tier?.toUpperCase()}</Text>
            <Text style={styles.loyaltyLabel}>Nível</Text>
          </View>
          <View style={styles.loyaltyItem}>
            <Text style={styles.loyaltyValue}>{loyalty.total_orders}</Text>
            <Text style={styles.loyaltyLabel}>Pedidos</Text>
          </View>
        </View>
        <Text style={styles.helperText}>10 pontos = R$ 1,00 de desconto</Text>

        <View style={styles.redeemRow}>
          <Text style={styles.redeemLabel}>Pontos para resgatar:</Text>
          <View style={styles.redeemInputRow}>
            {[10, 50, 100].map((p) => (
              <TouchableOpacity
                key={p}
                style={[
                  styles.presetButton,
                  redeemPoints === String(p) && styles.presetButtonActive,
                ]}
                onPress={() => setRedeemPoints(String(p))}
              >
                <Text
                  style={[
                    styles.presetText,
                    redeemPoints === String(p) && styles.presetTextActive,
                  ]}
                >
                  {p}
                </Text>
              </TouchableOpacity>
            ))}
          </View>
        </View>

        <TouchableOpacity
          style={styles.redeemButton}
          onPress={handleRedeem}
          disabled={redeeming}
        >
          {redeeming ? (
            <ActivityIndicator color={Colors.light.white} />
          ) : (
            <>
              <Text style={styles.redeemButtonText}>
                Resgatar ({parseInt(redeemPoints, 10) / 10} R$)
              </Text>
              <Ionicons name="arrow-forward" size={18} color={Colors.light.white} />
            </>
          )}
        </TouchableOpacity>
      </View>

      {referral ? (
        <View style={styles.section}>
          <View style={styles.sectionHeader}>
            <Ionicons name="gift" size={24} color={Colors.light.tint} />
            <Text style={styles.sectionTitle}>Indique e ganhe</Text>
          </View>
          <Text style={styles.referralText}>
            Seu amigo ganha R$ {Number(referral.welcome_value).toFixed(0)} no primeiro pedido (a partir de R${" "}
            {Number(referral.min_order).toFixed(0)}). Quando o pedido dele for entregue, você ganha{" "}
            {referral.reward_points} pontos (R$ {(referral.reward_points / 10).toFixed(0)} de desconto).
          </Text>
          <View style={styles.referralCodeBox}>
            <Text style={styles.referralCode}>{referral.code}</Text>
            <TouchableOpacity
              style={styles.validateButton}
              onPress={() => Share.share({ message: referral.share_text }).catch(() => {})}
            >
              <Text style={styles.validateButtonText}>Compartilhar</Text>
            </TouchableOpacity>
          </View>
          <Text style={styles.referralStats}>
            {referral.invited} amigo(s) usaram seu código · {referral.rewarded} já renderam pontos
          </Text>
        </View>
      ) : null}

      <View style={styles.section}>
        <View style={styles.sectionHeader}>
          <Ionicons name="pricetag" size={24} color={Colors.light.tint} />
          <Text style={styles.sectionTitle}>Cupom</Text>
        </View>
        <View style={styles.couponRow}>
          <TextInput
            style={styles.couponInput}
            placeholder="Digite o código do cupom"
            value={couponCode}
            onChangeText={setCouponCode}
            autoCapitalize="characters"
          />
          <TouchableOpacity style={styles.validateButton} onPress={handleValidateCoupon}>
            <Text style={styles.validateButtonText}>Validar</Text>
          </TouchableOpacity>
        </View>
      </View>

      {history.length > 0 && (
        <View style={styles.section}>
          <View style={styles.sectionHeader}>
            <Ionicons name="time" size={24} color={Colors.light.tint} />
            <Text style={styles.sectionTitle}>Histórico de Pontos</Text>
          </View>
          {history.slice(0, 10).map((item, idx) => (
            <View key={idx} style={styles.historyItem}>
              <View style={{ flex: 1 }}>
                <Text style={styles.historyDescription}>{item.description}</Text>
                <Text style={styles.historyDate}>
                  {new Date(item.created_at).toLocaleDateString("pt-BR")}
                </Text>
              </View>
              <Text
                style={[
                  styles.historyPoints,
                  item.type === "earn" ? styles.historyEarn : styles.historyRedeem,
                ]}
              >
                {item.type === "earn" ? "+" : "-"}
                {Math.abs(item.points)}
              </Text>
            </View>
          ))}
        </View>
      )}

      <View style={{ height: 30 }} />
    </ScrollView>
  );
}

import { TextInput } from "react-native";

const styles = StyleSheet.create({
  referralText: { fontSize: 13, color: Colors.light.secondaryText, marginBottom: 10 },
  referralCodeBox: { flexDirection: "row", alignItems: "center", justifyContent: "space-between", gap: 10 },
  referralCode: { fontSize: 22, fontWeight: "700", letterSpacing: 2, color: Colors.light.text },
  referralStats: { fontSize: 12, color: Colors.light.secondaryText, marginTop: 8 },
  container: { flex: 1, backgroundColor: Colors.light.background },
  centerContainer: { flex: 1, justifyContent: "center", alignItems: "center" },
  section: {
    backgroundColor: Colors.light.white,
    marginHorizontal: 12,
    marginTop: 12,
    borderRadius: 8,
    padding: 16,
  },
  sectionHeader: { flexDirection: "row", alignItems: "center", gap: 8, marginBottom: 12 },
  sectionTitle: { fontSize: 18, fontWeight: "bold", color: Colors.light.text },
  balanceValue: { fontSize: 36, fontWeight: "bold", color: Colors.light.tint },
  balanceLabel: { fontSize: 14, color: Colors.light.secondaryText, marginTop: 4 },
  loyaltyRow: { flexDirection: "row", justifyContent: "space-around", marginBottom: 10 },
  loyaltyItem: { alignItems: "center" },
  loyaltyValue: { fontSize: 22, fontWeight: "bold", color: Colors.light.text },
  loyaltyLabel: { fontSize: 12, color: Colors.light.secondaryText, marginTop: 2 },
  helperText: { fontSize: 12, color: Colors.light.secondaryText, textAlign: "center" },
  redeemRow: { flexDirection: "row", justifyContent: "space-between", alignItems: "center", marginTop: 12 },
  redeemLabel: { fontSize: 14, color: Colors.light.text },
  redeemInputRow: { flexDirection: "row", gap: 8 },
  presetButton: {
    paddingVertical: 6,
    paddingHorizontal: 14,
    borderRadius: 4,
    backgroundColor: Colors.light.background,
  },
  presetButtonActive: { backgroundColor: Colors.light.tint },
  presetText: { fontSize: 14, color: Colors.light.text, fontWeight: "600" },
  presetTextActive: { color: Colors.light.white },
  redeemButton: {
    flexDirection: "row",
    backgroundColor: Colors.light.tint,
    padding: 14,
    borderRadius: 8,
    justifyContent: "center",
    alignItems: "center",
    gap: 8,
    marginTop: 12,
  },
  redeemButtonText: { color: Colors.light.white, fontSize: 16, fontWeight: "bold" },
  couponRow: { flexDirection: "row", gap: 8 },
  couponInput: {
    flex: 1,
    borderWidth: 1,
    borderColor: Colors.light.tabIconDefault,
    borderRadius: 6,
    padding: 12,
    fontSize: 14,
  },
  validateButton: {
    backgroundColor: Colors.light.tint,
    padding: 12,
    borderRadius: 6,
    justifyContent: "center",
  },
  validateButtonText: { color: Colors.light.white, fontWeight: "bold" },
  historyItem: {
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "center",
    paddingVertical: 10,
    borderBottomWidth: 1,
    borderColor: Colors.light.tabIconDefault,
  },
  historyDescription: { fontSize: 14, color: Colors.light.text },
  historyDate: { fontSize: 12, color: Colors.light.secondaryText, marginTop: 2 },
  historyPoints: { fontSize: 16, fontWeight: "bold" },
  historyEarn: { color: "green" },
  historyRedeem: { color: "red" },
});
