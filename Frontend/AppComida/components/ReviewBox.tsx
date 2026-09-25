import React, { useState } from "react";
import { View, Text, TextInput, TouchableOpacity, StyleSheet, ActivityIndicator } from "react-native";
import Colors from "@/constants/Colors";
import api from "@/services/api";

// Avaliação de um pedido entregue (POST /reviews). Mostra a nota já dada e a
// resposta da loja quando existir. O servidor só aceita do dono do pedido.
export default function ReviewBox({
  orderId,
  existing,
  userName,
  onDone,
}: {
  orderId: string;
  existing?: any;
  userName?: string;
  onDone: () => void;
}) {
  const [rating, setRating] = useState(0);
  const [comment, setComment] = useState("");
  const [sending, setSending] = useState(false);
  const [error, setError] = useState("");

  if (existing) {
    return (
      <View style={styles.box}>
        <Text style={styles.title}>Sua avaliação</Text>
        <Text style={styles.stars}>{"★".repeat(existing.rating)}{"☆".repeat(5 - existing.rating)}</Text>
        {existing.comment ? <Text style={styles.comment}>{existing.comment}</Text> : null}
        {existing.response_text ? (
          <View style={styles.reply}>
            <Text style={styles.replyTitle}>Resposta da loja</Text>
            <Text style={styles.comment}>{existing.response_text}</Text>
          </View>
        ) : null}
      </View>
    );
  }

  const send = async () => {
    setSending(true);
    setError("");
    try {
      await api.post("/reviews", {
        order_id: orderId,
        rating,
        comment: comment.trim(),
        user_name: userName || "",
      });
      onDone();
    } catch (e: any) {
      setError(e?.response?.data?.error || "Não foi possível enviar. Tente de novo.");
    }
    setSending(false);
  };

  return (
    <View style={styles.box}>
      <Text style={styles.title}>Como foi o pedido?</Text>
      <View style={styles.row}>
        {[1, 2, 3, 4, 5].map((n) => (
          <TouchableOpacity key={n} onPress={() => setRating(n)} accessibilityLabel={`${n} estrela${n > 1 ? "s" : ""}`}>
            <Text style={[styles.starBtn, n <= rating && styles.starOn]}>{n <= rating ? "★" : "☆"}</Text>
          </TouchableOpacity>
        ))}
      </View>
      <TextInput
        value={comment}
        onChangeText={setComment}
        placeholder="Conte como foi (opcional)"
        placeholderTextColor={Colors.light.secondaryText}
        maxLength={500}
        multiline
        style={styles.input}
      />
      {error ? <Text style={styles.error}>{error}</Text> : null}
      <TouchableOpacity
        style={[styles.send, (!rating || sending) && { opacity: 0.5 }]}
        disabled={!rating || sending}
        onPress={send}
      >
        {sending ? <ActivityIndicator color="#FFF" /> : <Text style={styles.sendText}>Enviar avaliação (+5 pontos)</Text>}
      </TouchableOpacity>
    </View>
  );
}

const styles = StyleSheet.create({
  box: { marginTop: 12, padding: 10, borderRadius: 8, backgroundColor: "#FFFBEB" },
  title: { fontSize: 14, fontWeight: "600", color: Colors.light.text, marginBottom: 6 },
  row: { flexDirection: "row", gap: 6, marginBottom: 8 },
  starBtn: { fontSize: 30, color: "#D1D5DB" },
  starOn: { color: "#F59E0B" },
  stars: { fontSize: 20, color: "#F59E0B", marginBottom: 4 },
  input: {
    minHeight: 50,
    borderWidth: 1,
    borderColor: Colors.light.tabIconDefault,
    borderRadius: 6,
    padding: 8,
    fontSize: 13,
    backgroundColor: "#FFF",
    textAlignVertical: "top",
    color: Colors.light.text,
  },
  comment: { fontSize: 13, color: Colors.light.text },
  reply: { marginTop: 8, padding: 8, borderRadius: 6, backgroundColor: "#FFF" },
  replyTitle: { fontSize: 12, fontWeight: "600", color: Colors.light.secondaryText, marginBottom: 2 },
  error: { color: "#B91C1C", fontSize: 12, marginTop: 6 },
  send: {
    marginTop: 8,
    backgroundColor: Colors.light.tint,
    borderRadius: 6,
    paddingVertical: 10,
    alignItems: "center",
  },
  sendText: { color: "#FFF", fontWeight: "600" },
});
