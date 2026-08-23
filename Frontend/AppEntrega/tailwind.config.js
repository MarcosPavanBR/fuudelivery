/** @type {import('tailwindcss').Config} */
// Tailwind/NativeWind do AppEntrega — tokens da marca FuuDelivery
// (fonte única: brand/tokens.ts). Cores de status seguem o mesmo padrão
// dos outros apps (ver constants/Colors.ts).
//
// Personalidade do app (plano-melhorias-produto.md): foco e legibilidade —
// alto contraste para uso na rua, botões grandes, mapa em destaque.
module.exports = {
  content: [
    "./app/**/*.{js,jsx,ts,tsx}",
    "./components/**/*.{js,jsx,ts,tsx}",
    "./componentes/**/*.{js,jsx,ts,tsx}",
  ],
  presets: [require("nativewind/preset")],
  theme: {
    extend: {
      colors: {
        // Marca
        brand: {
          red: "#DC2626", // primária (brand/tokens.ts 600)
          "red-dark": "#B91C1C",
          "red-light": "#FEF2F2",
          yellow: "#F59E0B", // secundária
        },
        // Semânticas
        success: "#10B981",
        warning: "#F59E0B",
        danger: "#DC2626",
        info: "#3B82F6",
      },
    },
  },
  plugins: [],
};
