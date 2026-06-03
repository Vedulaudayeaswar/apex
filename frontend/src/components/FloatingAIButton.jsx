import { motion } from 'framer-motion'

export default function FloatingAIButton({ onClick, open }) {
  return (
    <motion.button
      type="button"
      onClick={onClick}
      className="fixed bottom-8 right-8 z-50 min-w-24 h-14 px-5 rounded-full glass-strong flex items-center justify-center animate-pulseGlow animate-float shadow-glow cursor-pointer border border-white/20"
      whileHover={{ scale: 1.08 }}
      whileTap={{ scale: 0.95 }}
      aria-label="Open analytics"
    >
      <span className={`${open ? 'text-xl' : 'text-sm'} font-bold text-white`}>
        {open ? 'x' : 'Analytics'}
      </span>
    </motion.button>
  )
}
