---
name: react-native-patterns
description: Padrões Expo/React Native para os apps AppComida, AppEntrega e AppRestaurante — performance de lista, localização em background, e navegação. Use ao construir ou revisar qualquer tela mobile.
---

# Padrões React Native / Expo

## Listas
- `FlatList`/`FlashList` com `keyExtractor` estável (id real, nunca índice do array) para cardápio, histórico de pedidos, e fila de entregas — `.map()` direto em lista que cresce é a causa mais comum de tela travando em dispositivo real de gama baixa.

## Localização em background (AppEntrega)
- Peça permissão de localização em background explicitamente (`expo-location`), com throttling de envio (ex: a cada 5-10s ou X metros percorridos, não a cada frame do GPS).
- O backend que recebe essas coordenadas via WebSocket precisa confirmar que o entregador autenticado é o dono daquela entrega específica — isso é responsabilidade do backend, mas o app deve assumir que pode ser rejeitado e tratar o erro sem crashar.

## Navegação e estado de rede
- Telas de checkout e status de pedido tratam perda de conexão sem duplicar a ação ao reconectar — desabilite o botão de confirmar após o primeiro toque, não confie só em debounce visual.
- Reconexão de WebSocket não deve duplicar listeners: sempre remova o listener anterior antes de registrar um novo ao reconectar.

## Sessão
- Token/sessão via cookie HttpOnly quando o backend expõe essa opção (ex: `/auth/session`) em vez de `AsyncStorage` — AsyncStorage não é criptografado por padrão e é acessível caso o dispositivo seja comprometido.

## Cobertura de teste
- Apps com zero testes: comece pelo fluxo de maior risco financeiro (checkout no AppComida, aceite de pedido no AppRestaurante) usando Detox ou Maestro antes de qualquer tela de exibição pura.
