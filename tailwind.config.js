/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ["./templates/**/*.html", "./static/js/**/*.js"],
  theme: {
    extend: {
      colors: {
        "theme-bg": "#0B0F19",
        "theme-card": "#111827",
        "theme-overlay": "#111827",
        "theme-border": "#24334D",
        "theme-blue": "#3B82F6",
        "theme-muted": "#9CA3AF"
      },
      borderColor: {
        DEFAULT: "#24334D",
        "theme-border": "#24334D"
      }
    }
  },
  plugins: []
};
