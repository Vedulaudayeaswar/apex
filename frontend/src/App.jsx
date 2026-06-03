import { useState, useEffect, useCallback } from 'react'
import VideoExperience from './components/VideoExperience'
import FloatingAIButton from './components/FloatingAIButton'
import AnalyticsPanel from './components/AnalyticsPanel'
import { useWebSocket } from './hooks/useWebSocket'
import { useOverlay, useHeatmap } from './hooks/useOverlay'
import { useSystemHealth } from './hooks/useSystemHealth'
import { API, STORE_ID } from './config'

export default function App() {
  const [panelOpen, setPanelOpen] = useState(false)
  const { connected, events, metrics, anomalies, resetDashboard } = useWebSocket()
  const overlay = useOverlay(300)
  const heatmapCells = useHeatmap(2000)
  const health = useSystemHealth()
  const detections = overlay.detections || []
  const mergedMetrics = {
    ...metrics,
    ...(health?.metrics || {}),
    active_visitors: detections.length,
    queue_depth: detections.filter((d) => d.zone_id === 'BILLING').length,
  }

  const refreshMetrics = useCallback(async () => {
    try {
      const res = await fetch(`${API}/stores/${STORE_ID}/metrics`)
      if (res.ok) {
        const data = await res.json()
        // metrics updated via websocket primarily
        void data
      }
    } catch { /* offline */ }
  }, [])

  useEffect(() => {
    refreshMetrics()
    const id = setInterval(refreshMetrics, 10000)
    return () => clearInterval(id)
  }, [refreshMetrics])

  return (
    <div className="h-screen w-screen bg-void overflow-hidden relative">
      {/* Minimal top bar */}
      <header className="absolute top-0 left-0 right-0 z-30 flex justify-between items-center px-6 py-4 pointer-events-none">
        <div className="glass px-4 py-2 rounded-full pointer-events-auto">
          <span className="text-xs font-mono tracking-widest text-white/70">RETAIL AI / OPS CENTER</span>
        </div>
        <div className="glass px-3 py-1.5 rounded-full text-[10px] font-mono text-white/50 pointer-events-auto">
          {STORE_ID}
        </div>
      </header>

      {/* Full-screen video experience */}
      <main className="h-full w-full">
        <VideoExperience
          overlay={overlay}
          heatmapCells={heatmapCells}
          onUploadStart={resetDashboard}
          onUploadComplete={refreshMetrics}
        />
      </main>

      {/* Floating AI analytics trigger */}
      <FloatingAIButton open={panelOpen} onClick={() => setPanelOpen((o) => !o)} />

      {/* Slide-in analytics panel */}
      <AnalyticsPanel
        open={panelOpen}
        onClose={() => setPanelOpen(false)}
        metrics={mergedMetrics}
        events={events}
        anomalies={anomalies}
        health={health}
        heatmapCells={heatmapCells}
        wsConnected={connected}
      />
    </div>
  )
}
