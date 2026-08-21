import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// The dev server proxies API and WebSocket traffic to the Go backend so the
// SPA never needs to hardcode a backend URL.
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://localhost:8080',
      '/ws': { target: 'ws://localhost:8080', ws: true },
    },
  },
})
