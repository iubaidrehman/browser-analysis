import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, type Product } from '../lib/api.ts'

export function Home() {
  const [products, setProducts] = useState<Product[]>([])
  const [error, setError] = useState('')

  useEffect(() => {
    let cancelled = false
    api
      .get<{ products: Product[] }>('/api/products')
      .then((r) => {
        if (!cancelled) setProducts(r.products)
      })
      .catch((e) => {
        if (!cancelled) setError(String(e))
      })
    return () => {
      cancelled = true
    }
  }, [])

  return (
    <div>
      <h1>Home</h1>
      {error && <p style={{ color: 'red' }}>{error}</p>}
      <ul>
        {products.map((p) => (
          <li key={p.id}>
            <Link to={`/product/${p.id}`}>
              {p.name} — ${p.price.toFixed(2)}
            </Link>
          </li>
        ))}
      </ul>
    </div>
  )
}
