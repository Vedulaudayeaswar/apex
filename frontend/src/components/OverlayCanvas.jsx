import { useEffect, useRef } from 'react'

const ZONE_COLORS = {
  ENTRY: 'rgba(255,255,255,0.08)',
  ELECTRONICS: 'rgba(255,255,255,0.05)',
  APPAREL: 'rgba(255,255,255,0.05)',
  BILLING: 'rgba(255,255,255,0.1)',
}

export default function OverlayCanvas({ detections, frameSize, zones, heatmapCells, videoElementId }) {
  const canvasRef = useRef(null)
  const containerRef = useRef(null)

  useEffect(() => {
    const canvas = canvasRef.current
    const container = containerRef.current
    if (!canvas || !container) return

    const draw = () => {
      const rect = container.getBoundingClientRect()
      canvas.width = rect.width
      canvas.height = rect.height
      const ctx = canvas.getContext('2d')
      ctx.clearRect(0, 0, canvas.width, canvas.height)

      const sourceWidth = frameSize.width || 640
      const sourceHeight = frameSize.height || 480
      const video = document.getElementById(videoElementId)
      const videoWidth = video?.naturalWidth || sourceWidth
      const videoHeight = video?.naturalHeight || sourceHeight
      const scale = Math.min(canvas.width / videoWidth, canvas.height / videoHeight)
      const renderWidth = videoWidth * scale
      const renderHeight = videoHeight * scale
      const offsetX = (canvas.width - renderWidth) / 2
      const offsetY = (canvas.height - renderHeight) / 2
      const sx = renderWidth / sourceWidth
      const sy = renderHeight / sourceHeight

      // Zone labels
      Object.entries(zones || {}).forEach(([name, rect]) => {
        if (!rect?.[0] || !rect?.[1]) return
        const [x1, y1] = rect[0]
        const [x2, y2] = rect[1]
        ctx.fillStyle = ZONE_COLORS[name] || 'rgba(255,255,255,0.04)'
        ctx.fillRect(offsetX + x1 * sx, offsetY + y1 * sy, (x2 - x1) * sx, (y2 - y1) * sy)
        ctx.strokeStyle = 'rgba(255,255,255,0.15)'
        ctx.lineWidth = 1
        ctx.strokeRect(offsetX + x1 * sx, offsetY + y1 * sy, (x2 - x1) * sx, (y2 - y1) * sy)
        ctx.fillStyle = 'rgba(255,255,255,0.5)'
        ctx.font = '10px monospace'
        ctx.fillText(name, offsetX + x1 * sx + 4, offsetY + y1 * sy + 14)
      })

      // Heatmap glow
      if (heatmapCells?.length) {
        const grid = Math.sqrt(heatmapCells.length) || 16
        heatmapCells.forEach((cell) => {
          const intensity = cell.intensity || 0
          if (intensity < 0.05) return
          const cellW = renderWidth / grid
          const cellH = renderHeight / grid
          ctx.fillStyle = `rgba(255,255,255,${intensity * 0.25})`
          ctx.fillRect(offsetX + cell.grid_x * cellW, offsetY + cell.grid_y * cellH, cellW, cellH)
        })
      }

      // Detections
      ;(detections || []).forEach((d) => {
        const [x1, y1, x2, y2] = d.bbox || [0, 0, 0, 0]
        const bx = offsetX + x1 * sx
        const by = offsetY + y1 * sy
        const bw = (x2 - x1) * sx
        const bh = (y2 - y1) * sy

        const isReentry = d.reentry
        const isAnomaly = d.is_anomaly

        ctx.strokeStyle = isAnomaly ? 'rgba(255,80,80,0.9)' : isReentry ? 'rgba(255,255,255,0.95)' : 'rgba(255,255,255,0.85)'
        ctx.lineWidth = isReentry ? 2.5 : 1.5
        ctx.strokeRect(bx, by, bw, bh)

        // Trajectory
        const traj = d.trajectory || []
        if (traj.length > 1) {
          ctx.beginPath()
          ctx.strokeStyle = 'rgba(255,255,255,0.35)'
          ctx.lineWidth = 1.5
          traj.forEach((p, i) => {
            const px = offsetX + p[0] * sx
            const py = offsetY + p[1] * sy
            if (i === 0) ctx.moveTo(px, py)
            else ctx.lineTo(px, py)
          })
          ctx.stroke()
        }

        // HUD label
        const label = [
          d.visitor_id,
          `ZONE: ${d.zone_id || '—'}`,
          `CONF: ${Math.round((d.confidence || 0) * 100)}%`,
        ]
        if (isReentry) label.push('REENTRY')
        if (isAnomaly) label.push('ANOMALY')

        const lh = 14
        const boxH = label.length * lh + 8
        ctx.fillStyle = 'rgba(0,0,0,0.7)'
        ctx.fillRect(bx, by - boxH - 2, Math.max(bw, 120), boxH)
        ctx.fillStyle = '#fff'
        ctx.font = '11px monospace'
        label.forEach((line, i) => ctx.fillText(line, bx + 4, by - boxH + 12 + i * lh))
      })
    }

    draw()
    const ro = new ResizeObserver(draw)
    ro.observe(container)
    const id = setInterval(draw, 100)
    return () => { ro.disconnect(); clearInterval(id) }
  }, [detections, frameSize, zones, heatmapCells, videoElementId])

  return (
    <div ref={containerRef} className="absolute inset-0 pointer-events-none">
      <canvas ref={canvasRef} className="w-full h-full" />
    </div>
  )
}
