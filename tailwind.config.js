/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ["./templates/**/*.html", "./static/js/**/*.js"],
  theme: {
    extend: {
      colors: {
        "fius-bg": "#0B0F19",
        "fius-card": "#111827",
        "fius-border": "#24334D",
        "fius-blue": "#3B82F6",
        "fius-muted": "#9CA3AF"
      },
      borderColor: {
        DEFAULT: "#24334D",
        "fius-border": "#24334D"
      }
    }
  },
  plugins: []
};
