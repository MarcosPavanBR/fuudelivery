// NativeWind (Tailwind para React Native) — mesma configuração do
// AppRestaurante para manter a stack unificada entre os 3 apps.
// Migração gradual: telas novas usam className="..."; as antigas continuam
// com StyleSheet até serem convertidas.
module.exports = function (api) {
  api.cache(true);
  return {
    presets: [
      ["babel-preset-expo", { jsxImportSource: "nativewind" }],
      "nativewind/babel",
    ],
  };
};
