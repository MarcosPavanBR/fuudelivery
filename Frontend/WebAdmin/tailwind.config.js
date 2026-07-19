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
        },
      },
      fontFamily: {
        display: ['Inter', 'system-ui', 'sans-serif'],
        body: ['Inter', 'system-ui', 'sans-serif'],
      },
    },
  },
  plugins: [],
}