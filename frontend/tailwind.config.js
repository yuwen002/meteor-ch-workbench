/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{vue,js,ts,jsx,tsx}'],
  theme: {
    extend: {
      colors: {
        ch: {
          bg: '#1a1b1e',
          panel: '#25262b',
          border: '#3a3d45',
          text: '#d4d4d8',
          muted: '#71717a',
          accent: '#e74c3c',
          hover: '#2e2f36',
          active: '#3a3d45',
        },
      },
      fontSize: {
        xs: '11px',
        sm: '12px',
      },
    },
  },
  plugins: [],
}