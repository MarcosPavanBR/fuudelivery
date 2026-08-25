import Colors from "@/constants/Colors";
import { useApi } from "@/contexts/ApiContext";
import api from "@/services/api";
import React, { useState } from "react";
import {
  View,
  TextInput,
  TouchableOpacity,
  Text,
  StyleSheet,
  SafeAreaView,
  Alert,
  ActivityIndicator,
  KeyboardAvoidingView,
  Platform,
} from "react-native";

const LoginScreen = () => {
  const [password, setPassword] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [isRegister, setIsRegister] = useState(false);
  const [name, setName] = useState("");
  const [phone, setPhone] = useState("");
  const { login } = useApi();

  const handleLogin = async () => {
    // Auth do cliente é por TELEFONE + senha (endpoints /clients/*).
    // NÃO usar /users/* — esses são para donos de restaurante.
    if (!phone.trim() || !password.trim()) {
      Alert.alert("", "Preencha telefone e senha.");
      return;
    }
    if (isRegister && !name.trim()) {
      Alert.alert("", "Preencha seu nome.");
      return;
    }

    setIsLoading(true);
    try {
      // Normaliza o telefone para só dígitos — garante que o valor do
      // login bata com o do registro independente da formatação digitada.
      const normalizedPhone = phone.replace(/\D/g, "");
      const endpoint = isRegister ? "/clients/register" : "/clients/login";
      const body = isRegister
        ? { name: name.trim(), phone: normalizedPhone, password }
        : { phone: normalizedPhone, password };

      const response = await api.post(endpoint, body);
      const { token } = response.data;

      if (token) {
        await login(token);
      } else {
        Alert.alert("", "Resposta inválida do servidor.");
      }
    } catch (error: any) {
      const msg =
        error?.response?.data?.error ||
        "Erro ao conectar com o servidor. Tente novamente.";
      Alert.alert("", msg);
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <SafeAreaView style={styles.container}>
      <KeyboardAvoidingView
        behavior={Platform.OS === "ios" ? "padding" : "height"}
        style={styles.keyboardView}
      >
        {/* Marca: wordmark FUUD ELIVERY (a Image com uri vazia reservava
            250px em branco e não exibia marca nenhuma). */}
        <View style={styles.brandBlock}>
          <Text style={styles.brandText}>
            <Text style={styles.brandFuud}>FUUD</Text>
            <Text style={styles.brandElivery}>ELIVERY</Text>
          </Text>
          <Text style={styles.brandTagline}>SABOR · RAPIDEZ · CONFIANÇA</Text>
        </View>
        <View style={styles.formContainer}>
          <Text style={styles.title}>
            {isRegister ? "Criar Conta" : "Entrar"}
          </Text>

          {isRegister && (
            <TextInput
              style={styles.input}
              placeholder="Nome"
              placeholderTextColor={Colors.light.tabIconDefault}
              onChangeText={setName}
              value={name}
              autoCapitalize="words"
            />
          )}

          <TextInput
            style={styles.input}
            placeholder="Telefone (ex.: 11 99999-9999)"
            placeholderTextColor={Colors.light.tabIconDefault}
            onChangeText={setPhone}
            value={phone}
            keyboardType="phone-pad"
          />

          <TextInput
            style={styles.input}
            placeholder="Senha"
            placeholderTextColor={Colors.light.tabIconDefault}
            onChangeText={setPassword}
            value={password}
            secureTextEntry
            autoCapitalize="none"
          />

          <TouchableOpacity
            style={[styles.loginButton, isLoading && styles.loginButtonDisabled]}
            onPress={handleLogin}
            disabled={isLoading}
          >
            {isLoading ? (
              <ActivityIndicator color={Colors.light.white} />
            ) : (
              <Text style={styles.loginButtonText}>
                {isRegister ? "Cadastrar" : "Entrar"}
              </Text>
            )}
          </TouchableOpacity>

          <TouchableOpacity
            style={styles.switchButton}
            onPress={() => setIsRegister(!isRegister)}
          >
            <Text style={styles.switchText}>
              {isRegister
                ? "Já tem conta? Entrar"
                : "Não tem conta? Cadastre-se"}
            </Text>
          </TouchableOpacity>
        </View>
      </KeyboardAvoidingView>
    </SafeAreaView>
  );
};

const styles = StyleSheet.create({
  container: {
    flex: 1,
    justifyContent: "center",
    alignItems: "center",
    backgroundColor: Colors.light.white,
  },
  keyboardView: {
    flex: 1,
    justifyContent: "center",
    alignItems: "center",
    width: "100%",
  },
  brandBlock: {
    alignItems: "center",
    marginBottom: 28,
  },
  brandText: {
    fontSize: 34,
    letterSpacing: 0.5,
  },
  brandFuud: {
    fontWeight: "900",
    color: Colors.light.primary,
  },
  brandElivery: {
    fontWeight: "700",
    color: Colors.light.text,
  },
  brandTagline: {
    fontSize: 11,
    letterSpacing: 4,
    color: Colors.light.secondaryText,
    marginTop: 4,
  },
  formContainer: {
    width: "90%",
    padding: 16,
    borderRadius: 2,
    backgroundColor: Colors.light.white,
    gap: 10,
  },
  title: {
    fontSize: 24,
    fontWeight: "bold",
    color: Colors.light.text,
    marginBottom: 8,
    textAlign: "center",
  },
  input: {
    borderBottomWidth: 2,
    borderColor: Colors.light.tabIconDefault,
    fontSize: 16,
    marginBottom: 12,
    padding: 10,
    color: Colors.light.text,
  },
  loginButton: {
    backgroundColor: Colors.light.tint,
    padding: 12,
    borderRadius: 2,
    alignItems: "center",
    marginTop: 8,
  },
  loginButtonDisabled: {
    opacity: 0.7,
  },
  loginButtonText: {
    color: Colors.light.white,
    fontSize: 16,
    fontWeight: "bold",
  },
  switchButton: {
    alignItems: "center",
    marginTop: 16,
  },
  switchText: {
    color: Colors.light.tint,
    fontSize: 14,
  },
});

export default LoginScreen;
