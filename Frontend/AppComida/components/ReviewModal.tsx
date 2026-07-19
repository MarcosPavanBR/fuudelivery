import React, { useState } from "react";
import { View, Text, TextInput, TouchableOpacity, StyleSheet, Modal, Alert } from "react-native";
import { Ionicons } from "@expo/vector-icons";
import Colors from "@/constants/Colors";
import api from "@/services/api";

interface ReviewModalProps {
  visible: boolean;
  onClose: () => void;
  orderId: string;
  establishmentId: number;
  userPhone: string;
  userName: string;
}

const ReviewModal: React.FC<ReviewModalProps> = ({
  visible, onClose, orderId, establishmentId, userPhone, userName
}) => {
  const [rating, setRating] = useState(0);
  const [comment, setComment] = useState("");
  const [submitting, setSubmitting] = useState(false);

  const handleSubmit = async () => {
    if (rating === 0) { Alert.alert("", "Selecione uma avaliação"); return; }
    setSubmitting(true);
    try {
      await api.post("/api/order/reviews", {
        order_id: orderId,
        user_phone: userPhone,
        user_name: userName,
        rating,
        comment,
        establishment_id: establishmentId,
      });
      Alert.alert("Obrigado!", "Sua avaliação foi registrada!");
      onClose();
    } catch (e) {
      Alert.alert("Erro", "Não foi possível enviar a avaliação");
    }
    setSubmitting(false);
  };

  return (
    <Modal visible={visible} transparent animationType="slide">
      <View style={styles.overlay}>
        <View style={styles.container}>
          <Text style={styles.title}>Avalie seu pedido</Text>
          <View style={styles.stars}>
            {[1,2,3,4,5].map(n => (
              <TouchableOpacity key={n} onPress={() => setRating(n)}>
                <Ionicons
                  name={n <= rating ? "star" : "star-outline"}
                  size={40}
                  color={n <= rating ? "#F59E0B" : "#D1D5DB"}
                />
              </TouchableOpacity>
            ))}
          </View>
          <TextInput
            style={styles.input}
            placeholder="Conte sua experiência (opcional)"
            value={comment}
            onChangeText={setComment}
            multiline
          />
          <View style={styles.buttons}>
            <TouchableOpacity style={styles.cancelBtn} onPress={onClose}>
              <Text style={styles.cancelText}>Pular</Text>
            </TouchableOpacity>
            <TouchableOpacity
              style={[styles.submitBtn, { opacity: submitting ? 0.6 : 1 }]}
              onPress={handleSubmit}
              disabled={submitting}
            >
              <Text style={styles.submitText}>
                {submitting ? "Enviando..." : "Enviar Avaliação"}
              </Text>
            </TouchableOpacity>
          </View>
        </View>
      </View>
    </Modal>
  );
};

const styles = StyleSheet.create({
  overlay: { flex: 1, backgroundColor: "rgba(0,0,0,0.5)", justifyContent: "flex-end" },
  container: { backgroundColor: "#FFF", borderTopLeftRadius: 20, borderTopRightRadius: 20, padding: 24, paddingBottom: 40 },
  title: { fontSize: 20, fontWeight: "700", textAlign: "center", marginBottom: 20 },
  stars: { flexDirection: "row", justifyContent: "center", gap: 8, marginBottom: 20 },
  input: { borderWidth: 1, borderColor: "#E5E7EB", borderRadius: 8, padding: 12, minHeight: 80, marginBottom: 20, textAlignVertical: "top" },
  buttons: { flexDirection: "row", gap: 12 },
  cancelBtn: { flex: 1, padding: 14, borderRadius: 8, borderWidth: 1, borderColor: "#E5E7EB", alignItems: "center" },
  cancelText: { color: "#666", fontWeight: "600" },
  submitBtn: { flex: 2, padding: 14, borderRadius: 8, backgroundColor: "#F97316", alignItems: "center" },
  submitText: { color: "#FFF", fontWeight: "600" },
});

export default ReviewModal;
