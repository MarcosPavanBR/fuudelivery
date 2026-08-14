/**
 * React Navigation v7 exige a declaração do RootParamList via module
 * augmentation. Os apps usam a API dinâmica (expo-router) com rotas
 * registradas por string — sem isso, o useNavigation().navigate() fica
 * tipado como `never` e todo navegador por nome quebra no type-check.
 * Esta augmentation restaura o comportamento v6 (nomes como string).
 */
import { ParamListBase } from "@react-navigation/native";

declare global {
  namespace ReactNavigation {
    interface RootParamList extends ParamListBase {}
  }
}

export {};
