import { useLocation } from 'react-router-dom'
import { type Order } from '../lib/api.ts'

export function Result() {
  const location = useLocation()
  const order = (location.state as { order?: Order } | null)?.order

  return (
    <div>
      <h1>Order Result</h1>
      {order ? (
        <div>
          <p>
            Order <strong>{order.id}</strong> — {order.status} — ${order.total.toFixed(2)}
          </p>
          <ul>
            {order.items.map((it) => (
              <li key={it.product_id}>
                {it.name} × {it.qty} — ${it.line_total.toFixed(2)}
              </li>
            ))}
          </ul>
        </div>
      ) : (
        <p>No order was placed yet. Head to the cart to check out.</p>
      )}
    </div>
  )
}
