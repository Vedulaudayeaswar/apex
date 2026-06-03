import { useEffect, useState } from 'react'
import { API } from '../config'

export function useOverlay(intervalMs = 100) {
  const [overlay, setOverlay] = useState({ detections: [], frame_size: { width: 640, height: 480 }, zones: {} })

  useEffect(() => {
    let active = true
    const poll = async () => {
      try {
        const res = await fetch(`${API}/detection/overlay`)
        if (res.ok && active) setOverlay(await res.json())
      } catch { /* offline */ }
    }
    let id
    const schedule = async () => {
      await poll()
      if (active) id = setTimeout(schedule, intervalMs)
    }
    schedule()
    return () => { active = false; clearTimeout(id) }
  }, [intervalMs])

  return overlay
}

export function useHeatmap(intervalMs = 2000) {
  const [cells, setCells] = useState([])

  useEffect(() => {
    let active = true
    const poll = async () => {
      try {
        const res = await fetch(`${API}/detection/heatmap`)
        if (res.ok && active) {
          const data = await res.json()
          setCells(data.cells || [])
        }
      } catch { /* offline */ }
    }
    poll()
    const id = setInterval(poll, intervalMs)
    return () => { active = false; clearInterval(id) }
  }, [intervalMs])

  return cells
}
