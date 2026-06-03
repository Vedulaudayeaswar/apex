/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{js,jsx}'],
  theme: {
    extend: {
      colors: {
        void: '#050505',
        glass: 'rgba(255,255,255,0.06)',
        'glass-border': 'rgba(255,255,255,0.12)',
      },
      boxShadow: {
        glow: '0 0 30px rgba(255,255,255,0.15)',
        'glow-sm': '0 0 15px rgba(255,255,255,0.2)',
      },
      animation: {
        pulseGlow: 'pulseGlow 2.5s ease-in-out infinite',
        float: 'float 4s ease-in-out infinite',
      },
      keyframes: {
        pulseGlow: {
          '0%, 100%': { boxShadow: '0 0 20px rgba(255,255,255,0.15)' },
          '50%': { boxShadow: '0 0 40px rgba(255,255,255,0.35)' },
        },
        float: {
          '0%, 100%': { transform: 'translateY(0)' },
          '50%': { transform: 'translateY(-6px)' },
        },
      },
    },
  },
  plugins: [],
}
