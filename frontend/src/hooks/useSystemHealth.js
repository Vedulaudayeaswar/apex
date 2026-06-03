import { useEffect, useState } from 'react'
import { API, STORE_ID } from '../config'

export function useSystemHealth(intervalMs = 8000) {
  const [health, setHealth] = useState(null)

  useEffect(() => {
    let active = true
    const poll = async () => {
      try {
        const [hRes, mRes] = await Promise.all([
          fetch(`${API}/system/health`),
          fetch(`${API}/stores/${STORE_ID}/metrics`),
        ])
        const h = hRes.ok ? await hRes.json() : null
        const m = mRes.ok ? await mRes.json() : {}
        if (active) setHealth({ ...h, metrics: m })
      } catch { /* offline */ }
    }
    poll()
    const id = setInterval(poll, intervalMs)
    return () => { active = false; clearInterval(id) }
  }, [intervalMs])

  return health
}
