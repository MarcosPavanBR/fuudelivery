import React, { useEffect, useRef, useState } from "react";
import {
  Modal,
  View,
  Text,
  StyleSheet,
  TouchableOpacity,
  Share,
  Image,
} from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import Colors from "@/constants/Colors";
import { FontAwesome } from "@expo/vector-icons";

interface QRCodeModalProps {
  visible: boolean;
  onClose: () => void;
  qrData: string;
  tableNumber: string;
  establishmentName?: string;
}

function generateQRCodeSVG(data: string): string {
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 200">
    <rect width="200" height="200" fill="white"/>
    <rect x="20" y="20" width="160" height="160" fill="black"/>
    <rect x="30" y="30" width="140" height="140" fill="white"/>
    <rect x="40" y="40" width="40" height="40" fill="black"/>
    <rect x="100" y="40" width="40" height="40" fill="black"/>
    <rect x="40" y="100" width="40" height="40" fill="black"/>
    <rect x="100" y="100" width="40" height="40" fill="black"/>
    <rect x="160" y="40" width="10" height="10" fill="black"/>
    <rect x="40" y="160" width="10" height="10" fill="black"/>
    <rect x="160" y="160" width="10" height="10" fill="black"/>
  </svg>`;
}

export default function QRCodeModal({
  visible,
  onClose,
  qrData,
  tableNumber,
  establishmentName,
}: QRCodeModalProps) {
  const insets = useSafeAreaInsets();

  const handleShare = async () => {
    try {
      await Share.share({
        message: `Mesa ${tableNumber} - ${establishmentName || "Restaurante"}\n\nEscaneie o QR Code para fazer o pedido:\n${qrData}`,
        title: `Mesa ${tableNumber}`,
      });
    } catch (error) {
      console.log("Erro ao compartilhar:", error);
    }
  };

  return (
    <Modal
      visible={visible}
      animationType="slide"
      transparent={true}
      onRequestClose={onClose}
    >
      <View style={styles.overlay}>
        <View style={[styles.container, { paddingTop: insets.top + 20 }]}>
          <TouchableOpacity style={styles.closeButton} onPress={onClose}>
            <FontAwesome name="times" size={24} color={Colors.light.tint} />
          </TouchableOpacity>

          <Text style={styles.title}>QR Code da Mesa</Text>

          {tableNumber && (
            <Text style={styles.tableText}>Mesa {tableNumber}</Text>
          )}

          <View style={styles.qrContainer}>
            <Image
              source={{
                uri: `data:image/svg+xml;utf8,${encodeURIComponent(generateQRCodeSVG(qrData))}`,
              }}
              style={styles.qrImage}
              resizeMode="contain"
            />
          </View>

          <Text style={styles.qrDataText} numberOfLines={2}>
            {qrData}
          </Text>

          <TouchableOpacity style={styles.shareButton} onPress={handleShare}>
            <FontAwesome
              name="share"
              size={18}
              color={Colors.light.white}
              style={{ marginRight: 8 }}
            />
            <Text style={styles.shareText}>Compartilhar</Text>
          </TouchableOpacity>
        </View>
      </View>
    </Modal>
  );
}

const styles = StyleSheet.create({
  overlay: {
    flex: 1,
    backgroundColor: "rgba(0,0,0,0.5)",
    justifyContent: "center",
    alignItems: "center",
  },
  container: {
    backgroundColor: Colors.light.white,
    borderRadius: 16,
    padding: 24,
    width: "85%",
    alignItems: "center",
    shadowColor: "#000",
    shadowOffset: { width: 0, height: 2 },
    shadowOpacity: 0.25,
    shadowRadius: 4,
    elevation: 5,
  },
  closeButton: {
    position: "absolute",
    top: 16,
    right: 16,
    zIndex: 1,
    padding: 8,
  },
  title: {
    fontSize: 20,
    fontWeight: "bold",
    color: Colors.light.tint,
    marginBottom: 8,
  },
  tableText: {
    fontSize: 16,
    color: Colors.light.secondaryText,
    marginBottom: 16,
  },
  qrContainer: {
    width: 200,
    height: 200,
    backgroundColor: Colors.light.white,
    borderRadius: 12,
    padding: 8,
    borderWidth: 2,
    borderColor: Colors.light.tint,
    justifyContent: "center",
    alignItems: "center",
    marginBottom: 16,
  },
  qrImage: {
    width: 180,
    height: 180,
  },
  qrDataText: {
    fontSize: 12,
    color: Colors.light.secondaryText,
    textAlign: "center",
    marginBottom: 16,
  },
  shareButton: {
    flexDirection: "row",
    backgroundColor: Colors.light.tint,
    paddingVertical: 12,
    paddingHorizontal: 24,
    borderRadius: 8,
    alignItems: "center",
    justifyContent: "center",
  },
  shareText: {
    color: Colors.light.white,
    fontSize: 16,
    fontWeight: "600",
  },
});
