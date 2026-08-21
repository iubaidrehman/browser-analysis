import { useEffect, useState } from 'react'
import { Outlet, Link } from 'react-router-dom'
import { api, apiBase, sessionID, type SessionInfo } from './lib/api.ts'
import { useWS } from './lib/ws.ts'

export function App() {
  const [session, setSession] = useState<SessionInfo | null>(null)
  const [events, setEvents] = useState<string[]>([])
  const [wsState, setWsState] = useState('disconnected')

  // Establish the synthetic session on first load.
  useEffect(() => {
    let cancelled = false
    api
      .post<SessionInfo>('/api/session')
      .then((s) => {
        if (cancelled) return
        sessionID.set(s.session_id)
        setSession(s)
      })
      .catch((e) => {
        if (!cancelled) setEvents((p) => [...p, `session error: ${e}`])
      })
    return () => {
      cancelled = true
    }
  }, [])

  // Listen to server WebSocket events.
  useWS('/ws/events', {
    onOpen: () => setWsState('connected'),
    onClose: () => setWsState('disconnected'),
    onEvent: (ev) =>
      setEvents((p) => [...p.slice(-19), `${ev.type}${ev.order_id ? ' ' + ev.order_id : ''}`]),
  })

  return (
    <div style={styles.layout}>
      <nav style={styles.nav}>
        <Link to="/home" style={styles.link}>Home</Link>
        <Link to="/cart" style={styles.link}>Cart</Link>
        <Link to="/checkout" style={styles.link}>Checkout</Link>
        <Link to="/result" style={styles.link}>Result</Link>
        <span style={styles.meta}>
          {session ? `session ${session.session_id}` : 'connecting…'} · ws {wsState}
        </span>
      </nav>
      <main style={styles.main}>
        <Outlet />
      </main>
      <footer style={styles.footer}>
        <div>
          <strong>WS events:</strong> {events.join(' · ') || 'none'}
        </div>
        <div style={styles.meta}>api {apiBase}</div>
      </footer>
    </div>
  )
}

const styles = {
  layout: { minHeight: '100vh', display: 'flex', flexDirection: 'column' as const, fontFamily: 'system-ui, sans-serif' },
  nav: { display: 'flex', gap: 16, padding: '12px 24px', borderBottom: '1px solid #ddd', alignItems: 'center' },
  link: { color: '#1a73e8', textDecoration: 'none' },
  meta: { marginLeft: 'auto', color: '#666', fontSize: 12 },
  main: { flex: 1, padding: '24px', maxWidth: 900, width: '100%', margin: '0 auto' },
  footer: { padding: '12px 24px', borderTop: '1px solid #ddd', fontSize: 12, color: '#444' },
}
