import { useEffect, useRef, useState, useCallback } from 'react'
import { WS_URL } from '../config'

export function useWebSocket() {
  const [connected, setConnected] = useState(false)
  const [events, setEvents] = useState([])
  const [metrics, setMetrics] = useState({})
  const [anomalies, setAnomalies] = useState([])
  const [lastEvent, setLastEvent] = useState(null)
  const wsRef = useRef(null)

  const resetDashboard = useCallback(() => {
    setEvents([])
    setMetrics({})
    setAnomalies([])
    setLastEvent(null)
  }, [])

  const handleMessage = useCallback((data) => {
    if (data.event_type === 'RESET') {
      resetDashboard()
      return
    }
    setLastEvent(data)
    if (data.active_visitors !== undefined || data.queue_depth !== undefined) {
      setMetrics((m) => ({ ...m, ...data }))
      return
    }
    if (data.event_type === 'ANOMALY' || data.metadata?.anomaly_type) {
      setAnomalies((prev) => [data, ...prev].slice(0, 40))
    }
    setEvents((prev) => [data, ...prev].slice(0, 80))
  }, [resetDashboard])

  useEffect(() => {
    let active = true
    let retryId

    const connect = () => {
      const ws = new WebSocket(WS_URL)
      wsRef.current = ws
      ws.onopen = () => setConnected(true)
      ws.onclose = () => {
        setConnected(false)
        if (active) retryId = setTimeout(connect, 2000)
      }
      ws.onmessage = (e) => {
        try {
          handleMessage(JSON.parse(e.data))
        } catch {
          handleMessage({ raw: e.data })
        }
      }
    }

    connect()
    return () => {
      active = false
      clearTimeout(retryId)
      wsRef.current?.close()
    }
  }, [handleMessage])

  return { connected, events, metrics, anomalies, lastEvent, resetDashboard }
}
