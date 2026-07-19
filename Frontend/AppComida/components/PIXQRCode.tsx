import React from "react";
import { View, Text, StyleSheet, TouchableOpacity, Image } from "react-native";
import * as Clipboard from "expo-clipboard";
import { Ionicons } from "@expo/vector-icons";

interface PIXQRCodeProps {
  qrCodeBase64: string;
  copyPaste: string;
  expiresIn: number;
}

const PIXQRCode: React.FC<PIXQRCodeProps> = ({ qrCodeBase64, copyPaste, expiresIn }) => {
  const copyToClipboard = async () => {
    await Clipboard.setStringAsync(copyPaste);
  };

  return (
    <View style={styles.container}>
      <Text style={styles.title}>Pague com PIX</Text>
      <Text style={styles.subtitle}>Escaneie o QR Code ou copie o codigo</Text>

      {qrCodeBase64 ? (
        <Image
          source={{ uri: `data:image/png;base64,${qrCodeBase64}` }}
          style={styles.qrcode}
        />
      ) : (
        <View style={[styles.qrcode, styles.qrcodePlaceholder]}>
          <Ionicons name="qr-code" size={80} color="#F97316" />
        </View>
      )}

      <TouchableOpacity style={styles.copyButton} onPress={copyToClipboard}>
        <Ionicons name="copy-outline" size={18} color="#FFF" />
        <Text style={styles.copyText}>Copiar codigo PIX</Text>
      </TouchableOpacity>

      <Text style={styles.expiry}>
        Expira em {Math.floor(expiresIn / 60)} min
      </Text>
    </View>
  );
};

const styles = StyleSheet.create({
  container: { alignItems: "center", padding: 20 },
  title: { fontSize: 20, fontWeight: "700", marginBottom: 4 },
  subtitle: { fontSize: 14, color: "#666", marginBottom: 20, textAlign: "center" },
  qrcode: { width: 200, height: 200, borderRadius: 12, marginBottom: 20 },
  qrcodePlaceholder: { backgroundColor: "#FFF", justifyContent: "center", alignItems: "center", borderWidth: 2, borderColor: "#F97316", borderStyle: "dashed" },
  copyButton: { flexDirection: "row", alignItems: "center", gap: 8, backgroundColor: "#F97316", paddingVertical: 12, paddingHorizontal: 24, borderRadius: 8 },
  copyText: { color: "#FFF", fontWeight: "600", fontSize: 15 },
  expiry: { marginTop: 12, fontSize: 12, color: "#999" },
});

export default PIXQRCode;
