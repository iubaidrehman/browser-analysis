import { useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { api, sessionID, type CartItem } from '../lib/api.ts'

export function Cart() {
  const navigate = useNavigate()
  const [items, setItems] = useState<CartItem[]>([])
  const [total, setTotal] = useState(0)
  const [error, setError] = useState('')

  useEffect(() => {
    const sid = sessionID.get()
    if (!sid) {
      navigate('/login')
      return
    }
    let cancelled = false
    api
      .get<{ items: CartItem[]; total: number }>(`/api/cart?session_id=${encodeURIComponent(sid)}`)
      .then((r) => {
        if (!cancelled) {
          setItems(r.items)
          setTotal(r.total)
        }
      })
      .catch((e) => {
        if (!cancelled) setError(String(e))
      })
    return () => {
      cancelled = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  return (
    <div>
      <h1>Cart</h1>
      {error && <p style={{ color: 'red' }}>{error}</p>}
      {items.length === 0 ? (
        <p>Your cart is empty. <Link to="/home">Browse products</Link></p>
      ) : (
        <ul>
          {items.map((it) => (
            <li key={it.product_id}>
              {it.name} × {it.qty} — ${it.line_total.toFixed(2)}
            </li>
          ))}
        </ul>
      )}
      <p>
        <strong>Total: ${total.toFixed(2)}</strong>
      </p>
      <Link to="/checkout">Proceed to checkout</Link>
    </div>
  )
}
