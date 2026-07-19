/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ["./src/**/*.{js,jsx,ts,tsx}"],
  theme: {
    extend: {
      colors: {
        fuu: {
          red: "#EA1D2C",
          "red-dark": "#C41420",
          "red-light": "#FEF2F2",
          yellow: "#F7A11E",
          "yellow-light": "#FFFBEB",
          orange: "#FF6B35",
        },
        customRed: "#EA1D2C",
        customRed2: "#FF4444",
        menu1: "#EA1D2C",
        menu2: "#C41420",
        primary: "#EA1D2C",
      },
      fontFamily: {
        display: ['"Inter"', '"Segoe UI"', "sans-serif"],
        body: ['"Inter"', '"Segoe UI"', "sans-serif"],
      },
      borderRadius: {
        xl: "16px",
        "2xl": "20px",
      },
      boxShadow: {
        card: "0 2px 12px rgba(0,0,0,0.06)",
        "card-hover": "0 8px 24px rgba(234,29,44,0.12)",
        sidebar: "4px 0 24px rgba(0,0,0,0.08)",
        modal: "0 20px 60px rgba(0,0,0,0.2)",
      },
      animation: {
        "fade-in": "fadeIn 0.3s ease-in-out",
        "slide-up": "slideUp 0.3s ease-out",
        "slide-in-left": "slideInLeft 0.3s ease-out",
      },
      keyframes: {
        fadeIn: {
          "0%": { opacity: "0" },
          "100%": { opacity: "1" },
        },
        slideUp: {
          "0%": { transform: "translateY(10px)", opacity: "0" },
          "100%": { transform: "translateY(0)", opacity: "1" },
        },
        slideInLeft: {
          "0%": { transform: "translateX(-20px)", opacity: "0" },
          "100%": { transform: "translateX(0)", opacity: "1" },
        },
      },
    },
  },
  plugins: [],
};
