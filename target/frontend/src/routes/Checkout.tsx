import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, sessionID, type Order } from '../lib/api.ts'

export function Checkout() {
  const navigate = useNavigate()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  async function placeOrder() {
    const sid = sessionID.get()
    if (!sid) {
      navigate('/login')
      return
    }
    setBusy(true)
    setError('')
    try {
      const res = await api.post<{ order: Order }>('/api/checkout', { session_id: sid })
      navigate('/result', { state: { order: res.order } })
    } catch (e) {
      setError(String(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div>
      <h1>Checkout</h1>
      <p>Places the order against the synthetic backend and records it.</p>
      <button onClick={placeOrder} disabled={busy}>
        {busy ? 'placing order…' : 'Place order'}
      </button>
      {error && <p style={{ color: 'red' }}>{error}</p>}
    </div>
  )
}
