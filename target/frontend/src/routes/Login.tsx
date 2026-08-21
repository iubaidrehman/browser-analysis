import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, sessionID, type SessionInfo } from '../lib/api.ts'

export function Login() {
  const navigate = useNavigate()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  async function login() {
    setBusy(true)
    setError('')
    try {
      // Reuse the existing session if present, else create one.
      const existing = sessionID.get()
      const sess = existing
        ? await api.get<SessionInfo>(`/api/session?id=${encodeURIComponent(existing)}`)
        : await api.post<SessionInfo>('/api/session')
      sessionID.set(sess.session_id)
      navigate('/home')
    } catch (e) {
      setError(String(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div>
      <h1>Synthetic Target — Login</h1>
      <p>Creates or resumes a server-side session, then navigates to Home.</p>
      <button onClick={login} disabled={busy}>
        {busy ? 'creating session…' : 'Sign in'}
      </button>
      {error && <p style={{ color: 'red' }}>{error}</p>}
    </div>
  )
}
