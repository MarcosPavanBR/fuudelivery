import React, { useEffect, useState } from "react";
import {
  StyleSheet,
  TouchableOpacity,
  TextInput,
  ActivityIndicator,
  Alert,
} from "react-native";
import { useApi } from "@/contexts/ApiContext";
import { useCartApi } from "@/contexts/ApiCartContext";
import { Text, View } from "@/components/Themed";
import Colors from "@/constants/Colors";
import { Ionicons, MaterialIcons } from "@expo/vector-icons";
import Texts from "@/constants/Texts";
import { useNavigation } from "@react-navigation/native";
import api from "@/services/api";

export default function Perfil() {
  const { logout, getUserData, updateUser } = useApi();
  const { cleanCart, location } = useCartApi();
  const [user, setUser] = useState<any>(null);
  const nav = useNavigation();
  const [editingPhone, setEditingPhone] = useState(false);
  const [phoneInput, setPhoneInput] = useState("");
  const [savingPhone, setSavingPhone] = useState(false);

  async function init() {
    setUser(getUserData() as any);
  }

  useEffect(() => {
    init();
  }, []);

  const savePhone = async () => {
    const phone = phoneInput.trim();
    if (!user?.id) return;
    setSavingPhone(true);
    try {
      await api.put(`/users/${user.id}`, { phone });
      updateUser({ phone });
      setEditingPhone(false);
    } catch (e: any) {
      Alert.alert(
        "",
        e?.response?.data?.error || "Erro ao salvar telefone. Tente novamente."
      );
    } finally {
      setSavingPhone(false);
    }
  };

  const handleLogout = () => {
    logout();
    cleanCart();
  };

  return (
    <View style={styles.container}>
      <View style={styles.userDataContainer}>
        <View style={styles.infoBox}>
          <Text style={styles.label}>{Texts.nome_es}</Text>
          <Text style={styles.userInfo}>{user?.name}</Text>
        </View>

        <View style={styles.infoBox}>
          <Text style={styles.label}>{Texts.telefone}</Text>
          {editingPhone ? (
            <View>
              <TextInput
                style={styles.phoneInput}
                placeholder="Ex.: 11 99999-9999"
                placeholderTextColor={Colors.light.tabIconDefault}
                keyboardType="phone-pad"
                value={phoneInput}
                onChangeText={setPhoneInput}
              />
              <View style={styles.phoneActions}>
                <TouchableOpacity
                  onPress={() => {
                    setEditingPhone(false);
                    setPhoneInput(user?.phone || "");
                  }}
                >
                  <Text style={styles.cancelText}>Cancelar</Text>
                </TouchableOpacity>
                <TouchableOpacity onPress={savePhone} disabled={savingPhone}>
                  {savingPhone ? (
                    <ActivityIndicator size="small" color={Colors.light.tint} />
                  ) : (
                    <Text style={styles.saveText}>Salvar</Text>
                  )}
                </TouchableOpacity>
              </View>
            </View>
          ) : (
            <TouchableOpacity
              style={styles.phoneRow}
              onPress={() => {
                setPhoneInput(user?.phone || "");
                setEditingPhone(true);
              }}
            >
              <Text style={styles.userInfo}>{user?.phone || "Não informado"}</Text>
              <Ionicons name="pencil" size={20} style={styles.pencilIcon} />
            </TouchableOpacity>
          )}
        </View>

        {user?.email ? (
          <View style={styles.infoBox}>
            <Text style={styles.label}>E-mail</Text>
            <Text style={styles.userInfo}>{user?.email}</Text>
          </View>
        ) : null}

        <TouchableOpacity
          style={styles.addressBox}
          onPress={() => nav.navigate("location")}
        >
          <View style={{ width: "85%" }}>
            <Text style={styles.addressLabel}>{Texts.endereco}</Text>
            <View style={{ paddingTop: 10 }}>
              {location.logradouro ? (
                <Text style={styles.userInfo}>
                  {`${location?.logradouro} ${location?.bairro ?? ""}  Nº${
                    location?.numero ?? ""
                  }  ${location?.complemento ?? ""}`}
                </Text>
              ) : null}
              <Text style={styles.userInfo}>
                {location?.localidade} - {location?.uf}
              </Text>
            </View>
          </View>
          <View>
            <Ionicons name="pencil" size={20} style={styles.pencilIcon} />
          </View>
        </TouchableOpacity>
      </View>

      <View style={{ padding: 15 }}>
        <TouchableOpacity
          style={styles.onboardingButton}
          onPress={() => nav.navigate("onboarding")}
        >
          <Text style={styles.onboardingButtonText}>Cadastrar Restaurante</Text>
          <Ionicons name="restaurant" size={16} color={Colors.light.white} />
        </TouchableOpacity>
      </View>

      <View style={{ padding: 15 }}>
        <TouchableOpacity onPress={handleLogout} style={styles.logoutButton}>
          <Text style={styles.logoutButtonText}>Sair</Text>
          <MaterialIcons name="logout" size={16} color={Colors.light.white} />
        </TouchableOpacity>
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
  },
  title: {
    fontSize: 24,
    fontWeight: "bold",
    marginBottom: 20,
  },
  userDataContainer: {
    marginBottom: 20,
  },
  infoBox: {
    borderRadius: 3,
    borderColor: Colors.light.tabIconDefault,
    marginBottom: 10,
    padding: 15,
    borderBottomWidth: 1,
    paddingTop: 10,
  },
  label: {
    fontSize: 16,
    marginBottom: 10,
  },
  userInfo: {
    fontSize: 12,
    marginBottom: 10,
    color: Colors.light.secondaryText,
    paddingLeft: 3,
  },
  phoneRow: {
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "center",
  },
  phoneInput: {
    borderBottomWidth: 1,
    borderColor: Colors.light.tabIconDefault,
    fontSize: 16,
    padding: 8,
    color: Colors.light.text,
  },
  phoneActions: {
    flexDirection: "row",
    justifyContent: "flex-end",
    alignItems: "center",
    gap: 16,
    marginTop: 6,
  },
  cancelText: {
    fontSize: 14,
    color: Colors.light.secondaryText,
  },
  saveText: {
    fontSize: 14,
    fontWeight: "bold",
    color: Colors.light.tint,
  },
  addressBox: {
    borderRadius: 3,
    padding: 15,
    paddingTop: 5,
    paddingBottom: 0,
    borderBottomWidth: 1,
    flexDirection: "row",
    justifyContent: "space-between",
    borderColor: Colors.light.tabIconDefault,
    alignItems: "center",
  },
  addressLabel: {
    fontSize: 16,
  },
  pencilIcon: {
    color: Colors.light.secondaryText,
  },
  logoutButton: {
    backgroundColor: Colors.light.tint,
    padding: 10,
    flexDirection: "row",
    justifyContent: "space-between",
    borderRadius: 3,
    marginTop: "10%",
  },
  logoutButtonText: {
    color: Colors.light.white,
    fontSize: 14,
    fontWeight: "bold",
  },
  onboardingButton: {
    backgroundColor: Colors.light.tint,
    padding: 10,
    flexDirection: "row",
    justifyContent: "space-between",
    borderRadius: 3,
    marginBottom: 10,
  },
  onboardingButtonText: {
    color: Colors.light.white,
    fontSize: 14,
    fontWeight: "bold",
  },
});
