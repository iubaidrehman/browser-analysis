// API client for the BCRL synthetic target. All requests go through the Vite
// dev-server proxy in development and through nginx in production.

export const apiBase = import.meta.env.VITE_API_BASE ?? ''

export type SessionInfo = { session_id: string; created_at: string; expires_at: string }

export type Product = {
  id: string
  name: string
  description: string
  price: number
  stock: number
}

export type CartItem = {
  product_id: string
  name: string
  qty: number
  price: number
  line_total: number
}

export type Order = {
  id: string
  session_id: string
  status: string
  total: number
  items: CartItem[]
  created_at: string
}

// sessionID persists the synthetic session across the SPA's storage layers
// (localStorage) so routes can share it.
export const sessionID = {
  get(): string | null {
    return localStorage.getItem('bcrl.session')
  },
  set(id: string) {
    localStorage.setItem('bcrl.session', id)
  },
  clear() {
    localStorage.removeItem('bcrl.session')
  },
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(`${apiBase}${path}`, {
    method,
    headers: body !== undefined ? { 'Content-Type': 'application/json' } : undefined,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })
  if (!res.ok) {
    const text = await res.text()
    throw new Error(`${res.status} ${text}`)
  }
  return (await res.json()) as T
}

export const api = {
  get: <T>(path: string) => request<T>('GET', path),
  post: <T>(path: string, body?: unknown) => request<T>('POST', path, body),
}
