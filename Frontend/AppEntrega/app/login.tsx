import Colors from "@/constants/Colors";
import { useAuthApi } from "@/contexts/AuthContext";

import React, { useState } from "react";
import {
  View,
  TextInput,
  TouchableOpacity,
  Text,
  StyleSheet,
  SafeAreaView,
  ActivityIndicator,
  Alert,
} from "react-native";
import RegisterScreen from "./cadastro";

const LoginScreen = () => {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const { login } = useAuthApi();
  const [register, setRegister] = useState(false);
  const [load, setLoad] = useState(false);

  const handleLogin = async () => {
    if (!email.trim() || !password.trim()) return;
    setLoad(true);
    try {
      await login(email, password);
    } catch (e: any) {
      Alert.alert(
        "",
        e?.response?.data?.error ||
          "E-mail ou senha inválidos. Tente novamente."
      );
    } finally {
      setLoad(false);
    }
  };
  if (register) {
    return <RegisterScreen setRegister={setRegister} />;
  }

  return (
    <SafeAreaView style={styles.container}>
      {/* Marca: wordmark FUUD ELIVERY (o app não exibia marca nenhuma). */}
      <View style={styles.brandBlock}>
        <Text style={styles.brandText}>
          <Text style={styles.brandFuud}>FUUD</Text>
          <Text style={styles.brandElivery}>ELIVERY</Text>
        </Text>
        <Text style={styles.brandTagline}>SABOR · RAPIDEZ · CONFIANÇA</Text>
      </View>
      <View style={styles.formContainer}>
        <TextInput
          style={styles.input}
          onChangeText={setEmail}
          value={email}
          placeholder="Email"
          autoCapitalize="none"
          keyboardType="email-address"
          placeholderTextColor={Colors.light.tabIconDefault}
        />
        <TextInput
          style={styles.input}
          onChangeText={setPassword}
          value={password}
          placeholder="Senha"
          secureTextEntry={true}
          autoCapitalize="none"
          placeholderTextColor={Colors.light.tabIconDefault}
        />
        <TouchableOpacity
          disabled={load}
          style={styles.loginButton}
          onPress={handleLogin}
        >
          {!load ? (
            <Text style={styles.loginButtonText}>Entrar</Text>
          ) : (
            <ActivityIndicator
              size={20}
              color={Colors.light.white}
              style={{ alignSelf: "center" }}
            />
          )}
        </TouchableOpacity>

        <TouchableOpacity
          style={{
            ...styles.loginButton,
            backgroundColor: Colors.light.white,
            borderWidth: 1,
            borderColor: Colors.light.tint,
            marginTop: 20,
          }}
          onPress={() => setRegister(true)}
        >
          <Text style={{ ...styles.loginButtonText, color: Colors.light.tint }}>
            Novo Entregador
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
  input: {
    borderBottomWidth: 2,
    borderColor: Colors.light.tabIconDefault,
    fontSize: 16,
    marginBottom: 12,
    padding: 10,
  },
  loginButton: {
    backgroundColor: Colors.light.tint,
    padding: 8,
    borderRadius: 2,
    alignItems: "center",
  },
  loginButtonText: {
    color: Colors.light.white,
    fontSize: 16,
    fontWeight: "bold",
  },
});

export default LoginScreen;
