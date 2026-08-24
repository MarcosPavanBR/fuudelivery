/**
 * ErrorBoundary — captura erros de renderizacao e exibe tela amigavel.
 *
 * Fluxo:
 * 1. Qualquer erro de React (render, lifecycle, hooks) e capturado
 * 2. Exibe tela com icone, mensagem e botao de retry
 * 3. Loga o erro para debug
 * 4. Botao "Tentar novamente" reseta o estado e re-renderiza
 *
 * Uso: envolver qualquer tela ou o layout raiz.
 */
import React, { Component, ErrorInfo, ReactNode } from "react";
import { View, Text, TouchableOpacity, StyleSheet } from "react-native";

interface Props {
  children: ReactNode;
  screenName?: string;
}

interface State {
  hasError: boolean;
  error: Error | null;
}

export class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = { hasError: false, error: null };
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error(
      `[ErrorBoundary] Erro capturado em "${this.props.screenName || "desconhecida"}":`,
      error.message,
      errorInfo.componentStack
    );
  }

  handleRetry = () => {
    this.setState({ hasError: false, error: null });
  };

  render() {
    if (this.state.hasError) {
      return (
        <View style={styles.container}>
          <Text style={styles.emoji}>😔</Text>
          <Text style={styles.title}>Algo deu errado</Text>
          <Text style={styles.message}>
            Ocorreu um erro inesperado. Tente novamente.
          </Text>
          {__DEV__ && this.state.error && (
            <Text style={styles.errorDetail}>{this.state.error.message}</Text>
          )}
          <TouchableOpacity style={styles.button} onPress={this.handleRetry}>
            <Text style={styles.buttonText}>Tentar novamente</Text>
          </TouchableOpacity>
        </View>
      );
    }

    return this.props.children;
  }
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    justifyContent: "center",
    alignItems: "center",
    backgroundColor: "#F5F5F5",
    padding: 24,
  },
  emoji: { fontSize: 64, marginBottom: 16 },
  title: { fontSize: 20, fontWeight: "700", color: "#1A1A1A", marginBottom: 8 },
  message: { fontSize: 15, color: "#666", textAlign: "center", marginBottom: 24 },
  errorDetail: { fontSize: 11, color: "#999", textAlign: "center", marginBottom: 16, fontFamily: "monospace" },
  button: { backgroundColor: "#DC2626", paddingHorizontal: 32, paddingVertical: 14, borderRadius: 12 },
  buttonText: { color: "#FFF", fontSize: 16, fontWeight: "600" },
});
