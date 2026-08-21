import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { App } from './App.tsx'
import { Login } from './routes/Login.tsx'
import { Home } from './routes/Home.tsx'
import { Product } from './routes/Product.tsx'
import { Cart } from './routes/Cart.tsx'
import { Checkout } from './routes/Checkout.tsx'
import { Result } from './routes/Result.tsx'
import './index.css'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <BrowserRouter>
      <Routes>
        <Route element={<App />}>
          <Route path="/login" element={<Login />} />
          <Route path="/home" element={<Home />} />
          <Route path="/product/:id" element={<Product />} />
          <Route path="/cart" element={<Cart />} />
          <Route path="/checkout" element={<Checkout />} />
          <Route path="/result" element={<Result />} />
          <Route path="/" element={<Login />} />
        </Route>
      </Routes>
    </BrowserRouter>
  </StrictMode>,
)
