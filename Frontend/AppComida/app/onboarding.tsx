import React, { useState } from "react";
import {
  StyleSheet,
  ScrollView,
  TextInput,
  TouchableOpacity,
  ActivityIndicator,
  Alert,
  KeyboardAvoidingView,
  Platform,
} from "react-native";
import { useApi } from "@/contexts/ApiContext";
import { Text, View } from "@/components/Themed";
import Colors from "@/constants/Colors";
import { Ionicons } from "@expo/vector-icons";
import api from "@/services/api";
import { useNavigation } from "@react-navigation/native";

export default function OnboardingScreen() {
  const { getUserData } = useApi();
  const nav = useNavigation();
  const [loading, setLoading] = useState(false);

  const [form, setForm] = useState({
    name: "",
    description: "",
    address: "",
    city: "",
    state: "",
    phone: "",
    email: "",
    delivery_fee: "5.00",
    min_order: "20.00",
    delivery_time: "40",
  });

  const handleChange = (field: string, value: string) => {
    setForm((prev) => ({ ...prev, [field]: value }));
  };

  const handleSubmit = async () => {
    if (!form.name.trim()) {
      Alert.alert("Erro", "Nome do restaurante é obrigatório");
      return;
    }
    if (!form.address.trim()) {
      Alert.alert("Erro", "Endereço é obrigatório");
      return;
    }

    setLoading(true);
    try {
      const user = await getUserData();
      const res = await api.post("/establishments", {
        name: form.name.trim(),
        email: user?.phone || "",
        phone: form.phone.trim() || user?.phone || "",
        address: form.address.trim(),
        city: form.city.trim(),
        state: form.state.trim(),
        latitude: 0,
        longitude: 0,
        status: "open",
        delivery_fee: parseFloat(form.delivery_fee) || 5,
        min_order: parseFloat(form.min_order) || 20,
        delivery_time: parseInt(form.delivery_time, 10) || 40,
      });

      Alert.alert(
        "Restaurante criado!",
        "Seu restaurante foi cadastrado com sucesso. Você pode começar a receber pedidos.",
        [{ text: "OK", onPress: () => nav.goBack() }]
      );
    } catch (err: any) {
      Alert.alert(
        "Erro",
        err?.response?.data?.error || "Erro ao criar restaurante"
      );
    } finally {
      setLoading(false);
    }
  };

  return (
    <KeyboardAvoidingView
      style={styles.container}
      behavior={Platform.OS === "ios" ? "padding" : undefined}
    >
      <ScrollView contentContainerStyle={styles.scrollContent}>
        <View style={styles.header}>
          <TouchableOpacity onPress={() => nav.goBack()} style={styles.backButton}>
            <Ionicons name="arrow-back" size={24} color={Colors.light.text} />
          </TouchableOpacity>
          <Text style={styles.title}>Cadastrar Restaurante</Text>
          <Text style={styles.subtitle}>
            Preencha os dados do seu restaurante para começar a receber pedidos
          </Text>
        </View>

        <View style={styles.formSection}>
          <Text style={styles.sectionTitle}>Dados do Restaurante</Text>

          <Text style={styles.label}>Nome *</Text>
          <TextInput
            style={styles.input}
            value={form.name}
            onChangeText={(v) => handleChange("name", v)}
            placeholder="Ex: Restaurante Bom Sabor"
            placeholderTextColor={Colors.light.secondaryText}
          />

          <Text style={styles.label}>Descrição</Text>
          <TextInput
            style={[styles.input, styles.textArea]}
            value={form.description}
            onChangeText={(v) => handleChange("description", v)}
            placeholder="Breve descrição do restaurante"
            placeholderTextColor={Colors.light.secondaryText}
            multiline
            numberOfLines={3}
          />
        </View>

        <View style={styles.formSection}>
          <Text style={styles.sectionTitle}>Endereço</Text>

          <Text style={styles.label}>Endereço completo *</Text>
          <TextInput
            style={styles.input}
            value={form.address}
            onChangeText={(v) => handleChange("address", v)}
            placeholder="Rua, número, bairro"
            placeholderTextColor={Colors.light.secondaryText}
          />

          <View style={styles.row}>
            <View style={styles.halfField}>
              <Text style={styles.label}>Cidade</Text>
              <TextInput
                style={styles.input}
                value={form.city}
                onChangeText={(v) => handleChange("city", v)}
                placeholder="São Paulo"
                placeholderTextColor={Colors.light.secondaryText}
              />
            </View>
            <View style={styles.halfField}>
              <Text style={styles.label}>Estado</Text>
              <TextInput
                style={styles.input}
                value={form.state}
                onChangeText={(v) => handleChange("state", v)}
                placeholder="SP"
                placeholderTextColor={Colors.light.secondaryText}
                maxLength={2}
              />
            </View>
          </View>
        </View>

        <View style={styles.formSection}>
          <Text style={styles.sectionTitle}>Entrega</Text>

          <View style={styles.row}>
            <View style={styles.halfField}>
              <Text style={styles.label}>Taxa de entrega (R$)</Text>
              <TextInput
                style={styles.input}
                value={form.delivery_fee}
                onChangeText={(v) => handleChange("delivery_fee", v)}
                keyboardType="decimal-pad"
                placeholder="5.00"
                placeholderTextColor={Colors.light.secondaryText}
              />
            </View>
            <View style={styles.halfField}>
              <Text style={styles.label}>Pedido mínimo (R$)</Text>
              <TextInput
                style={styles.input}
                value={form.min_order}
                onChangeText={(v) => handleChange("min_order", v)}
                keyboardType="decimal-pad"
                placeholder="20.00"
                placeholderTextColor={Colors.light.secondaryText}
              />
            </View>
          </View>

          <Text style={styles.label}>Tempo estimado de entrega (min)</Text>
          <TextInput
            style={styles.input}
            value={form.delivery_time}
            onChangeText={(v) => handleChange("delivery_time", v)}
            keyboardType="number-pad"
            placeholder="40"
            placeholderTextColor={Colors.light.secondaryText}
          />
        </View>

        <TouchableOpacity
          style={[styles.submitButton, loading && styles.submitButtonDisabled]}
          onPress={handleSubmit}
          disabled={loading}
        >
          {loading ? (
            <ActivityIndicator color={Colors.light.white} />
          ) : (
            <>
              <Ionicons name="checkmark-circle" size={20} color={Colors.light.white} />
              <Text style={styles.submitButtonText}>Cadastrar Restaurante</Text>
            </>
          )}
        </TouchableOpacity>

        <Text style={styles.helperText}>
          Após o cadastro, você poderá configurar sua conta de recebimento na aba "Carteira"
          para receber pagamentos diretamente na sua conta bancária.
        </Text>

        <View style={{ height: 40 }} />
      </ScrollView>
    </KeyboardAvoidingView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: Colors.light.background },
  scrollContent: { padding: 16 },
  header: { marginBottom: 20 },
  backButton: { marginBottom: 12 },
  title: { fontSize: 24, fontWeight: "bold", color: Colors.light.text },
  subtitle: { fontSize: 14, color: Colors.light.secondaryText, marginTop: 4 },
  formSection: {
    backgroundColor: Colors.light.white,
    borderRadius: 8,
    padding: 16,
    marginBottom: 12,
  },
  sectionTitle: {
    fontSize: 16,
    fontWeight: "bold",
    color: Colors.light.tint,
    marginBottom: 12,
  },
  label: {
    fontSize: 13,
    fontWeight: "600",
    color: Colors.light.text,
    marginBottom: 4,
    marginTop: 8,
  },
  input: {
    borderWidth: 1,
    borderColor: Colors.light.tabIconDefault,
    borderRadius: 6,
    padding: 12,
    fontSize: 14,
    color: Colors.light.text,
    backgroundColor: Colors.light.background,
  },
  textArea: { height: 80, textAlignVertical: "top" },
  row: { flexDirection: "row", gap: 12 },
  halfField: { flex: 1 },
  submitButton: {
    flexDirection: "row",
    backgroundColor: Colors.light.tint,
    padding: 16,
    borderRadius: 8,
    justifyContent: "center",
    alignItems: "center",
    gap: 8,
    marginTop: 8,
  },
  submitButtonDisabled: { opacity: 0.6 },
  submitButtonText: { color: Colors.light.white, fontSize: 16, fontWeight: "bold" },
  helperText: {
    fontSize: 12,
    color: Colors.light.secondaryText,
    textAlign: "center",
    marginTop: 12,
  },
});
