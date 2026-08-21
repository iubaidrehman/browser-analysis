import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { api, sessionID, type Product } from '../lib/api.ts'

export function Product() {
  const { id } = useParams()
  const navigate = useNavigate()
  const [product, setProduct] = useState<Product | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    let cancelled = false
    api
      .get<{ product: Product }>(`/api/products/${id}`)
      .then((r) => {
        if (!cancelled) setProduct(r.product)
      })
      .catch((e) => {
        if (!cancelled) setError(String(e))
      })
    return () => {
      cancelled = true
    }
  }, [id])

  async function addToCart() {
    if (!product) return
    const sid = sessionID.get()
    if (!sid) {
      navigate('/login')
      return
    }
    try {
      await api.post('/api/cart', { session_id: sid, product_id: product.id, qty: 1 })
      navigate('/cart')
    } catch (e) {
      setError(String(e))
    }
  }

  if (!product && !error) return <p>Loading product {id}…</p>
  if (error) return <p style={{ color: 'red' }}>{error}</p>
  if (!product) return null

  return (
    <div>
      <h1>{product.name}</h1>
      <p>{product.description}</p>
      <p>
        ${product.price.toFixed(2)} · stock {product.stock}
      </p>
      <button onClick={addToCart}>Add to cart</button>
    </div>
  )
}
