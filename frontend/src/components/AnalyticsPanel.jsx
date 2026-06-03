import { motion, AnimatePresence } from 'framer-motion'
import { BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer } from 'recharts'

function Section({ title, children }) {
  return (
    <section className="glass rounded-xl p-4 mb-4">
      <h3 className="text-xs uppercase tracking-[0.2em] text-white/40 mb-3">{title}</h3>
      {children}
    </section>
  )
}

function MetricTile({ label, value, unit }) {
  return (
    <div className="glass rounded-lg p-3 text-center">
      <p className="text-[10px] uppercase text-white/40">{label}</p>
      <p className="text-xl font-light mt-1">
        {value}
        {unit && <span className="text-xs text-white/40 ml-1">{unit}</span>}
      </p>
    </div>
  )
}

function StatusDot({ ok }) {
  return (
    <span className={`inline-block w-2 h-2 rounded-full ${ok ? 'bg-white' : 'bg-white/20'}`} />
  )
}

export default function AnalyticsPanel({ open, onClose, metrics, events, anomalies, health, heatmapCells, wsConnected }) {
  const leaderboard = metrics.leaderboard || []
  const reentryCount = events.filter((e) => e.event_type === 'REENTRY').length

  const journey = events
    .filter((e) => e.visitor_id && e.visitor_id !== 'SYSTEM')
    .slice(0, 8)
    .map((e) => e.zone_id || e.event_type)
    .filter(Boolean)
  const journeyPath = journey.length ? ['ENTRY', ...new Set(journey)] : ['ENTRY', '-', 'EXIT']

  const heatData = heatmapCells || []

  return (
    <AnimatePresence>
      {open && (
        <>
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 0.5 }}
            exit={{ opacity: 0 }}
            className="fixed inset-0 bg-black z-40"
            onClick={onClose}
          />
          <motion.aside
            initial={{ x: '100%' }}
            animate={{ x: 0 }}
            exit={{ x: '100%' }}
            transition={{ type: 'spring', damping: 28, stiffness: 260 }}
            className="fixed top-0 right-0 h-full w-full max-w-lg z-50 glass-strong overflow-y-auto p-6 shadow-glow"
          >
            <header className="flex justify-between items-center mb-6 pb-4 border-b border-white/10">
              <div>
                <h2 className="text-lg font-light tracking-wide">AI Analytics</h2>
                <p className="text-xs text-white/40">Live intelligence control center</p>
              </div>
              <button type="button" onClick={onClose} className="text-white/50 hover:text-white text-2xl">x</button>
            </header>

            <Section title="Live Metrics">
              <div className="grid grid-cols-3 gap-2">
                <MetricTile label="Visitors" value={metrics.active_visitors ?? 0} />
                <MetricTile label="Queue" value={metrics.queue_depth ?? 0} />
                <MetricTile label="Conversion" value={((metrics.conversion_rate ?? 0) * 100).toFixed(0)} unit="%" />
                <MetricTile label="Anomalies" value={anomalies.length} />
                <MetricTile label="Re-entry" value={reentryCount} />
                <MetricTile label="Staff" value={0} />
              </div>
            </Section>

            <Section title="Zone Leaderboard">
              <ResponsiveContainer width="100%" height={140}>
                <BarChart data={leaderboard.slice(0, 6).map((z) => ({ name: z.zone_id, v: z.visits || z.score || 0 }))}>
                  <XAxis dataKey="name" tick={{ fill: 'rgba(255,255,255,0.4)', fontSize: 9 }} />
                  <YAxis tick={{ fill: 'rgba(255,255,255,0.4)', fontSize: 9 }} />
                  <Tooltip contentStyle={{ background: '#111', border: '1px solid rgba(255,255,255,0.1)' }} />
                  <Bar dataKey="v" fill="rgba(255,255,255,0.85)" radius={[3, 3, 0, 0]} />
                </BarChart>
              </ResponsiveContainer>
            </Section>

            <Section title="Anomaly Feed">
              <ul className="space-y-2 max-h-36 overflow-y-auto text-xs font-mono">
                {anomalies.length === 0 && <li className="text-white/30">No anomalies detected</li>}
                {anomalies.map((a, i) => (
                  <li key={a.event_id || i} className="flex gap-2 items-start">
                    <span className="text-white/90 shrink-0">!</span>
                    <span>
                      {a.metadata?.anomaly_type || a.metadata?.movement_pattern || a.event_type}
                      {a.zone_id && <span className="text-white/40"> / {a.zone_id}</span>}
                    </span>
                  </li>
                ))}
              </ul>
            </Section>

            <Section title="Live Event Stream">
              <ul className="space-y-1 max-h-40 overflow-y-auto text-xs font-mono">
                {events.length === 0 && <li className="text-white/30">Waiting for Kafka events...</li>}
                {events.slice(0, 25).map((e, i) => (
                  <li key={e.event_id || i} className="truncate">
                    <span className="text-white">{e.event_type}</span>
                    <span className="text-white/40"> / {e.visitor_id || '-'}</span>
                    {e.zone_id && <span className="text-white/30"> @ {e.zone_id}</span>}
                  </li>
                ))}
              </ul>
            </Section>

            <Section title="Movement Heatmap">
              <div className="grid grid-cols-[repeat(16,minmax(0,1fr))] gap-1 w-full aspect-square max-h-64">
                {heatData.map((cell) => (
                  <div
                    key={`${cell.grid_x}-${cell.grid_y}`}
                    className="rounded-sm bg-white"
                    title={`${cell.grid_x},${cell.grid_y}: ${Math.round((cell.intensity || 0) * 100)}%`}
                    style={{ opacity: Math.max(0.04, cell.intensity || 0) }}
                  />
                ))}
              </div>
            </Section>

            <Section title="Visitor Journey">
              <div className="flex flex-wrap items-center gap-2 text-xs font-mono">
                {journeyPath.map((step, i) => (
                  <span key={i} className="flex items-center gap-2">
                    <span className="glass px-2 py-1 rounded">{step}</span>
                    {i < journeyPath.length - 1 && <span className="text-white/30">-&gt;</span>}
                  </span>
                ))}
              </div>
            </Section>

            <Section title="System Health">
              <div className="space-y-2 text-xs">
                <div className="flex items-center gap-2">
                  <StatusDot ok={wsConnected} /> WebSocket {wsConnected ? 'connected' : 'disconnected'}
                </div>
                {(health?.services || []).map((s) => (
                  <div key={s.name} className="flex items-center gap-2">
                    <StatusDot ok={s.ok} /> {s.name}
                  </div>
                ))}
                {health?.infrastructure && (
                  <>
                    <div className="flex items-center gap-2"><StatusDot ok={health.infrastructure.kafka?.ok} /> Kafka</div>
                    <div className="flex items-center gap-2"><StatusDot ok={health.infrastructure.redis?.ok} /> Redis Pub/Sub</div>
                    <div className="flex items-center gap-2"><StatusDot ok={health.infrastructure.postgresql?.ok} /> PostgreSQL</div>
                    <div className="flex items-center gap-2"><StatusDot ok={health.infrastructure.websocket?.ok} /> Detection + Gateway</div>
                  </>
                )}
              </div>
            </Section>
          </motion.aside>
        </>
      )}
    </AnimatePresence>
  )
}
