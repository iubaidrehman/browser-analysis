import { useEffect } from 'react'

export type WSEvent = { type: string; session_id?: string; order_id?: string; at: number }

export function useWS(
  path: string,
  handlers: { onOpen?: () => void; onClose?: () => void; onEvent: (ev: WSEvent) => void },
) {
  useEffect(() => {
    const proto = location.protocol === 'https:' ? 'wss' : 'ws'
    const ws = new WebSocket(`${proto}://${location.host}${path}`)
    ws.onopen = () => handlers.onOpen?.()
    ws.onclose = () => handlers.onClose?.()
    ws.onmessage = (msg) => {
      try {
        handlers.onEvent(JSON.parse(msg.data) as WSEvent)
      } catch {
        // Ignore malformed frames.
      }
    }
    return () => ws.close()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [path])
}
