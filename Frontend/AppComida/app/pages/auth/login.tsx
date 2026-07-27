import Colors from "@/constants/Colors";
import { useApi } from "@/contexts/ApiContext";
import helpers from "@/helpers/helpers";
import api from "@/services/api";
import React, { useState } from "react";
import {
  View,
  TextInput,
  TouchableOpacity,
  Text,
  Image,
  StyleSheet,
  SafeAreaView,
  Alert,
  ActivityIndicator,
} from "react-native";

const LoginScreen = () => {
  const [phone, setPhone] = useState("");
  const [password, setPassword] = useState("");
  const [name, setName] = useState("");
  const [isRegistering, setIsRegistering] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const { login } = useApi();

  const handleLogin = async () => {
    const cleanPhone = phone.replace(/\D/g, "");
    if (cleanPhone.length < 10) {
      Alert.alert("Erro", "Telefone inválido. Digite um número com DDD.");
      return;
    }
    if (password.length < 6) {
      Alert.alert("Erro", "A senha deve ter pelo menos 6 caracteres.");
      return;
    }
    if (isRegistering && name.trim().length < 2) {
      Alert.alert("Erro", "Digite seu nome.");
      return;
    }

    setIsLoading(true);

    try {
      let response;

      if (isRegistering) {
        // Cadastro: cria conta e já retorna token
        response = await api.post("/clients/register", {
          name: name.trim(),
          phone: cleanPhone,
          password,
        });
      } else {
        // Login: autentica com telefone + senha
        response = await api.post("/clients/login", {
          phone: cleanPhone,
          password,
        });
      }

      const { token, user } = response.data;

      // Armazena o JWT real e dados do usuário
      await login(token, {
        id: user.id,
        name: user.name,
        phone: user.phone,
      });
    } catch (error: any) {
      const message =
        error.response?.data?.error ||
        "Erro de conexão. Verifique sua internet.";
      Alert.alert("Erro", message);
    } finally {
      setIsLoading(false);
    }
  };

  const toggleMode = () => {
    setIsRegistering(!isRegistering);
    setName("");
    setPassword("");
  };

  return (
    <SafeAreaView style={styles.container}>
      <View style={styles.headerContainer}>
        <Text style={styles.title}>FuuDelivery</Text>
        <Text style={styles.subtitle}>
          {isRegistering ? "Criar sua conta" : "Entrar na sua conta"}
        </Text>
      </View>

      <View style={styles.formContainer}>
        {isRegistering && (
          <TextInput
            style={styles.input}
            onChangeText={setName}
            value={name}
            placeholder="Seu nome"
            placeholderTextColor={Colors.light.tabIconDefault}
            autoCapitalize="words"
          />
        )}

        <TextInput
          style={styles.input}
          onChangeText={(text) => setPhone(text.replace(/\D/g, "").slice(0, 11))}
          keyboardType="phone-pad"
          value={helpers.formatPhoneNumber(phone)}
          placeholder="Telefone com DDD"
          placeholderTextColor={Colors.light.tabIconDefault}
        />

        <TextInput
          style={styles.input}
          onChangeText={setPassword}
          value={password}
          placeholder="Senha"
          placeholderTextColor={Colors.light.tabIconDefault}
          secureTextEntry
        />

        <TouchableOpacity
          style={[styles.loginButton, isLoading && styles.buttonDisabled]}
          onPress={handleLogin}
          disabled={isLoading}
        >
          {isLoading ? (
            <ActivityIndicator color={Colors.light.white} />
          ) : (
            <Text style={styles.loginButtonText}>
              {isRegistering ? "Criar Conta" : "Entrar"}
            </Text>
          )}
        </TouchableOpacity>

        <TouchableOpacity style={styles.toggleButton} onPress={toggleMode}>
          <Text style={styles.toggleButtonText}>
            {isRegistering
              ? "Já tem conta? Faça login"
              : "Não tem conta? Cadastre-se"}
          </Text>
        </TouchableOpacity>
      </View>
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
  headerContainer: {
    alignItems: "center",
    marginBottom: 32,
  },
  title: {
    fontSize: 36,
    fontWeight: "900",
    color: Colors.light.tint,
    letterSpacing: 1,
  },
  subtitle: {
    fontSize: 16,
    color: Colors.light.secondaryText,
    marginTop: 8,
  },
  formContainer: {
    width: "85%",
    padding: 16,
    gap: 12,
  },
  input: {
    borderBottomWidth: 2,
    borderColor: Colors.light.tabIconDefault,
    fontSize: 16,
    padding: 12,
    color: Colors.light.text,
  },
  loginButton: {
    backgroundColor: Colors.light.tint,
    padding: 14,
    borderRadius: 8,
    alignItems: "center",
    marginTop: 8,
  },
  buttonDisabled: {
    opacity: 0.7,
  },
  loginButtonText: {
    color: Colors.light.white,
    fontSize: 16,
    fontWeight: "bold",
  },
  toggleButton: {
    alignItems: "center",
    padding: 12,
  },
  toggleButtonText: {
    color: Colors.light.info,
    fontSize: 14,
  },
});

export default LoginScreen;
